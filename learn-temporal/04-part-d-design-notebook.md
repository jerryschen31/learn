# Part D — First-principles design notebook

Each topic: problem → invariant → mechanism → code → concrete failure → common wrong model → self-check question → (optional) harness comparison. Order is chosen so each topic only depends on earlier ones.

---

## D1. Event history versus mutable state

**Problem.** You need a durable, replayable record of everything that happened to an execution (for recovery and for the Worker's replay), *and* you need to answer "what is pending right now?" thousands of times per second without re-reading the whole record.

**Invariant.** Mutable state is always a faithful projection of the history up to a known event (`NextEventID`), and both are persisted together with the tasks they imply, or not at all.

**Mechanism.** `MutableStateImpl` holds `executionInfo`, `executionState`, `pendingActivityInfoIDs`, `pendingTimerInfoIDs`, `pendingChildExecutionInfoIDs`, the current workflow task, `nextEventIDInDB`, `dbRecordVersion`, and an embedded `HistoryBuilder` (`service/history/workflow/mutable_state_impl.go:112-180`). Every `AddXEvent` method (a) asks the builder to create the event, (b) applies it to the projection via the matching `ApplyXEvent` method (the same methods used when rebuilding from history), and (c) may generate tasks. `CloseTransactionAsSnapshot` / `CloseTransactionAsMutation` (`:7652`, `:7606`) → `closeTransaction` (`:7775`) flush events into batches, bump `StateTransitionCount`, compute a checksum, and hand back `WorkflowSnapshot|WorkflowMutation` + `[]*WorkflowEvents` for one persistence call.

**Code.** `service/history/interfaces/mutable_state.go:44` (interface), `service/history/historybuilder/history_builder.go`, `service/history/workflow/mutable_state_rebuilder.go` (rebuild from events; used by reset/replication, showing the projection is derivable). Storage shape: `proto/internal/temporal/server/api/persistence/v1/` (`WorkflowExecutionInfo`, `ActivityInfo`, ...). Doc: `docs/architecture/history-service.md` "Consistency guarantees".

**Concrete failure.** History appends events 4–5 but the mutable-state write fails with `WorkflowConditionFailedError`. Observed handling: `ContextImpl.Clear()` on error paths in `workflow/context.go` drops the cached projection; the next request reloads from persistence, which still says `NextEventID=4`; the orphan events 4–5 are invisible because mutable state does not point at them (doc: "a History Event is only 'valid' if it is in Mutable State").

**Wrong model.** "Mutable state is a cache of history." It is a *persisted projection* and the source of authority for what is *pending*; history is the source of authority for what *happened*. Both are written durably.

**Check question.** After `RespondWorkflowTaskCompleted` with a `ScheduleActivityTask` command, name every field of mutable state that changed and the one event ID they reference.

**Harness comparison.** Your chat-history memory is the "history"; a summary/context-window selection is a projection. Temporal's rule "projection and log commit together" is the guarantee your harness lacks if it summarizes in memory and persists later.

---

## D2. Deterministic workflow execution and replay

**Problem.** The process running the workflow logic will die. Its position (which line, which local variables) must be reconstructable elsewhere.

**Invariant.** Given the same history prefix, workflow code must issue the same commands. The server relies on it; it does not check it.

**Mechanism.** The server ships history to the Worker with every workflow task (`RecordWorkflowTaskStartedResponse.History`, `recordworkflowtaskstarted.CreateRecordWorkflowTaskStartedResponseWithRawHistory`, `service/history/api/recordworkflowtaskstarted/api.go:408`), or only the delta when a sticky queue is in use (`StickyExecutionEnabled`). The SDK replays. The `speculative-workflow-task.md` doc states the contract: `WorkflowTaskScheduled` and `WorkflowTaskStarted` are always the last two events shipped, and transient/speculative tasks synthesize them.

**Code.** `service/history/api/recordworkflowtaskstarted/api.go`, `service/history/api/get_history_util.go`, `docs/architecture/speculative-workflow-task.md`. Nondeterminism is reported *by the SDK* via `RespondWorkflowTaskFailed` (`service/history/api/respondworkflowtaskfailed/api.go:22`) with a `WorkflowTaskFailedCause`; the server records `WorkflowTaskFailed` and reschedules (`workflowTaskStateMachine.AddWorkflowTaskFailedEvent`, `workflow_task_state_machine.go:898`).

**Concrete failure.** A deploy changes the workflow so it now calls activity B before A. The Worker replays history containing `ActivityTaskScheduled(A)` and its code asks for B: the SDK detects the mismatch, fails the task; server records `WorkflowTaskFailed`, attempt increments, next task is transient; this repeats until the code is fixed or the workflow is reset. `tests/workflow_failures_test.go` `TestWorkflowTaskFailed` shows the server side.

**Wrong model.** "The server replays my workflow." It never runs user code. Server-side "rebuild" (`mutable_state_rebuilder.go`) rebuilds *mutable state* from events; that is a different thing.

**Check question.** Why must the workflow task shipped to a Worker end with `WorkflowTaskStarted`, and what would break if it ended with `ActivityTaskCompleted`?

---

## D3. Why Activities are separate from Workflow code

**Problem.** Side effects (HTTP calls) are non-deterministic, slow, and may or may not have happened when they fail. They cannot live inside replayable logic.

**Invariant.** An activity attempt is identified (`ScheduledEventID`, `Attempt`, `Stamp`, `RequestId`) and only the current attempt can report a result. Retries are server-driven.

**Mechanism.** `handleCommandScheduleActivity` (`service/history/api/respondworkflowtaskcompleted/workflow_task_completed_handler.go:470`) → `AddActivityTaskScheduledEvent` (`mutable_state_impl.go:4305`) creates an `ActivityInfo` and a transfer `ActivityTask`; timeouts come from `TimerSequence` (`workflow/timer_sequence.go:118 CreateNextActivityTimer`); `RetryActivity` (`mutable_state_impl.go:6879`) computes `nextBackoffInterval`, increments `Attempt`, and emits an `ActivityRetryTimerTask`; the retry timer executor calls Matching directly (`timer_queue_active_task_executor.go:540`).

**Code.** also `service/history/api/recordactivitytaskstarted/api.go:38`, `service/history/api/respondactivitytaskfailed/api.go:23`, `service/history/workflow/retry.go`.

**Concrete failure.** Attempt 1 times out at start-to-close; server schedules attempt 2. The attempt-1 Worker then sends `RespondActivityTaskCompleted`. `recordactivitytaskstarted`/`respondactivitytask*` compare `Attempt`/`Stamp` from the token with `ActivityInfo` and reject with `ErrActivityTaskNotFound` (`service/history/consts/const.go:45`). The external side effect may have happened twice; that is the user's idempotency problem, by design.

**Wrong model.** "Activity retries appear in history." They do not; only the final outcome does. Observed: `RetryActivity` mutates `ActivityInfo` and generates a timer task, no event.

**Check question.** Which persisted fields let the server reject a stale activity completion, and where are they compared?

**Harness comparison.** A tool call in your agent loop is an activity. If your loop retries a tool call after a timeout, you have the same duplicate-side-effect problem; Temporal's answer is attempt tracking plus user-supplied idempotency, not magic.

---

## D4. Task queues and pull-based workers

**Problem.** The server must not know Worker addresses, Workers must scale independently, and load must not overwhelm Workers.

**Invariant.** A task is delivered to at most one Worker *as started* (enforced by History `Record*TaskStarted`), even if Matching delivers it more than once.

**Mechanism.** Workers long-poll (`PollWorkflowTaskQueue`), Matching either sync-matches a task to a waiting poller (`TaskMatcher.Offer`, `matcher.go:108`) or spools it (`SpoolTask` → `taskWriter`), and before handing it out calls History (`recordWorkflowTaskStarted`, `matching_engine.go:3502`). Partitions (`MatchingNumTaskqueueReadPartitions`/`WritePartitions`) spread load; forwarding (`forwarder.go`) moves polls/tasks toward the root partition when a partition is idle (`docs/architecture/matching-service.md`).

**Code.** `service/matching/matching_engine.go`, `task_queue_partition_manager.go`, `physical_task_queue_manager.go`, `matcher.go`, `task_writer.go`, `task_reader.go`, `ack_manager.go`, `client/matching/loadbalancer.go`.

**Concrete failure.** Matching host dies after sync-matching a task but before the Worker responds. Nothing is lost: History already recorded `WorkflowTaskStarted` and a start-to-close timer; the Worker either responds (fine) or the timer reschedules. If Matching had spooled the task and died before persisting its ack level, the task is re-read and re-offered; History replies `TaskAlreadyStarted` / `NotFound` and Matching drops it (`matching_engine.go:~806`).

**Wrong model.** "Matching's backlog is the queue of truth." It is a delivery buffer. The truth about whether a task is still needed is in History's mutable state.

**Check question.** Trace what happens if two pollers on two partitions both receive the same spooled task after a Matching restart.

---

## D5. Persistence, optimistic concurrency, and ownership

**Problem.** Many goroutines and hosts may try to update one execution; and the store must reject writes from a host that no longer owns the shard.

**Invariant.** A write succeeds only if (a) the shard `RangeID` in the request equals the stored one and (b) the execution's version condition (`DBRecordVersion` / legacy `NextEventID` `Condition`) matches.

**Mechanism.** `shard.ContextImpl.CreateWorkflowExecution` / `UpdateWorkflowExecution` stamp `request.RangeID` under the shard lock (`shard/context_impl.go:584`), then call `ExecutionManager`. Cassandra: one logged batch with `templateUpdateLeaseQuery` and `MapExecuteBatchCAS` (`cassandra/mutable_state_store.go:383+`); SQL: a transaction with `lockShard` (`sql/shard.go:124`) and `lockCurrentExecutionIfExists` (`sql/execution_util.go:575`). Errors map to `ShardOwnershipLostError`, `CurrentWorkflowConditionFailedError`, `WorkflowConditionFailedError`, `ConditionFailedError` (`data_interfaces.go:114-149`). In-process, the workflow cache (`workflow/cache/cache.go`) serializes access per execution with a lock and release function; `closeTransaction` bumps `dbRecordVersion`.

**Code.** `common/persistence/execution_manager.go`, `common/persistence/data_interfaces.go`, `service/history/shard/context_impl.go:1501 handleWriteErrorLocked`, `service/history/workflow/transaction_impl.go`.

**Concrete failure.** Host A owned shard 7, got partitioned, and still has a cached mutable state for `ticket-42`. Host B acquires shard 7 (`RangeID` 12→13) and completes a workflow task. Host A's queued update carries `RangeID=12`; the CAS/lock fails → `ShardOwnershipLostError` → `handleWriteErrorLocked` transitions the shard to stop. No torn write.

**Wrong model.** "Membership (ringpop) decides who can write." Membership decides who *tries*; the persisted `RangeID` decides who *succeeds*.

**Check question.** What does `handleWriteErrorLocked` do with a context-deadline error, and why is that different from a condition-failed error?

---

## D6. Shards/partitions and scalable state ownership

**Problem.** Millions of executions; one host cannot hold the locks, caches and queues for all of them; but per-execution ordering must be preserved.

**Invariant.** `(namespaceID, workflowID)` → exactly one shard (`common.WorkflowIDToHistoryShard`, `common/util.go:418`), and one shard is served by one engine at a time.

**Mechanism.** `ControllerImpl.acquireShards` (`shard/controller_impl.go:375`) walks all shard IDs, asks `ownership.verifyOwnership` (membership), and for owned shards calls `GetShardByID` → `ContextImpl.acquireShard` (`context_impl.go:2030`) → `renewRangeLocked` → `ShardManager.UpdateShard`. Losing ownership triggers `CloseShardByID` or a linger (`ShardLingerTimeLimit`). Each shard has its own queue processors and task-key allocator (`task_key_manager.go`).

**Code.** as above plus `service/history/shard/ownership.go`, `common/membership/interfaces.go`, `client/history/caching_redirector.go`.

**Concrete failure.** A History host is added. Membership fires; some shards move. During the move, a Frontend's cached shard→host mapping is stale; the old host answers `ShardOwnershipLostError`; `CachingRedirector.redirectLoop` re-resolves and retries. Meanwhile the new owner reloads queue ack levels and re-runs some tasks, which the stale checks tolerate.

**Wrong model.** "Number of shards can be tuned like a knob." It is static config (`numHistoryShards`), fixed at cluster creation; `MapShardID` only supports whole-multiple mappings.

**Check question.** Why does the shard hash include the namespace ID, and what would break in a multi-tenant cluster if it did not?

---

## D7. Timers, retries, timeouts, and delayed work

**Problem.** "Do X at time T" must survive every process dying, and there may be millions of pending timers.

**Invariant.** A persisted timer task fires at least once at or after its `VisibilityTimestamp`; firing is idempotent because the executor re-validates against mutable state.

**Mechanism.** Timer tasks (`service/history/tasks/*_timer.go`: `UserTimerTask`, `ActivityTimeoutTask`, `WorkflowTaskTimeoutTask`, `ActivityRetryTimerTask`, `WorkflowRunTimeoutTask`, ...) are written in the same transaction as the state change that needs them. The per-shard `scheduledQueue` (`queues/queue_scheduled.go:175 processEventLoop`, `lookAheadTask`) reads tasks by time and submits them; `timerQueueActiveTaskExecutor.Execute` (`timer_queue_active_task_executor.go:77`) dispatches by type. `TimerSequence` (`workflow/timer_sequence.go`) makes sure only the *next* activity/user timer is materialized as a task, not one per timeout type. Speculative workflow-task timeouts use an in-memory queue (`docs/architecture/in-memory-queue.md`).

**Code.** `service/history/workflow/task_generator.go` (`GenerateStartWorkflowTaskTasks`, `GenerateActivityRetryTasks`, `GenerateUserTimerTasks`), `service/history/timer_queue_active_task_executor.go`.

**Concrete failure.** The shard was unowned for 10 minutes (rolling restart gone wrong). On re-acquire, the timer queue reads all tasks with `VisibilityTimestamp` ≤ now and fires them in order; a user timer that should have fired 9 minutes ago fires now. Late, never early, never lost.

**Wrong model.** "`workflow.Sleep(30d)` holds a goroutine somewhere." It is one row (`UserTimerTask`) plus a `TimerInfo` in mutable state; no process waits.

**Check question.** For an activity with schedule-to-close 10m, start-to-close 2m, and a retry policy, how many timer tasks exist right after `ActivityTaskScheduled`, and what creates the next one?

---

## D8. Failure recovery and duplicate-delivery implications

**Problem.** At-least-once is the only delivery guarantee any layer here gives (outbox re-execution after checkpoint, Matching re-offer, client retries). Every consumer must be safe under duplicates and staleness.

**Invariant.** Every task and every worker-facing request carries enough identity (`ScheduledEventID`, `Attempt`, `Stamp`, `Version`, `RequestId`, `RangeID`) that the owner can decide "already done / stale / current" by comparison with mutable state.

**Mechanism.** Concrete comparison sites: `processWorkflowTask` (`transfer_queue_active_task_executor.go:289`: task gone → nil; `Stamp` mismatch → `ErrStaleReference`; `CheckTaskVersion`), `processActivityTask` (`:234`), `executeWorkflowTaskTimeoutTask` (`timer_queue_active_task_executor.go:382`: `Stamp`, `ScheduleAttempt`), `executeActivityRetryTimerTask` (`:540`: `Stamp`, `Attempt`, `StartedEventId`), `recordworkflowtaskstarted.Invoke` (`RequestId` dedupe, `TaskAlreadyStarted`), `Starter.handleConflict` (`RequestId` dedupe for starts). `queues/executable.go:451 isInvalidTaskError` turns `ErrStaleReference`/`NotFound` into "drop, ack".

**Code.** as listed; plus `common/backoff` for client-side retries (`docs/architecture/retry.md`) and `client/history/retryable_client_gen.go`.

**Concrete failure.** Transfer queue checkpointed ack level 100; task 105 (`AddWorkflowTask`) executed; host dies; new owner re-runs 101–105. Task 105 finds `GetWorkflowTaskByID` returns the same pending task with the same `Stamp` → it calls `AddWorkflowTask` **again**; Matching may now hold two copies; the first poller's `RecordWorkflowTaskStarted` succeeds, the second gets `TaskAlreadyStarted` and the copy is dropped. No double execution.

**Wrong model.** "Idempotency is handled once at the API." It is handled at *every* hop, by comparing identities against the single owner's state.

**Check question.** List three different identities used for staleness checks and, for each, where it is incremented.

**Harness comparison.** If your agent harness ever persists "tool call issued" and retries after a crash, you need the same "attempt id in the record, compare before applying result" discipline.

---

## D9. Namespace boundaries, routing, and multi-tenancy

**Problem.** Many tenants share a cluster; requests must be scoped, validated and limited per tenant; and namespace metadata must be available on every host without a lookup per request.

**Invariant.** Every request resolves to a registered namespace whose ID is part of the routing key and the mutable-state record; per-namespace policies (rate limits, retention, dynamic config) apply by that identity.

**Mechanism.** `namespace.Registry` (`common/namespace/nsregistry/registry.go`: `Start` :216, `refreshNamespaces` :558, `GetNamespace` :327, `GetNamespaceByID` :347) caches metadata and refreshes; Frontend interceptors `NamespaceValidatorInterceptor`, `NamespaceRateLimitInterceptor`, `ConcurrentRequestLimitInterceptor` (`common/rpc/interceptor/`); dynamic config supports `namespace:` constraints (`config/dynamicconfig/README.md`); History checks namespace state in `errorByNamespaceStateLocked` (`shard/context_impl.go`). Cross-cluster replication and handover (`NamespaceHandoverInterceptor`, `Redirection`) — defer.

**Code.** as above; `common/namespace/namespace.go`.

**Concrete failure.** Namespace deleted while a workflow is running: `errorByNamespaceStateLocked` rejects writes; the `service/worker/deletenamespace` system workflow tears down executions. Defer details.

**Wrong model.** "Namespace is just a string prefix." It is a first-class entity with ID, state, retention, replication config and its own cache lifecycle.

**Check question.** Where does a request's namespace *name* get turned into an *ID*, and why does History key everything by ID rather than name?

---

## D10. Visibility/search as distinct from authoritative state

**Problem.** "List all running workflows for customer X" cannot be answered from per-shard execution rows without scanning everything.

**Invariant.** Visibility is a derived, eventually-consistent index fed by visibility tasks; it never gates a workflow's correctness.

**Mechanism.** Mutable state generates `StartExecutionVisibilityTask` / `UpsertExecutionVisibilityTask` / `CloseExecutionVisibilityTask` / `DeleteExecutionVisibilityTask` (`service/history/tasks/*visibility_task.go`, `task_generator.go:410 GenerateRecordWorkflowStartedTasks`, `:707 GenerateUpsertVisibilityTask`); the visibility queue executor (`service/history/visibility_queue_task_executor.go:73`) writes through `VisibilityManager` (`common/persistence/visibility/manager`) to SQL or Elasticsearch (`common/persistence/visibility/store/{sql,elasticsearch}`, factory `visibility/factory.go`). Search attributes are mapped/validated in `common/searchattribute`.

**Code.** plus `service/frontend/workflow_handler.go` List/Count/Describe handlers.

**Concrete failure.** Elasticsearch is down for an hour. Workflows keep running; visibility tasks retry per `executable.HandleErr`; the list API is stale; once ES recovers the backlog drains. `tests/advanced_visibility_test.go` exercises the path.

**Wrong model.** "Describe/List reads mutable state." `DescribeWorkflowExecution` goes to History; `ListWorkflowExecutions` goes to visibility. They can disagree briefly.

**Check question.** Which task category carries visibility updates, and what happens to a `CloseExecutionVisibilityTask` if it is executed twice?

---

## D11. Dynamic configuration and safe operations

**Problem.** Operators need to tune limits, enable features and mitigate incidents without restarts, scoped to a namespace or a task queue.

**Invariant.** A dynamic config value is looked up at use-time through a typed setting with a default, and constraint precedence (`namespace`, `taskQueueName`, `taskType`) is exact-match on the constraint set.

**Mechanism.** Settings declared in `common/dynamicconfig/constants.go` with constructors such as `NewGlobalIntSetting`, `NewNamespaceBoolSetting`, `NewTaskQueueDurationSetting`; read through a `Collection` bound to a `Client` (file-based `FileBasedClient` with `PollInterval` ≥ 5s, `file_based_client.go:20`; in-memory client for tests). Services capture typed accessors into config structs (e.g. `service/matching/config.go`, `service/history/configs`). Functional tests override with `FunctionalTestBase.OverrideDynamicConfig` (`tests/testcore/functional_test_base.go:657`).

**Code.** `common/dynamicconfig/{setting.go,collection.go,file_based_client.go,constants.go}`, `config/dynamicconfig/README.md`, `config/dynamicconfig/development-sql.yaml`.

**Concrete failure.** Runaway namespace floods `StartWorkflowExecution`. Operator sets `frontend.namespaceRPS` (`FrontendMaxNamespaceRPSPerInstance`, `constants.go:768`) with a `namespace:` constraint in the dynamic config file; within one poll interval the namespace rate limiter enforces it; other tenants unaffected; no restart.

**Wrong model.** "Dynamic config is read once at startup." It is read at each use (with subscription support for some settings, e.g. `NumTaskqueueReadPartitionsSub` in `service/matching/config.go`).

**Check question.** Given two values for the same key, one with `namespace: a` and one with `namespace: a, taskQueueName: q`, which is returned for a query filtered by namespace `a` only, and why?

---

## D12. Observability: metrics, logs, traces, and debugging a failed execution

**Problem.** A failed execution spans a client, three server roles, persistence and a Worker. You need to reconstruct the path after the fact.

**Invariant.** Every RPC gets operation-tagged metrics and logs with structured tags (`tag.WorkflowID`, `tag.WorkflowRunID`, `tag.ShardID`, `tag.TaskID`, ...), and the durable history itself is the primary debugging artifact.

**Mechanism.** `metrics.Handler` (`common/metrics/metrics.go:20`), definitions in `common/metrics/metric_defs.go` (`ServiceRequests` :655, `ServiceLatency`, `TaskFailures` :901, `TaskAttempt`, `PersistenceLatency` :1749, `ShardContextAcquisitionLatency` :828); `TelemetryInterceptor` (`common/rpc/interceptor/telemetry.go`) emits per-RPC metrics; `log.Logger` (`common/log/interface.go:18`) with `tag` package (`common/log/tag/tags.go`); OTEL spans via `common/telemetry/grpc.go` stats handlers and task spans in `queues/executable.go` (`queue.task.type` attribute) and Matching (`WorkerTaskIDKey` correlation in `AddWorkflowTask`); `docs/development/tracing.md`. Task-level failure classification and DLQ: `executable.HandleErr`, `docs/admin/dlq.md`. Tooling: `tdbg` (`cmd/tools/tdbg`) for reading shards/tasks/DLQ.

**Concrete failure.** A workflow is "stuck": history ends at `WorkflowTaskScheduled`. Path: check Matching backlog/pollers (`DescribeTaskQueue`), then History logs for `"Fail to process task"` / `"Critical error processing task, retrying."` with `task-category=transfer` and the workflow ID tag, then `TaskFailures`/`TaskAttempt` metrics, then DLQ (`tdbg dlq`).

**Wrong model.** "Logs are the primary record." History is. Logs and metrics tell you *why the server did not advance* history.

**Check question.** Name the log tag and the metric you would use to find a transfer task that has been retried 50 times, and where in code that log line is emitted.
