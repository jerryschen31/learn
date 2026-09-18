# Part E — Twelve-session apprenticeship plan

Each session is 60–120 minutes. Files are ordered by importance; stop when time is up. "Stop here" lists what you do *not* need yet. Every session has a manual trace, an explain-it-back checkpoint, Socratic questions (for me to ask you later), and a small exercise. Commands assume the repo root and `-tags test_dep` (per `AGENTS.md`).

Run a single unit test like this (pattern from `CONTRIBUTING.md`, adjusted for build tags):

```
go test -tags test_dep ./service/history -run TestTransferQueueActiveTaskExecutorSuite -testify.m TestProcessWorkflowTask_FirstWorkflowTask -v
```

Functional tests default to `PERSISTENCE_TYPE=nosql PERSISTENCE_DRIVER=cassandra` **when run through the Makefile**; when you call `go test ./tests` directly the flags default to `sql`/`sqlite` (`tests/testcore/flag.go:20-21`), so on a laptop:

```
go test -tags test_dep ./tests -run TestWorkflowTaskTestSuite -persistenceType=sql -persistenceDriver=sqlite -v
```

(Flag names observed in `Makefile` `functional-test` target; SQLite support observed in `common/persistence/tests/sqlite_test.go` and `config/development-sqlite.yaml`. If a suite requires ES or Cassandra it will say so.)

---

## Session 1 — Topology and the shape of a request
**Question answered:** What processes exist, and which one owns what?
**Prereqs:** none.
**Read:**
1. `docs/architecture/README.md`
2. `docs/architecture/history-service.md`
3. `docs/architecture/workflow-lifecycle.md`
4. `temporal/server.go` (`DefaultServices`) and skim `temporal/fx.go:526-635` (the four `*ServiceProvider` funcs, only their names)
5. `cmd/server/main.go` (only where `temporal.NewServer` is called)
6. `docs/architecture/matching-service.md`
**Concepts/symbols:** Frontend/History/Matching/Worker roles; `primitives.ServiceName`; transfer vs timer queues; transactional outbox.
**Manual trace:** On paper, draw the boxes for the "StartWorkflowExecution" sequence diagram in `workflow-lifecycle.md` and label which box owns durable state.
**Explain back:** In ≤10 sentences, what is a "state transition" in the History service and why is it one persistence transaction?
**Socratic:** (1) Why is Matching not the source of truth for whether a task is still needed? (2) If Frontend crashes mid-request, what state could be inconsistent? (3) Why are internal task queues different from user task queues? (4) What does "user code runs in environments owned by the user" force on the server design?
**Exercise:** `make bins` then `make start` (SQLite) and `temporal operator namespace create -n default`; run a hello-world sample from samples-go; open the UI at `localhost:8080` if you started dependencies; screenshot the history and identify events 1–3.
**Stop here:** fx internals, config loading, replication, Nexus, CHASM.
**Contribution link:** none yet; you are building vocabulary.

