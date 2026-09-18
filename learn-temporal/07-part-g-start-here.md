# Part G — Interactive starting point

## 1. Recommended first 90 minutes (six items, in order)

| # | Read | Time | Questions to answer while reading |
|---|---|---|---|
| 1 | `docs/architecture/README.md` | 10 min | Which two kinds of process exist? Which one runs user code? What are the two task types and what does each return to the server? |
| 2 | `docs/architecture/history-service.md` | 20 min | What is a "state transition" and what three things does it write? What is a transfer task vs a timer task? Why is mutable state persisted rather than recomputed? |
| 3 | `docs/architecture/workflow-lifecycle.md` (steps 1–3 only) | 10 min | In step 2, who calls `RecordWorkflowTaskStarted`, and what is persisted before the Worker sees the task? |
| 4 | `service/frontend/workflow_handler.go:550-720` | 15 min | Which validations happen here? What does the Frontend *not* do (hint: it never touches persistence)? How does it find History? |
| 5 | `service/history/api/startworkflow/api.go:186-330` | 20 min | In `Invoke`, in what order are: mutable state built, workflow-ID lock taken, persistence written, conflicts handled? What is a `CurrentWorkflowConditionFailedError` used for? |
| 6 | `service/history/shard/context_impl.go:538-600` and `:1501-1550` | 15 min | Where is `RangeID` attached to the request? Which error classes mean "definitely not written" vs "unknown"? What does the shard do on "unknown"? |

## 2. Five invariants to keep in mind

1. **Single owner, fenced by lease.** `(namespaceID, workflowID)` hashes to one shard (`common.WorkflowIDToHistoryShard`); every write carries the shard's `RangeID`; persistence rejects stale owners (`ShardOwnershipLostError`).
2. **Events, mutable state, and tasks commit together or not at all.** One `CreateWorkflowExecution`/`UpdateWorkflowExecution` call carries the event batch, the snapshot/mutation, and the generated tasks; the mutable-state version condition (`DBRecordVersion`) makes it optimistic-concurrency safe.
3. **"Started" is decided by History, not Matching.** A Worker only holds a task after `Record*TaskStarted` has durably appended the started event (and a start-to-close timer); duplicate deliveries are rejected there.
4. **Every asynchronous task is at-least-once and self-validating.** Outbox re-execution, Matching re-offer and client retries are all expected; consumers compare `ScheduledEventID`, `Attempt`, `Stamp`, version, `RequestId` against mutable state and drop what is stale (`ErrStaleReference` → ack).
5. **Timers are rows, not goroutines; activity retries are mutable state, not history.** A wait survives every process; a retried attempt leaves no event until the final outcome.

## 3. Three things not to learn yet

1. **CHASM (`chasm/`) and the `Archetype` parameters threaded through mutable state and queues.** It generalizes the machinery you are learning. Until you can narrate the workflow-only path, every `chasm.*` symbol is noise; treat `chasm.WorkflowArchetypeID` as a constant meaning "this is a workflow".
2. **Replication / multi-cluster (`ndc`, `xdc`, `NamespaceHandoverInterceptor`, `Redirection`, standby executors, versioned transitions).** It doubles the state machine (active vs passive) and the error classes. The single-cluster invariants above are prerequisites for understanding why replication is shaped as it is.
3. **Matching internals beyond engine/partition/matcher (forwarding trees, fairness/priority backlogs, worker versioning and deployments, partition auto-scaling).** These are optimizations and policy layers on top of the pull model; they do not change the ownership boundary and will make more sense after Session 6.

## 4. My first checkpoint

Before we continue, explain the `StartWorkflowExecution` path in your own words. Cover, in order: what the Frontend does and does not do; how the owning History shard is found; what `Starter.Invoke` builds in memory and what it writes; what protects that write against a concurrent start with the same workflow ID and against a stale shard owner; and how the resulting workflow task reaches a Worker, naming the callback that makes it "started". Keep it under 20 sentences. I will not give the answer until you respond, and I will then ask the Socratic questions from Session 3 and Session 4 in `05-part-e-apprenticeship-plan.md`.

