# Part A — The mental model, from first principles

Running example used throughout: **a customer-support agent workflow**. A request arrives; the workflow calls a flaky external ticketing API; it then waits for a human to approve a refund; workers and even server processes may crash in the middle; the workflow must retry safely, and eventually complete or time out.

Nothing here claims Temporal is an agent framework. The example is a generic long-running, failure-prone piece of application logic.

---

## A.1 Why a normal process or job queue is not enough

Consider the naive implementation: one Go process runs

```
ticket := callTicketAPI(req)        // may hang or fail
approval := waitForHuman(ticket)    // may take three days
finish(ticket, approval)
```

Problems, each of which a job queue alone does not fix:

1. **The process is the state.** Local variables (`ticket`, "we are now waiting for a human") live only in RAM. A crash or deploy at any point loses *where we were*. Restarting re-runs `callTicketAPI` and creates a second ticket.
2. **Waiting is not free.** Blocking a goroutine for three days pins a process to a machine and dies with it. You cannot upgrade or rebalance without losing the wait.
3. **Retries are ambiguous.** If `callTicketAPI` times out, you do not know whether the ticket was created. Retrying blindly duplicates side effects; not retrying loses work.
4. **A job queue only solves delivery.** It gives you "run this function at least once somewhere." It does not know that step 2 must follow step 1, does not remember step 1's result, and delivers the same job twice under failure. You end up hand-writing a state table in a database plus a scheduler plus timers plus dedup keys. That hand-written thing is the thing Temporal is.
5. **Concurrency on one logical entity.** Two workers might pick up "ticket 42" at once. You need a lock or a version check per entity, and you need it to survive the lock-holder dying.

## A.2 What "durable execution" means operationally

Operationally, durable execution is four concrete commitments:

1. **Every decision the workflow logic makes is recorded before it is acted on.** (Not "the result is saved after"; the *intent* is persisted first.)
2. **The recorded decisions are enough to rebuild the logic's in-memory position.** If the process dies, another process replays the record and lands exactly where the previous one was, including which calls were already issued.
3. **Side effects live outside the replayable logic**, in units that can be retried independently, and each attempt is tracked.
4. **Waiting is a persisted timer, not a blocked thread.** The process running the logic can go away while the wait continues.

Temporal's server implements these with an append-only event history per execution (commitment 1 and 2), Activities as the side-effect unit (3), and a per-shard timer queue (4). That is the whole idea; the rest is engineering for scale, consistency and failure.

## A.3 The vocabulary, precisely

| Term | What it is | Who owns it |
|---|---|---|
| **Workflow Execution** | One durable run of one workflow, identified by namespace + workflow ID + run ID. It has a history and a mutable state record. It is *not* a process. | History service (one shard owns it) |
| **Workflow code** | User code (in a Worker) that, given the history so far, decides what should happen next. Must be deterministic: same history in, same decisions out. | User's Worker process |
| **Activity code** | User code that performs side effects (call the ticket API). May be non-deterministic and slow. Retried by the server according to a retry policy. | User's Worker process |
| **Worker process** | A user-hosted process running the SDK. It long-polls task queues, runs workflow code and activity code, and reports results. It holds no durable state. | User |
| **Workflow Task** | A unit of work meaning "run the workflow code forward from its last position, using these new events, and return commands." Created by the server when something happened that the workflow code needs to react to. | Created by History, queued in Matching, executed by a Worker |
| **Activity Task** | A unit of work meaning "run this activity attempt." | Same path |
| **Task Queue** | A named queue in the Matching service that Workers poll. Multiplexes tasks for many executions. Has partitions (default 4 in docs) and a persisted backlog. | Matching service |
| **Event history** | The append-only, ordered list of history events for one execution (`WorkflowExecutionStarted`, `WorkflowTaskScheduled`, `ActivityTaskScheduled`, ...). It is the durable source of truth and is what the Worker replays. | History service; stored in the history-node tables |
| **Mutable state** | A per-execution summary record (pending activities, pending timers, current workflow task, next event ID, versions) kept so the server does not replay history on every request. It is persisted alongside history tasks in one transaction and cached in memory. | History service |
| **Persistence** | The storage layer behind interfaces in `common/persistence`: executions (mutable state), history nodes, history tasks (transfer/timer/visibility/...), shards, task-queue tasks. Backends: Cassandra, MySQL, PostgreSQL, SQLite. | Everyone via interfaces |
| **Replay** | The SDK-side act of re-running workflow code from the beginning against the history to reconstruct its position. The server does not replay user code; it only guarantees the history is complete and ordered. | Worker (SDK). Server-side "rebuild" of mutable state from events exists separately (`service/history/workflow/mutable_state_rebuilder.go`), used for reset/replication, not for normal requests. |

## A.4 Who decides, who does, who owns, who routes

Using the example:

- **Decides** (what should happen next): the **workflow code** in the Worker. It says "schedule activity CallTicketAPI", "start a 72-hour timer", "wait for signal `approved`". These are *commands*, not effects.
- **Performs external work**: **activity code** in the Worker (the actual HTTP call). The human approval arrives as a *signal* from some client, not as activity code.
- **Owns durable state**: the **History service** shard that owns the execution. Only it appends events, updates mutable state, and creates internal tasks; it does so in one persistence transaction fenced by a shard `RangeID`.
- **Routes** (holds no truth): the **Frontend** (validates, authorizes, rate-limits, maps workflow ID → shard, forwards) and the **Matching** service (holds task queues and matches tasks to pollers; its persisted backlog is a delivery buffer, not the source of truth about the execution).
- **Internal Worker service** (`service/worker`): system workflows (scanner, batcher, deletenamespace, scheduler, migration, ...) implemented with the Go SDK against the cluster itself. Defer.

## A.5 The central guarantees and their limits

Observed in `docs/architecture/README.md` and confirmed in code:

1. **Exactly-once *recording* of decisions.** A given `WorkflowTaskCompleted` and its commands are appended once, guarded by mutable-state conditions (`DBRecordVersion` / `NextEventID`) and shard `RangeID`. Limit: this is exactly-once on the *record*, not on the side effects the activities perform.
2. **At-least-once execution of activities**, with attempt tracking and retry policy. Limit: activity code must be idempotent (or marked non-retryable). The server cannot know whether a timed-out HTTP call landed.
3. **Workflow code sees a consistent, ordered history and is resumed from where it was.** Limit: only if the workflow code is deterministic. The server does not verify this; the SDK detects mismatches during replay.
4. **Timers fire at least once after their due time**, even if every Worker was down. Limit: precision is bounded by timer-queue polling and shard load; late is normal, early is a bug.
5. **A task written to a History internal queue will eventually be delivered to Matching** (transactional outbox). Limit: "eventually" and "at least once"; the consumer must tolerate duplicates and stale tasks, which is why tasks carry `ScheduledEventID`, `Stamp`, `Attempt`, and versions.
6. **One shard owner at a time.** Membership decides the intended owner; the persisted `RangeID` lease is the actual fence. Limit: two hosts may *believe* they own a shard briefly; the loser's writes fail with `ShardOwnershipLostError`.

Assumptions behind all of this: a persistence store with conditional writes (Cassandra LWT batches, SQL row locks in a transaction), clocks that are roughly synchronized but never trusted for correctness, and user code that follows the determinism and idempotency rules.