## Session 2 — Frontend: the boundary where requests are rejected
**Question:** What must be true before a request is allowed to touch state?
**Prereqs:** S1.
**Read:**
1. `service/frontend/workflow_handler.go:550-720` (`StartWorkflowExecution`, `prepareStartWorkflowRequest`)
2. `service/frontend/fx.go:243-330` (`GrpcServerOptionsProvider`, interceptor order)
3. `common/rpc/interceptor/namespace_validator.go`
4. `common/rpc/interceptor/rate_limit.go`
5. `service/frontend/validators.go` (skim)
6. `service/frontend/workflow_handler_test.go` (`TestStartWorkflowExecution_Failed_*`)
**Concepts/symbols:** interceptor chain, `namespace.Registry.GetNamespaceID`, `tqid.NormalizeAndValidateUserDefined`, `common.CreateHistoryStartWorkflowRequest`, `historyClient`.
**Manual trace:** Follow one invalid request (empty task queue) from gRPC entry to the returned error; name each interceptor it passed.
**Explain back:** Why does `SetDefaultWorkflowIDPolicies` run before validation, and what would go wrong on a client retry if it ran after?
**Socratic:** (1) Which interceptor must be outermost and why? (2) Where is the namespace *state* (vs existence) checked? (3) Why do both Frontend and History compute the shard? (4) What is "redirection" and why is it above telemetry?
**Exercise:** Run `TestWorkflowHandlerSuite/TestStartWorkflowExecution_Failed_TaskQueueNotSet` and then write (locally, don't commit) one more negative test for a validator you read (e.g. workflow type too long) and make it pass.
**Stop here:** authorization claim mappers, Nexus HTTP handlers, admin/operator handlers.
**Contribution link:** validators and their tests are a classic newcomer area (clear inputs/outputs).

## Session 3 — History StartWorkflow: mutable state is born
**Question:** What exactly is written when a workflow starts?
**Prereqs:** S1, S2.
**Read:**
1. `service/history/handler.go:642-671`
2. `service/history/api/startworkflow/api.go:92-330` (`NewStarter`, `Invoke`, `prepareNewWorkflow`, `createBrandNew`)
3. `service/history/api/create_workflow_util.go:45-200` (`NewWorkflowWithSignal`)
4. `service/history/workflow/task_generator.go:129-200, 424-470`
5. `service/history/workflow/context.go:508-575` (`CreateWorkflowExecution`)
6. `service/history/historybuilder/history_builder_test.go` (`TestWorkflowExecutionStarted`, `TestWorkflowTaskScheduled`)
**Concepts/symbols:** `MutableState`, `AddWorkflowExecutionStartedEvent`, `AddFirstWorkflowTaskScheduled`, `CloseTransactionAsSnapshot`, `WorkflowSnapshot`, `WorkflowEvents`, `tasks.WorkflowTask`, `CreateWorkflowModeBrandNew`.
**Manual trace:** Write the list of persisted artifacts after a successful start: events, mutable-state fields, tasks by category.
**Explain back:** Explain `CloseTransactionAsSnapshot` in your own words: what goes in, what comes out, what it does *not* do.
**Socratic:** (1) Why is the workflow-ID lock taken *after* building mutable state? (2) What happens to the history nodes if the current-execution insert fails? (3) When is a `SCHEDULE_TO_START` timer created for the first workflow task? (4) What is `Stamp` for?
**Exercise:** Run `go test -tags test_dep ./service/history/historybuilder -run TestHistoryBuilderSuite -v` and read the assertions for `WorkflowTaskScheduled`.
**Stop here:** eager start, signal-with-start, id-reuse policies beyond "fail if running", CHASM archetypes.
**Contribution link:** `service/history/api/*` packages are small and well-tested; good place for targeted tests.

## Session 4 — Shards, RangeID, and conditional writes
**Question:** How does the server stop two hosts from writing the same execution?
**Prereqs:** S3.
**Read:**
1. `common/util.go:416-427` (`WorkflowIDToHistoryShard`)
2. `service/history/shard/context_impl.go:538-600` (`CreateWorkflowExecution`) and `:1501-1550` (`handleWriteErrorLocked`)
3. `service/history/shard/controller_impl.go:159-230, 375-450`
4. `common/persistence/data_interfaces.go:105-160` (error types) and `:1116-1170` (`ShardManager`, `ExecutionManager`)
5. `common/persistence/sql/shard.go:124-170` (`lockShard`) — or Cassandra `mutable_state_store.go:383-470`
6. `service/history/shard/context_test.go` (`TestAcquireShard*`), `common/persistence/tests/shard.go`
**Concepts/symbols:** `RangeID`, `ShardOwnershipLostError`, `CurrentWorkflowConditionFailedError`, `DBRecordVersion`, `contextRequestLost` / `contextRequestStop`, `membership.ServiceResolver`.
**Manual trace:** Two hosts both believe they own shard 3. Walk each host's `CreateWorkflowExecution` through `handleWriteErrorLocked` and state the outcome.
**Explain back:** Distinguish "membership says I own it" from "persistence says I own it".
**Socratic:** (1) Why does a timeout error trigger shard re-acquire but a condition-failed error does not? (2) What does `AssertOwnership` add over the write-time check? (3) Why are task IDs allocated inside the shard lock? (4) Is `numHistoryShards` dynamic config? Prove it.
**Exercise:** Run `TestShardContextSuite/TestAcquireShardOwnershipLostErrorIsNotRetried` and `TestAcquireShardEventuallySucceeds`; annotate which mock expectations model persistence.
**Stop here:** ringpop internals, shard linger, replication readers, `MapShardID` resharding.
**Contribution link:** ownership/queues are owned by `@temporalio/oss-foundations` per `.github/CODEOWNERS`; observability improvements here need care.

## Session 5 — The outbox: transfer queue to Matching
**Question:** How does a persisted row become an RPC, safely, more than once?
**Prereqs:** S3, S4.
**Read:**
1. `service/history/transfer_queue_active_task_executor.go:106-190, 289-375`
2. `service/history/transfer_queue_task_executor_base.go:95-200`
3. `service/history/queues/executable.go:273-330, 451-500, 584-712`
4. `service/history/queues/queue_immediate.go:91-175`
5. `service/history/history_engine.go:377-395` (Start)
6. `service/history/transfer_queue_active_task_executor_test.go` (`TestProcessWorkflowTask_Duplication`, `TestProcessWorkflowTask_StampMismatch`), `service/history/queues/executable_test.go`
**Concepts/symbols:** `Executable`, `HandleErr`, `isInvalidTaskError`, `ErrStaleReference`, `CheckTaskVersion`, `NotifyNewTasks`, ack level checkpoint, DLQ (`HistoryTaskDLQEnabled`).
**Manual trace:** Replay of tasks 101–105 after a shard restart; for each, decide drop/execute.
**Explain back:** Why does `processWorkflowTask` release the workflow lock before calling Matching?
**Socratic:** (1) Which errors ack a task without executing it? (2) What is the difference between `Nack` and `Reschedule`? (3) How does a task reach the DLQ? (4) Why is `TaskQueue` stored on the transfer task instead of read from mutable state?
**Exercise:** Run the two named transfer-executor tests with `-v`; then add a temporary `t.Log` (do not commit) printing `transferTask.Stamp` vs `workflowTask.Stamp` in the mismatch test to see the values.
**Stop here:** reader/slice/scope machinery, priority assigner, mitigator, standby executors, replication queue.
**Contribution link:** `docs/admin/dlq.md` cross-references log lines in `executable.go`; docs/observability improvements live here.

## Session 6 — Matching: pull, sync-match, and the callback to History
**Question:** How does a task reach exactly one Worker as "started"?
**Prereqs:** S5.
**Read:**
1. `service/matching/matching_engine.go:586-700` (`AddWorkflowTask`, start of `PollWorkflowTaskQueue`) and `:790-870` (result handling), `:3502-3580` (`recordWorkflowTaskStarted`)
2. `service/matching/task_queue_partition_manager.go:555-620, 727-800`
3. `service/matching/physical_task_queue_manager.go:470-500, 730-760`
4. `service/matching/matcher.go:108-260, 394-410`
5. `service/history/api/recordworkflowtaskstarted/api.go:35-200`
6. `service/matching/matching_engine_test.go` (`TestPollWorkflowTaskQueues`, `TestConcurrentPublishConsumeWorkflowTasks`), `service/history/history_engine2_test.go` (`TestRecordWorkflowTaskStartedIfTaskAlreadyStarted`)
**Concepts/symbols:** `tqid.Partition`, `internalTask`, sync match vs spool, `taskWriter`/`taskReader`, `TaskAlreadyStarted`, `RequestId` dedupe, task token.
**Manual trace:** Task spooled on partition 1, poller on partition 2; state every hop until the Worker holds a token.
**Explain back:** What does "started" mean durably, and which service decides it?
**Socratic:** (1) Why doesn't `AddWorkflowTask` load a sticky queue that isn't already loaded? (2) What does Matching do on `NotFound` from History? (3) Where is long-poll timeout configured? (4) What is a task ID block lease for?
**Exercise:** Run `TestMatchingEngine_Classic_Suite/TestPollWorkflowTaskQueues -v`; find the mocked History call and note what response fields the poll response needs.
**Stop here:** forwarding math, fairness/priority backlog (`fairness.md`), worker versioning, partition auto-scaling.
**Contribution link:** `client/matching`, `common/tqid` are owned by `@temporalio/oss-matching`; matching has many focused unit tests to extend.

## Session 7 — Commands: RespondWorkflowTaskCompleted and ScheduleActivityTask
**Question:** How do Worker decisions become state, exactly once?
**Prereqs:** S3, S6.
**Read:**
1. `service/history/api/respondworkflowtaskcompleted/api.go:112-420`
2. `service/history/api/respondworkflowtaskcompleted/workflow_task_completed_handler.go:172-330, 470-560`
3. `service/history/workflow/workflow_task_state_machine.go:407-520, 761-800, 1050-1110`
4. `service/history/workflow/mutable_state_impl.go:4305-4340` (`AddActivityTaskScheduledEvent`) and `:7775-7900` (`closeTransaction`)
5. `service/history/api/update_workflow_util.go` and `service/history/workflow/context.go:695-760`
6. `service/history/api/respondworkflowtaskcompleted/api_test.go`, `tests/workflow_failures_test.go` (`TestRespondWorkflowTaskCompletedReturnsErrorIfInvalidArgument`)
**Concepts/symbols:** task token, `GetWorkflowLeaseWithConsistencyCheck`, `handleCommands`, `failWorkflowTask`, `UpdateWorkflowExecutionAsActive`, `WorkflowMutation`, `ReturnNewWorkflowTask`.
**Manual trace:** Worker sends the same completion twice (network retry). Trace both.
**Explain back:** What is a "mutable state transaction" versus a "database transaction"?
**Socratic:** (1) Why is `WorkflowTaskCompleted` added before commands are handled? (2) What causes a command to *fail the workflow task* rather than the workflow? (3) When does the server schedule the next workflow task immediately? (4) How does sticky affect attempt counting on failure?
**Exercise:** Run `TestWorkflowTaskCompletedHandlerSuite -v`; then run `tests/workflow_task_test.go` `TestWorkflowTaskHeartbeatingWithEmptyResult` with SQLite and read how `TaskPoller` plays the Worker.
**Stop here:** workflow update/messages (`workflow-update.md`, `message-protocol.md`), queries, speculative tasks beyond the definition, child workflows, continue-as-new.
**Contribution link:** command attribute validation (`command_attr_validator.go`) has tests per command — narrowly bounded fixes/tests fit here.

## Session 8 — Activities: dispatch, start, complete, fail, retry, timeout
**Question:** How does the server retry side effects it cannot observe?
**Prereqs:** S5–S7.
**Read:**
1. `service/history/transfer_queue_active_task_executor.go:234-288`
2. `service/history/api/recordactivitytaskstarted/api.go:38-260`
3. `service/history/api/respondactivitytaskfailed/api.go:23-130`
4. `service/history/workflow/mutable_state_impl.go:6879-6990` (`RetryActivity`) and `service/history/workflow/retry.go`
5. `service/history/timer_queue_active_task_executor.go:204-380, 540-660`
6. `service/history/timer_queue_active_task_executor_test.go` (`TestProcessActivityTimeout_RetryPolicy_Retry`, `TestActivityRetryTimer_Fire`), `tests/activity_test.go` (`TestActivityRetry`)
**Concepts/symbols:** `ActivityInfo`, `Attempt`, `Stamp`, `RequestId`, `RETRY_STATE_*`, `ActivityRetryTimerTask`, `TimerSequence`, `ErrActivityTaskNotFound`.
**Manual trace:** Attempt 1 times out at start-to-close; attempt 2 succeeds; attempt-1 Worker reports completion late.
**Explain back:** Which activity transitions produce history events and which do not, and why.
**Socratic:** (1) Why does the retry timer call Matching directly instead of writing a transfer task? (2) What does `EnableActivityRetryStampIncrement` change? (3) What distinguishes `RETRY_STATE_TIMEOUT` from `NON_RETRYABLE_FAILURE`? (4) Where is the heartbeat timeout evaluated?
**Exercise:** Run `TestTimerQueueActiveTaskExecutorSuite/TestProcessActivityTimeout_RetryPolicy_Retry -v` and list the mutable-state mutations the test asserts.
**Stop here:** activity pause/reset/update APIs, standalone activities (`tests/activity_standalone_*`), worker commands (`worker-commands.md`).
**Contribution link:** retry math in `workflow/retry.go` is pure and testable.

## Session 9 — Timers and Worker crash recovery
**Question:** What happens when nobody tells the server anything?
**Prereqs:** S7, S8.
**Read:**
1. `service/history/timer_queue_active_task_executor.go:77-145, 382-480`
2. `service/history/workflow/task_generator.go:519-552` (`GenerateStartWorkflowTaskTasks`)
3. `service/history/queues/queue_scheduled.go:125-260`
4. `service/history/workflow/timer_sequence.go:78-220`
5. `docs/architecture/speculative-workflow-task.md` (transient section only)
6. `tests/transient_task_test.go` (`TestTransientWorkflowTaskTimeout`), `tests/stickytq_test.go`, `timer_queue_active_task_executor_test.go` (`TestWorkflowTaskTimeout_*`)
**Concepts/symbols:** `WorkflowTaskTimeoutTask{START_TO_CLOSE|SCHEDULE_TO_START}`, `ScheduleAttempt`, transient WFT, sticky clear, `StickyWorkerUnavailable`.
**Manual trace:** Worker A polls and dies; Worker B recovers. Write the history events and non-events.
**Explain back:** Why is the second workflow task "transient" and what does the Worker see?
**Socratic:** (1) Why is there no schedule-to-start timer for a normal (non-sticky) queue? (2) What makes a fired timer task a no-op? (3) How does a timer that was due during a shard outage get handled? (4) What is the in-memory timer queue for?
**Exercise:** Run `tests` `TestTransientTaskSuite/TestTransientWorkflowTaskTimeout` with SQLite and read the asserted history.
**Stop here:** speculative tasks themselves, time-skipping (`timeskipping.go`), workflow run/execution timeouts, cron/backoff timers.
**Contribution link:** timer executor tests are table-like; adding a case for a scenario you traced is a realistic first PR shape.

## Session 10 — Persistence contract and one backend
**Question:** What does the storage layer promise, and how does one backend keep it?
**Prereqs:** S4.
**Read:**
1. `common/persistence/data_interfaces.go:105-160, 225-400, 1116-1220`
2. `common/persistence/execution_manager.go:105-200`
3. `common/persistence/sql/execution.go:60-200` and `sql/execution_util.go:575-620` (or the Cassandra pair)
4. `common/persistence/history_manager.go:482-560`
5. `schema/sqlite/v3/temporal/schema.sql` (executions, current_executions, history_node, transfer_tasks, timer_tasks tables)
6. `common/persistence/tests/execution_mutable_state.go` (`TestCreate_BrandNew_CurrentConflict`, `TestUpdate_NotZombie_Conflict`), `common/persistence/tests/sqlite_test.go`
**Concepts/symbols:** `ExecutionStore` vs `ExecutionManager` (serialization boundary), `WorkflowSnapshot`/`WorkflowMutation`, `Condition`/`DBRecordVersion`, history branches/`BranchToken`, task categories.
**Manual trace:** One `UpdateWorkflowExecution` with a new event batch: list every table touched in SQLite and the condition on each.
**Explain back:** Why is history stored in a separate table/tree from mutable state, and what links them?
**Socratic:** (1) What is a "zombie" execution in the tests? (2) What is `Condition` and what replaces it? (3) Why does the manager serialize before the store? (4) What's `TransactionSizeLimitError` for?
**Exercise:** Run `go test -tags test_dep ./common/persistence/tests -run TestSQLiteExecutionMutableStateStoreSuite -v` (suite name observed in `common/persistence/tests/sqlite_test.go:98`) and map one failing-condition test to the SQL that enforces it.
**Stop here:** Cassandra LWT tuning, history branching/reset, replication task stores, queue v2, fault injection.
**Contribution link:** persistence changes require multi-backend tests; understand before proposing.

## Session 11 — Configuration, namespaces, visibility
**Question:** How is behavior scoped and tuned per tenant, and what is derived vs authoritative?
**Prereqs:** S2, S5.
**Read:**
1. `config/dynamicconfig/README.md` and `common/dynamicconfig/setting.go` (constructors), `collection.go` (lookup)
2. `common/dynamicconfig/constants.go` (search: `FrontendRPS`, `MatchingNumTaskqueueReadPartitions`, `HistoryTaskDLQEnabled`)
3. `common/namespace/nsregistry/registry.go:216-360, 558-620`
4. `service/history/visibility_queue_task_executor.go:73-300`
5. `common/persistence/visibility/factory.go`
6. `common/dynamicconfig/collection_test.go`, `service/history/visibility_queue_task_executor_test.go`
**Concepts/symbols:** constraints (namespace/taskQueueName/taskType), `FileBasedClient` poll, `Registry` refresh, visibility task categories, dual visibility manager.
**Manual trace:** Change a namespace-scoped RPS limit in the dynamic config file; trace the code path from file change to enforcement.
**Explain back:** Why is visibility allowed to lag, and what user-visible symptom does lag cause?
**Socratic:** (1) How would a test override a dynamic config value? (2) What does the namespace registry refresh from? (3) Which visibility store does `development-sqlite.yaml` use? (4) Is a search attribute stored in mutable state, visibility, or both?
**Exercise:** In a functional test you already ran, use `OverrideDynamicConfig` to change `MatchingLongPollExpirationInterval` and observe the effect on test duration (local experiment only).
**Stop here:** Elasticsearch mapping/index management, archival, namespace replication, custom search attribute aliasing.
**Contribution link:** dynamic config docs and default-value comments in `constants.go` are frequently improved.

## Session 12 — Observability and debugging a stuck workflow; consolidation
**Question:** Given only a stuck workflow ID, how do you find where the server stopped?
**Prereqs:** all.
**Read:**
1. `common/metrics/metric_defs.go` (search `TaskFailures`, `TaskAttempt`, `ServiceLatency`, `PersistenceLatency`)
2. `common/rpc/interceptor/telemetry.go`
3. `common/log/tag/tags.go` (skim the workflow/task tags)
4. `docs/development/tracing.md` and `docs/admin/dlq.md`
5. `service/history/queues/executable.go:584-712` (re-read with logs/metrics in mind)
6. `cmd/tools/tdbg` (skim command list)
**Concepts/symbols:** operation tags, `tag.WorkflowID`/`tag.TaskID`/`tag.ShardID`, `OperationCritical`, DLQ, `tdbg`.
**Manual trace:** History ends at `ActivityTaskScheduled`; no Worker picks it up. Decide, in order, the three places you check and the exact log line/metric at each.
**Explain back:** Re-tell the full StartWorkflow → activity → crash story from Part C without notes, naming one file per hop.
**Socratic:** (1) Which metric distinguishes "Matching has no pollers" from "History never pushed"? (2) What log message marks a task going to the DLQ? (3) Where do you see per-namespace request rates? (4) What would you instrument first if a stamp mismatch became frequent?
**Exercise:** Start the server with SQLite, run a sample, kill the Worker mid-activity, watch the server logs for the timer-queue lines, restart the Worker, confirm completion in the UI.
**Stop here:** OTEL exporter configuration, Grafana dashboards (`develop/docker-compose`), pprof.
**Contribution link:** observability improvements (a missing tag on an existing log line, a metric on an unobserved branch) are among the most accepted small PRs, but check `CODEOWNERS` for the area.