## 5. Ambiguities and version-specific findings in this checkout

1. **Architecture docs pin an older commit.** `docs/architecture/history-service.md` and `workflow-lifecycle.md` link to `ef49189…` / `28dd23a…`. Several anchors have moved: `Start()` is `history_engine.go:377` (doc says L288); `GetAndUpdateWorkflowWithNew` is `update_workflow_util.go:14` (doc L37); the "workflow task handler" is now `service/history/api/respondworkflowtaskcompleted/workflow_task_completed_handler.go` (`workflowTaskCompletedHandler.handleCommands` at :172); transfer `processWorkflowTask` is at :289 (doc L217). The concepts still match.
2. **`AGENTS.md` lists a `/components` directory ("nexus components") that does not exist** in this checkout (`ls components` fails). Treat that line as stale.
3. **Legacy-vs-new concurrency conditions coexist.** `persistence.WorkflowMutation` has both `Condition`/`NextEventID` (marked "TODO deprecate") and `DBRecordVersion`; `MutableStateImpl` has both `nextEventIDInDB` and `dbRecordVersion`. Which one a backend enforces is backend-specific; read the store when it matters.
4. **`ReturnNewWorkflowTask` is effectively always true.** Comment in `respondworkflowtaskcompleted/api.go` (~line 579): "All current SDKs always set ReturnNewWorkflowTask to true … flag needs to be removed." Do not design around the false branch.
5. **Transient workflow task type enum is unused.** `speculative-workflow-task.md`: `WORKFLOW_TASK_TYPE_TRANSIENT` "is currently not used. Instead, `ms.IsTransientWorkflowTask()` checks if the attempts count > 1."
6. **Activity `Stamp` increment on retry is gated by dynamic config** `EnableActivityRetryStampIncrement` (`common/dynamicconfig/constants.go:245`, used in `RetryActivity`). Stale-attempt rejection therefore relies on `Attempt` plus (optionally) `Stamp`, depending on configuration.
7. **Eager workflow start and eager activity execution are server-enabled by default.** `EnableEagerWorkflowStart` (`common/dynamicconfig/constants.go:250`) and `EnableActivityEagerExecution` (`:209`) both default to `true`. They only take effect when the SDK request asks for it (`RequestEagerExecution` on the start request, checked in `Starter.requestEagerStart`; `attr.RequestEagerExecution` in `handleCommandScheduleActivity`). My trace assumes the client did not request eager execution, so the first task goes through Matching; with eager start the first workflow task is returned inline in the start response (`tests/eager_workflow_start_test.go`).
8. **Functional-test persistence defaults differ by entry point.** Makefile `functional-test` uses `PERSISTENCE_TYPE=nosql`/`cassandra`; `go test ./tests` directly defaults to `sql`/`sqlite` (`tests/testcore/flag.go:20-21`).
9. **Three matching backlog implementations** run under the same engine tests (`TestMatchingEngine_Classic_Suite`, `_Pri_Suite`, `_Fair_Suite`; `pri_*`, `fair_*` files). Which one a cluster uses is configuration; I did not trace the selection.
10. **History-node append order inside `executionManagerImpl.CreateWorkflowExecution`** was not traced line-by-line; the "events first, then mutable state + tasks" ordering is the architecture doc's statement (`history-service.md` "State transitions"), consistent with what I observed but not independently verified here.
11. **Versions**: `go.temporal.io/api` is a pre-release pseudo-version (`v1.63.6-0.20260909…`), so public API types may be ahead of the last tagged API release; `go.temporal.io/sdk v1.48.0` is used only for the internal worker service and tests.
12. **Generated files that look hand-written**: `client/history/client_gen.go`, `client/matching/client_gen.go`, `*/metric_client_gen.go`, `*/retryable_client_gen.go`, `common/rpc/interceptor/routing_key_extractor_gen.go`, `common/dynamicconfig/setting_gen.go`, all `*_mock.go`, everything under `api/`. Read them only to confirm routing keys (e.g. which request field selects the shard).
