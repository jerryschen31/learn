# Part C — One execution, end to end (commit `736e89ab9`)

Scenario: the Go SDK client calls `StartWorkflowExecution` for `SupportAgentWorkflow`, workflow ID `ticket-42`, task queue `support`. The workflow schedules activity `CallTicketAPI`, then waits for a human-approval signal. We follow the first workflow task, the first activity, its failure/retry/timeout, and a Worker crash.

Conventions: **Observed** = read in the code. **Inference** = my explanation. **Version-specific / uncertain** flagged inline. Line numbers drift; symbol names are the anchor.

Two things I deliberately hold constant to keep the trace readable:

- No *eager* workflow task (`request.RequestEagerExecution` false). Eager start returns the first workflow task inline in the start response and skips Matching; it is real (`Starter.requestEagerStart`, `tests/eager_workflow_start_test.go`) but a variation.
- No sticky task queue on the very first workflow task (the SDK only sets one after a successful task). Sticky appears in the crash path.

---

## Stage 1 — Go SDK client sends StartWorkflowExecution

- **Where**: outside this repo. The SDK builds a `workflowservice.StartWorkflowExecutionRequest` (types from `go.temporal.io/api`) with `Namespace`, `WorkflowId`, `WorkflowType`, `TaskQueue`, `RequestId` (a UUID, used for idempotency), timeouts, id-reuse/conflict policies, and sends it over gRPC to the Frontend.
- **Input/Output**: request → `StartWorkflowExecutionResponse{RunId, Started, ...}`.
- **Invariant protected**: none yet. **Retry semantics**: the SDK retries transient gRPC errors; the `RequestId` is what makes retrying safe (see stage 5).
- **Tests**: `tests/eager_workflow_start_test.go` `TestEagerWorkflowStart_RetryStartAfterTimeout` shows a client retry with the same request ID being deduped (eager variant, same mechanism).

## Stage 2 — Request enters the Frontend API boundary

- **Handler**: `WorkflowHandler.StartWorkflowExecution` in `service/frontend/workflow_handler.go:550`. Package `frontend`.
- Before the handler body runs, the gRPC unary interceptor chain built in `GrpcServerOptionsProvider` (`service/frontend/fx.go:243`) executes. Observed order (comment in code says "Order of interceptors is important"):
  1. `MaskInternalErrorDetailsInterceptor`, `ServiceErrorInterceptor`, `NewFrontendServiceErrorInterceptor` — error shaping
  2. `RoutingKeyInterceptor` (business/workflow ID extraction into context)
  3. `NamespaceValidatorInterceptor.NamespaceValidateIntercept` — namespace exists / is valid
  4. `NamespaceLogInterceptor`, metrics context injector
  5. `authorization.Interceptor` — `Authorizer` + `ClaimMapper` (`common/authorization/`)
  6. `NamespaceHandoverInterceptor`, `Redirection` — multi-cluster forwarding (defer)
  7. `TelemetryInterceptor` — metrics + tracing per RPC
  8. `HealthInterceptor`, `NamespaceValidatorInterceptor.StateValidationIntercept`
  9. `ConcurrentRequestLimitInterceptor`, `NamespaceRateLimitInterceptor`, `RateLimitInterceptor` (`common/rpc/interceptor/rate_limit.go:46`), keyed by dynamic config such as `FrontendRPS` / `FrontendMaxNamespaceRPSPerInstance` (`common/dynamicconfig/constants.go`)
  10. `SDKVersionInterceptor`, `CallerInfoInterceptor`, `SlowRequestLoggerInterceptor`, `ChasmVisibilityInterceptor`, `ContextMetadataInterceptor`
- **Ownership boundary**: Frontend owns nothing durable. Everything it does is rejectable without side effects.
- **Tests**: `common/rpc/interceptor/rate_limit_test.go`, `namespace_validator_test.go`, `service/frontend/workflow_handler_test.go` (`TestStartWorkflowExecution_Failed_NamespaceNotSet`, `..._WorkflowIdNotSet`, `..._TaskQueueNotSet`).

## Stage 3 — Validation and defaulting

- **Function**: `WorkflowHandler.prepareStartWorkflowRequest` (`workflow_handler.go:615`). Observed steps: `enums.SetDefaultWorkflowIDPolicies` (comment: "must be first for idempotency on internal retries"), `validator.ValidateWorkflowID`, `ValidateRetryPolicy`, `ValidateWorkflowStartDelay`, `backoff.ValidateSchedule` (cron), workflow type present and ≤ `MaxIDLengthLimit`, `tqid.NormalizeAndValidateUserDefined` for the task queue, `ValidateWorkflowTimeouts`, `validateRequestId`, `ValidateWorkflowIDReusePolicy`, then more (search attributes, memo, size limits) further down.
- Then `namespaceRegistry.GetNamespaceID(namespaceName)` resolves name → ID (`common/namespace/nsregistry`).
- Frontend calls `historyClient.StartWorkflowExecution(ctx, common.CreateHistoryStartWorkflowRequest(namespaceID, request, nil, nil, now))`, which wraps the public request into the internal `historyservice.StartWorkflowExecutionRequest` (proto in `proto/internal/temporal/server/api/historyservice/v1/request_response.proto`).
- **Output**: `convertToStartWorkflowExecutionResponse` maps the internal response back.
- **Invariant**: public request shape is valid and defaults are deterministic so a retried request is byte-equivalent. **Failure**: any error here → client gets an `InvalidArgument`-class error and nothing was written.
- **Tests**: `service/frontend/validators_test.go`, `workflow_handler_test.go` (`TestStartWorkflowExecution_EnsureNonNilRetryPolicyInitialized`).

## Stage 4 — Locating the owning History shard

- **Client side**: `client/history/client_gen.go:1344` (`clientImpl.StartWorkflowExecution`, **generated** by `cmd/tools/genrpcwrappers`; do not read as design) computes `shardID := c.shardIDFromWorkflowID(namespaceID, workflowID)` → `common.WorkflowIDToHistoryShard` (`common/util.go:418`): `farm.Fingerprint32(namespaceID + "_" + workflowID) % numberOfShards + 1`. It then calls `executeWithRedirect` → `CachingRedirector.Execute` (`client/history/caching_redirector.go:94`), which resolves shard → host via `membership.ServiceResolver.Lookup` (`common/membership/interfaces.go:69`) and caches the address, redirecting on ownership-lost errors.
- **Server side**: `history.Handler.StartWorkflowExecution` (`service/history/handler.go:642`) recomputes the shard with `h.controller.GetShardByNamespaceWorkflow(namespaceID, workflowID)` (`service/history/shard/controller_impl.go:159`) and gets the engine via `shardContext.GetEngine(ctx)`. If this host does not own the shard, the controller returns a `ShardOwnershipLostError` (`ownership.verifyOwnership` inside `acquireShards`, `controller_impl.go:375`), and the client redirects.
- **Number of shards**: static config `persistence.numHistoryShards` (`common/config/config.go:266`; `config/development.yaml` uses `1`). Fixed at cluster creation (doc: "cannot be changed later"; `common.MapShardID` exists for whole-multiple resharding, defer).
- **Invariant**: exactly one shard is responsible for `(namespaceID, workflowID)`; hash is deterministic, so Frontend and History agree without coordination.
- **Failure/retry**: `ShardOwnershipLostError` is retryable at the client via redirection; the `RetryableInterceptor` and `client/history/retryable_client_gen.go` wrap other transient errors.
- **Tests**: `client/history/caching_redirector_test.go`, `service/history/shard/controller_test.go`, `tests/acquire_shard_test.go` (`TestOwnershipLost_DoesNotRetry`, `TestEventualSuccess`).

## Stage 5 — Durable state is created; first events are written

- **Entry**: `historyEngineImpl.StartWorkflowExecution` (`service/history/history_engine.go:443`) → `startworkflow.NewStarter(...)` → `Starter.Invoke` (`service/history/api/startworkflow/api.go:186`).
- **Observed sequence inside `Invoke`**:
  1. `prepare` (namespace active check, overrides).
  2. `prepareNewWorkflow` (`api.go:258`): mint `runID := primitives.NewUUID()`, build a fresh `MutableStateImpl` via `api.NewWorkflowWithSignal` (`service/history/api/create_workflow_util.go:45`), which calls `AddWorkflowExecutionStartedEvent` (`mutable_state_impl.go:3009`) and `AddFirstWorkflowTaskScheduled` (`:3589`). The latter routes through `workflowTaskStateMachine.AddWorkflowTaskScheduledEvent` (`workflow/workflow_task_state_machine.go:407`) and then `TaskGeneratorImpl.GenerateScheduleWorkflowTaskTasks` (`workflow/task_generator.go:424`), which appends a `tasks.WorkflowTask` transfer task (and, only if a sticky queue is set, a `WorkflowTaskTimeoutTask` of type `SCHEDULE_TO_START`). Then `mutableState.CloseTransactionAsSnapshot(ctx, TransactionPolicyActive)` (`mutable_state_impl.go:7652`) → `closeTransaction` (`:7775`) which flushes the `HistoryBuilder`, updates transition history, prepares tasks, bumps `StateTransitionCount`, and computes a checksum. Exactly one event batch is expected (`"unable to create 1st event batch"` soft-assert otherwise).
  3. `lockCurrentWorkflowExecution` (`api.go:239`): takes the workflow-ID-level lock through `wcache.Cache.GetOrCreateCurrentExecution` (`service/history/workflow/cache/cache.go:186`). **Inference**: this serializes concurrent starts for the same workflow ID on this shard so that the "current run" pointer is updated by one writer at a time.
  4. `createBrandNew` (`api.go:311`) → `workflow.ContextImpl.CreateWorkflowExecution` (`service/history/workflow/context.go:508`) with `persistence.CreateWorkflowModeBrandNew` → `createWorkflowExecution` in `workflow/transaction_impl.go:362` → `shard.ContextImpl.CreateWorkflowExecution` (`service/history/shard/context_impl.go:538`).
  5. In the shard: acquire IO semaphore, take the shard write lock, `errorByState` (shard must be acquired), `errorByNamespaceStateLocked`, `taskKeyManager.setAndTrackTaskKeys` (assigns task IDs from the shard's range), **`request.RangeID = s.getRangeIDLocked()`**, release lock, then `executionManager.CreateWorkflowExecution(ctx, request)`, then `handleWriteError(request.RangeID, err)`.
  6. `executionManagerImpl.CreateWorkflowExecution` (`common/persistence/execution_manager.go:105`) serializes the snapshot and, per the architecture doc, appends history nodes then writes the execution row + tasks. (**Observed**: `SerializeWorkflowSnapshot`, `m.persistence.CreateWorkflowExecution`; the history-node append happens in the same manager via `AppendHistoryNodes` at `history_manager.go:482` — I did not trace the exact call order inside this function, so treat "events first, then mutable state + tasks" as the doc's statement, consistent with `docs/architecture/history-service.md` "State transitions".)
  7. Backend: Cassandra `MutableStateStore.CreateWorkflowExecution` (`common/persistence/cassandra/mutable_state_store.go:383`) builds a logged batch including `templateUpdateLeaseQuery` with `request.RangeID` and executes `MapExecuteBatchCAS`; if not applied, `convertErrors` (`cassandra/errors.go:48`) yields `ShardOwnershipLostError` / `CurrentWorkflowConditionFailedError` / `WorkflowConditionFailedError`. SQL: `sqlExecutionStore.createWorkflowExecutionTx` (`common/persistence/sql/execution.go:81`) runs in a DB transaction that calls `lockCurrentExecutionIfExists` and the shard lock (`sql/shard.go:124 lockShard` returns `ShardOwnershipLostError` on RangeID mismatch).
- **Durable state change**: new execution row (mutable state snapshot) with `ExecutionState.Status=RUNNING`, current-execution pointer `workflowID → runID`, history events `[1: WorkflowExecutionStarted, 2: WorkflowTaskScheduled]`, one transfer task (`tasks.WorkflowTask{ScheduledEventID: 2, TaskQueue: "support"}`), plus workflow-timeout timer tasks from `GenerateWorkflowStartTasks` (`task_generator.go:129`) if run/execution timeouts are set, plus a visibility start task.
- **Duplicate/idempotency**: if the write fails with `CurrentWorkflowConditionFailedError` carrying a `RunID`, `Invoke` calls `handleConflict` (`api.go:328`) → `resolveDuplicateWorkflowID` (`api.go:447`, logic in `service/history/api/workflow_id_dedup.go`). If the existing run has the same `RequestId`, it returns the existing run (`respondToRetriedRequest`, outcome `StartDeduped`); otherwise it applies the id-reuse/conflict policy. The comment "The history and mutable state generated above will be deleted by a background process" tells you the failed create may have left orphan history nodes; that is by design.
- **Post-write**: `NotifyOnExecutionSnapshot(engine, ...)` (`context.go`) notifies queue processors that new tasks exist (`immediateQueue.NotifyNewTasks`, `queues/queue_immediate.go:124`), so the transfer queue does not wait for its poll interval.
- **Invariants protected**: (a) one current run per workflow ID (current-execution conditional insert); (b) writes are fenced by shard `RangeID`; (c) events, mutable state and tasks are committed atomically (batch/tx); (d) a retried start with the same `RequestId` is idempotent.
- **Failure semantics** (`shard.ContextImpl.handleWriteErrorLocked`, `context_impl.go:1501`): condition-failed classes = "write definitely not committed", returned to caller; `ShardOwnershipLostError` → shard stops (`contextRequestStop`); *unknown* errors (timeouts) → shard re-acquires (`contextRequestLost`) to get a new RangeID so later reads can tell whether the write landed.
- **Tests**: `common/persistence/tests/execution_mutable_state.go` (`TestCreate_BrandNew`, `TestCreate_BrandNew_CurrentConflict`, `TestCreate_Reuse`), `service/history/api/workflow_id_dedup_test.go` (`TestResolveDuplicateWorkflowStart`), `service/history/historybuilder/history_builder_test.go` (`TestWorkflowExecutionStarted`, `TestWorkflowTaskScheduled`), `service/history/shard/context_test.go` (`TestAddTasks_Success`).

## Stage 6 — The Workflow Task becomes available for dispatch (History → Matching)

- **Queue**: the shard's transfer queue, created by `transferQueueFactory.CreateQueue` (`service/history/transfer_queue_factory.go:86`) and started by `historyEngineImpl.Start` (`history_engine.go:377`, loops `e.queueProcessors`).
- **Loop**: `immediateQueue.processEventLoop` (`queues/queue_immediate.go:132`) → `ReaderImpl.loadAndSubmitTasks` (`queues/reader.go:427`) reads persisted tasks via `ExecutionManager.GetHistoryTasks` and submits `Executable`s to a scheduler in `common/tasks` (e.g. `fifo_scheduler.go:194` calls `Execute()`, then `HandleErr`, then `Ack`/`Nack`).
- **Executor**: `transferQueueActiveTaskExecutor.Execute` (`service/history/transfer_queue_active_task_executor.go:106`) → `execute` switch on task type → `processWorkflowTask` (`:289`). Observed checks: load mutable state; return nil if the workflow is not running; `GetWorkflowTaskByID(transferTask.ScheduledEventID)` nil → nil (task already consumed); `transferTask.Stamp != workflowTask.Stamp` → `consts.ErrStaleReference`; `CheckTaskVersion`. Then it **releases the workflow lock before calling Matching** (comment: Matching will call History back with `RecordWorkflowTaskStarted`, which needs the lock).
- **RPC**: `pushWorkflowTask` (`transfer_queue_task_executor_base.go:147`) → `matchingRawClient.AddWorkflowTask` with `ScheduledEventId`, `ScheduleToStartTimeout`, a `Clock` vector clock (`vclock.NewVectorClock(clusterID, shardID, task.TaskID)`), `Stamp`, `Priority`, `VersionDirective`.
- **Sticky fallback**: if Matching answers `StickyWorkerUnavailable`, retry to the normal queue (`processWorkflowTask` second `pushWorkflowTask`).
- **Retry/dup semantics** (`queues/executable.go:584 HandleErr`): `ErrStaleReference`/`NotFound` → `isInvalidTaskError` → task dropped (return nil); `ErrTaskDiscarded` → safe drop; `ResourceExhausted` variants → expected retryable; unknown errors → warn, retry with backoff, and after `maxUnexpectedErrorAttempts` (dynamic config) go to DLQ if enabled (`docs/admin/dlq.md`). Ack levels are checkpointed periodically (`queue_base.go checkpoint`), so after a shard reload tasks can be **re-executed**; the checks above are what make that safe.
- **Invariant**: every persisted transfer task eventually produces an `AddWorkflowTask` call *or* is provably obsolete.
- **Tests**: `service/history/transfer_queue_active_task_executor_test.go` (`TestProcessWorkflowTask_FirstWorkflowTask`, `TestProcessWorkflowTask_Duplication`, `TestProcessWorkflowTask_StampMismatch`, `TestProcessWorkflowTask_Sticky_NonFirstWorkflowTask`), `service/history/queues/executable_test.go` (`TestExecute_TaskExecuted`, `TestExecuteHandleErr_ResetAttempt`, `TestExecute_SendToDLQAfterMaxAttempts`).

## Stage 7 — Matching makes the task available to a polling Worker

- **Add side**: `matching.Handler.AddWorkflowTask` (`service/matching/handler.go:201`) → `matchingEngineImpl.AddWorkflowTask` (`matching_engine.go:586`): `tqid.PartitionFromProto`, load the partition manager (`getTaskQueuePartitionManager`, does **not** load sticky queues on add: "do not load sticky task queues if not already loaded, which means they have no poller"), build a `persistencespb.TaskInfo{ScheduledEventId, Clock, ExpiryTime, Stamp, Priority, ...}`, then `pm.AddTask` (`task_queue_partition_manager.go:555`). Inside, `getPhysicalQueuesForAdd` picks a spool queue and a sync-match queue; `physicalTaskQueueManagerImpl.TrySyncMatch` (`physical_task_queue_manager.go:730`) offers to a waiting poller via `TaskMatcher.Offer` (`matcher.go:108`) or the priority matcher; if no poller, `SpoolTask` (`:470`) → `taskWriter.appendTask` (`task_writer.go:85`) → persistence `TaskManager.CreateTasks`, with task IDs from a leased block (`allocTaskIDBlock`, `renewLeaseWithRetry` → `UpdateTaskQueue` with a range ID; **inference**: the same fencing idea as History shards, applied to a task-queue partition).
- **Poll side**: Worker → Frontend `WorkflowHandler.PollWorkflowTaskQueue` (`workflow_handler.go:1076`) → `matchingClient.PollWorkflowTaskQueue` (`:1132`; the Frontend generates a `pollerId` so it can cancel outstanding polls) → `matching.Handler.PollWorkflowTaskQueue` (`handler.go:254`) → `matchingEngineImpl.PollWorkflowTaskQueue` (`matching_engine.go:697`) `pollLoop` → `pollTask` (`:3067`) → `pm.PollTask` (`task_queue_partition_manager.go:727`) → physical manager → `TaskMatcher.Poll` (`matcher.go:394`) with long-poll expiration `MatchingLongPollExpirationInterval` (dynamic config) minus jitter. Backlog tasks are fed by `taskReader.dispatchBufferedTasks` (`task_reader.go:85`). Partition forwarding (`forwarder.go`) and load balancing on the client (`client/matching/loadbalancer.go`) are how a poll on partition 3 can receive a task spooled on partition 1; defer the details.
- **The important callback**: before returning a non-query, non-forwarded task to the Worker, Matching calls History: `recordWorkflowTaskStarted` (`matching_engine.go:3502`) → `historyClient.RecordWorkflowTaskStarted` → `history.Handler.RecordWorkflowTaskStarted` (`service/history/handler.go:366`, routes by workflow ID) → `historyEngineImpl.RecordWorkflowTaskStarted` (`:600`) → `recordworkflowtaskstarted.Invoke` (`service/history/api/recordworkflowtaskstarted/api.go:35`).
  - Observed in `Invoke`: look up `GetWorkflowTaskByID(scheduledEventID)`; `NotFound` if absent; `req.GetStamp() != WorkflowTaskStamp` → NotFound; if already started and `RequestId` matches → return the same response (dedupe); if already started by a different request → `serviceerrors.NewTaskAlreadyStarted("Workflow")`; otherwise `AddWorkflowTaskStartedEvent` (`workflow_task_state_machine.go:453`) which appends `WorkflowTaskStarted` (event 3) and, via `GenerateStartWorkflowTaskTasks` (`task_generator.go:519`), a `WorkflowTaskTimeoutTask` of type `START_TO_CLOSE` due at `StartedTime + WorkflowTaskTimeout`; persist via the update path; return `RecordWorkflowTaskStartedResponse` including history (or raw history) for the poll response.
  - Matching's handling of that result (`matching_engine.go:~806`): `Internal`/`DataLoss` → drop task; `NotFound` → "Workflow task not found", finish task (it was stale); `TaskAlreadyStarted` → similar; success → `createPollWorkflowTaskQueueResponse` with a **task token** (`common/tasktoken`, proto `tokenspb.Task` containing namespace/workflow/run/scheduled event ID/attempt).
- **Durable state change**: History appends `WorkflowTaskStarted`, mutable state `WorkflowTaskStartedEventId=3`, a start-to-close timer task is persisted. Matching's persisted task (if spooled) is completed via ack levels (`ackManager.completeTask`, `CompleteTasksLessThan`), which is *not* transactional with History — hence the History-side checks.
- **Invariant**: a Worker only ever receives a workflow task whose `Started` event is durably recorded. If History cannot record it, the Worker never sees it.
- **Tests**: `service/matching/matching_engine_test.go` (`TestPollWorkflowTaskQueues`, `TestAddWorkflowTasks`, `TestConcurrentPublishConsumeWorkflowTasks`, `TestPollWorkflowTaskQueues_InternalError`), `service/matching/matcher_test.go` (`TestLocalSyncMatch`, `TestRejectSyncMatchWhenBacklog`), `service/history/history_engine2_test.go` (`TestRecordWorkflowTaskStartedSuccess`, `TestRecordWorkflowTaskStartedIfTaskAlreadyStarted`, `TestRecordWorkflowTaskStartedIfTaskAlreadyCompleted`, `TestRecordWorkflowTaskStartedConflictOnUpdate`).

## Stage 8 — The Worker runs workflow code and returns commands

- **Where**: SDK (external; see `https://github.com/temporalio/sdk-core/blob/master/ARCHITECTURE.md` linked from `docs/architecture/README.md`). The SDK replays history events 1–3 through `SupportAgentWorkflow`, which calls `ExecuteActivity(CallTicketAPI)` and then blocks. The SDK translates that into commands `[ScheduleActivityTask{ActivityId, ActivityType, TaskQueue, timeouts, RetryPolicy}]` and calls `RespondWorkflowTaskCompleted{TaskToken, Commands, StickyAttributes?, ReturnNewWorkflowTask: true}`.
- **Server-side contract** (`docs/architecture/speculative-workflow-task.md`): every workflow task the Worker sees contains `WorkflowTaskScheduled` and `WorkflowTaskStarted` as the last two events. Transient tasks (attempt > 1) are synthesized, not persisted (`ms.IsTransientWorkflowTask()` checks attempts > 1).
- **Invariant**: determinism — the server assumes replaying the same history yields the same commands. Not verified server-side.
- **Test to read**: `tests/workflow_task_test.go` uses `taskpoller.TaskPoller.PollAndHandleWorkflowTask` (`common/testing/taskpoller/taskpoller.go:124`) to play the Worker role in-process; that is the cleanest view of "poll, run handler, respond".

## Stage 9 — Commands re-enter the server, change state, cause follow-on work

- **Path**: Frontend `WorkflowHandler.RespondWorkflowTaskCompleted` (`workflow_handler.go:1220`) deserializes the token, then `historyClient.RespondWorkflowTaskCompleted` (`:1251`) → `history.Handler.RespondWorkflowTaskCompleted` (`handler.go:558`, deserializes the token again to find namespace/workflow → shard) → `historyEngineImpl.RespondWorkflowTaskCompleted` (`history_engine.go:616`) → `respondworkflowtaskcompleted.WorkflowTaskCompletedHandler.Invoke` (`service/history/api/respondworkflowtaskcompleted/api.go:112`).
- **Observed inside `Invoke`**: `workflowConsistencyChecker.GetWorkflowLeaseWithConsistencyCheck` (lock + load mutable state, `service/history/api/consistency_checker.go:126`); `GetWorkflowTaskByID(token.ScheduledEventId)`; `NotFound("Workflow task not found.")` if the task is gone (already completed/timed out); build-ID checks; `ms.AddWorkflowTaskCompletedEvent(currentWorkflowTask, request, limits)` (event 4: `WorkflowTaskCompleted`); then `workflowTaskHandler.handleCommands(...)` (`workflow_task_completed_handler.go:172`) → `handleCommand` (`:279`) → for our command `handleCommandScheduleActivity` (`:470`): `validateCommandAttr` → `attrValidator.ValidateActivityScheduleAttributes` (`service/history/api/command_attr_validator.go`), then `mutableState.AddActivityTaskScheduledEvent` (`mutable_state_impl.go:4305`) which appends event 5 `ActivityTaskScheduled`, creates an `ActivityInfo` in `pendingActivityInfoIDs`, and calls `taskGenerator.GenerateActivityTasks` (`task_generator.go:552`) → a `tasks.ActivityTask` transfer task; activity timeout timers come from the `TimerSequence` via `GenerateActivityTimerTasks` at transaction close (`mutable_state_impl.go:9061`).
- If a command is invalid, `failWorkflowTask` (`api.go:1124`) records `WorkflowTaskFailed` with a cause and reschedules.
- Then: if the workflow has more to do (buffered events, `ReturnNewWorkflowTask`), `AddWorkflowTaskScheduledEvent` may be called immediately; otherwise no new workflow task now. Persist via `weContext.UpdateWorkflowExecutionAsActive(ctx, shardContext)` (`workflow/context.go:695`) → `updateWorkflowExecutionWithNew` (`:888`) → `updateWorkflowExecution` (`transaction_impl.go:514`) → `shard.ContextImpl.UpdateWorkflowExecution` (`shard/context_impl.go`) with `RangeID` and the mutation's `Condition`/`DBRecordVersion` (`persistence.WorkflowMutation` fields, `data_interfaces.go:349-377`).
- **Durable state change**: events 4–5 appended; mutable state has a pending activity (attempt 1, scheduled time, retry policy) and no started workflow task; new transfer task `ActivityTask{ScheduledEventID: 5}`; timer tasks for schedule-to-close / schedule-to-start / start-to-close / heartbeat as configured.
- **Invariant**: commands are applied exactly once against the mutable state version the Worker's token refers to. A second `RespondWorkflowTaskCompleted` with the same token (Worker retry after a network error) finds no pending workflow task with that scheduled event ID and gets `NotFound`; nothing is double-applied. **Concurrency**: the per-execution lock from the cache (`wcache.Cache`) plus persistence `DBRecordVersion` condition; a lost race surfaces as `WorkflowConditionFailedError`/`ConditionFailedError`, the cached mutable state is cleared (`ContextImpl.Clear`) and the RPC returns an error for the caller to retry.
- **Tests**: `service/history/api/respondworkflowtaskcompleted/api_test.go`, `workflow_task_completed_handler_test.go`, `service/history/api/command_attr_validator_test.go`, `service/history/historybuilder/history_builder_test.go` (`TestWorkflowTaskCompleted`), `tests/workflow_failures_test.go` (`TestRespondWorkflowTaskCompletedReturnsErrorIfInvalidArgument`).

## Stage 10 — The Activity is dispatched, then completes, fails, or times out

**Dispatch**: transfer executor `processActivityTask` (`transfer_queue_active_task_executor.go:234`): load mutable state; `GetActivityInfo(task.ScheduledEventID)` missing → `ErrStaleReference` (drop); `ai.Stamp != task.Stamp || ai.Paused` → `ErrStaleReference`; workflow not running → drop; release lock; `pushActivity` (`transfer_queue_task_executor_base.go:95`) → `matchingRawClient.AddActivityTask`. Matching: `AddActivityTask` (`matching_engine.go:646`) → same partition/sync-match/spool machinery. Worker polls via Frontend `PollActivityTaskQueue` (`workflow_handler.go:1343`) → `matchingEngineImpl.PollActivityTaskQueue` (`:972`) → `recordActivityTaskStarted` (`:3583`) → History `RecordActivityTaskStarted` (`handler.go:328`) → `recordactivitytaskstarted.Invoke` (`service/history/api/recordactivitytaskstarted/api.go:38`) inside `api.GetAndUpdateWorkflowWithNew` (`update_workflow_util.go:14`): `GetActivityInfo`; not found → `ErrActivityTaskNotFound`; same `RequestId` → return same response (dedupe); already started by another request → `TaskAlreadyStarted("Activity")`; stamp mismatch → NotFound; else `AddActivityTaskStartedEvent` (event 6) and persist.

**Completion**: Worker → Frontend `RespondActivityTaskCompleted` (`workflow_handler.go:1643`) → History `RespondActivityTaskCompleted` (`handler.go:405`) → `respondactivitytaskcompleted.Invoke` → `AddActivityTaskCompletedEvent` (event 7) → since the workflow is now unblocked, `AddWorkflowTaskScheduledEvent` (event 8) → transfer `WorkflowTask` → back to stage 6.

**Failure with retry**: Worker → `RespondActivityTaskFailed` (`workflow_handler.go:1848`) → History (`handler.go:456`) → `respondactivitytaskfailed.Invoke` (`api.go:23`, inside `GetAndUpdateWorkflowWithNew`): validates the activity is still the current attempt (else `ErrActivityTaskNotFound`), then `mutableState.RetryActivity(ai, failure)` (`mutable_state_impl.go:6879`). Observed: returns `RETRY_STATE_RETRY_POLICY_NOT_SET` / `CANCEL_REQUESTED` / `TIMEOUT` / `NON_RETRYABLE_FAILURE` / `IN_PROGRESS`; on `IN_PROGRESS` it computes `nextBackoffInterval`, increments `ai.Attempt`, optionally `ai.Stamp++` (dynamic config `EnableActivityRetryStampIncrement`), and `GenerateActivityRetryTasks` adds an `ActivityRetryTimerTask`. **No history event is written for a retried attempt**; only mutable state changes. If not `IN_PROGRESS`, `AddActivityTaskFailedEvent` is written and a workflow task is scheduled so the workflow code can react.

**Retry timer fires**: timer queue `timerQueueActiveTaskExecutor.executeActivityRetryTimerTask` (`timer_queue_active_task_executor.go:540`): checks `GetActivityInfo`, `task.Stamp != activityInfo.Stamp || Paused` → `ErrActivityTaskNotFound`, `task.Attempt < activityInfo.Attempt || StartedEventId != EmptyEventID` → not found (stale), releases lock, then **calls Matching `AddActivityTask` directly** (no transfer task). Doc confirms: "activity retries do not contribute events to Workflow History".

**Timeout**: `executeActivityTimeoutTask` (`:204`) → `processSingleActivityTimeoutTask` (`:288`) uses `TimerSequence` to decide which timeout fired, calls `RetryActivity` with a timeout failure; if still `IN_PROGRESS` a retry is scheduled; else `AddActivityTaskTimedOutEvent` and schedule a workflow task.

- **Invariants**: (a) only the current attempt (matched by `Attempt`, `Stamp`, `RequestId`) can start/complete/fail an activity; (b) retries are server-driven and invisible in history until final outcome; (c) at-least-once execution of activity code.
- **Tests**: `service/history/transfer_queue_active_task_executor_test.go` (`TestProcessActivityTask_Success`, `TestProcessActivityTask_Duplication`, `TestProcessActivityTask_Paused`), `service/history/timer_queue_active_task_executor_test.go` (`TestProcessActivityTimeout_RetryPolicy_Retry`, `TestProcessActivityTimeout_RetryPolicy_RetryTimeout`, `TestActivityRetryTimer_Fire`), `service/history/workflow/retry_test.go`, `tests/activity_test.go` (`TestActivityRetry`, `Test_ActivityTimeouts`, `TestActivityScheduleToClose_FiredDuringBackoff`), `service/history/history_engine2_test.go` (`TestRecordActivityTaskStartedSuccess`).

## Stage 11 — Recovery when the Worker crashes at a meaningful point

Case A — **Worker crashes after polling the workflow task but before responding** (stage 8):
- Nothing tells the server. The `WorkflowTaskTimeoutTask{START_TO_CLOSE}` persisted at stage 7 fires in the timer queue: `executeWorkflowTaskTimeoutTask` (`timer_queue_active_task_executor.go:382`). Observed: `GetWorkflowTaskByID(task.EventID)`; `task.Stamp != workflowTask.Stamp` → `ErrStaleReference` (the task had already completed and a newer one exists; drop); `workflowTask.Attempt != task.ScheduleAttempt` → stale; for `START_TO_CLOSE`: `AddWorkflowTaskTimedOutEvent` then schedule a new workflow task (`scheduleWorkflowTask`), persisted through `updateWorkflowExecution`. `workflowTaskStateMachine.failWorkflowTask` (`workflow_task_state_machine.go:1050`) clears the sticky queue if set (and does not bump the attempt in that case), otherwise increments the attempt; attempt > 1 makes the next task *transient* (`speculative-workflow-task.md`).
- The new transfer task reaches Matching (stage 6–7). Because the Worker's sticky queue has no poller, `AddWorkflowTask` returns `StickyWorkerUnavailable` and History re-pushes to the normal queue (stage 6 fallback). If a sticky task was already spooled, the `SCHEDULE_TO_START` timeout task generated in `GenerateScheduleWorkflowTaskTasks` (sticky only) fires and reschedules onto the normal queue (`executeWorkflowTaskTimeoutTask`, `SCHEDULE_TO_START` branch).
- If the *late* Worker comes back and responds with the old token, `RespondWorkflowTaskCompleted` finds no workflow task with that scheduled event ID → `NotFound`; nothing is applied twice.
- Tests: `service/history/timer_queue_active_task_executor_test.go` (`TestWorkflowTaskTimeout_Fire`, `TestWorkflowTaskTimeout_Noop`, `TestWorkflowTaskTimeout_StampMismatch`), `tests/transient_task_test.go` (`TestTransientWorkflowTaskTimeout`), `tests/stickytq_test.go` (`TestStickyTimeoutNonTransientWorkflowTask`, `TestStickyTaskqueueResetThenTimeout`).

Case B — **Worker crashes mid-activity** (after `ActivityTaskStarted`): the start-to-close (or heartbeat) timer fires → stage 10 timeout path → retry attempt 2 → new poll by any Worker. The old Worker's late `RespondActivityTaskCompleted` for attempt 1 is rejected because `Attempt`/`Stamp` no longer match (`ErrActivityTaskNotFound`).

Case C — **History host crashes**: another host's `ControllerImpl.acquireShards` (membership change) → `shard.ContextImpl.acquireShard` (`context_impl.go:2030`) → `renewRangeLocked` → `ShardManager.UpdateShard` with a new `RangeID`; the old host's in-flight writes fail with `ShardOwnershipLostError`. Queue processors restart from the last checkpointed ack level and **re-execute** tasks after it; the stale-task checks in stages 6, 10 and 11 make that safe. Timer tasks are re-read from persistence, so the workflow-task timeout still fires.

Case D — **Matching host crashes**: spooled tasks are re-read from the task-queue backlog by the new owner (`taskReader`); a task that was sync-matched but whose Worker never responded is covered by Case A; a task delivered twice (ack level not yet persisted) is rejected by History's `Record*TaskStarted` with `TaskAlreadyStarted`/`NotFound` (stage 7).

---

## Mermaid — happy path

```mermaid
sequenceDiagram
    autonumber
    participant C as SDK Client
    participant F as Frontend<br/>WorkflowHandler
    participant H as History shard<br/>(Handler → Engine → api/*)
    participant P as Persistence<br/>(executions, history nodes, tasks)
    participant Q as History transfer/timer<br/>queue processors
    participant M as Matching<br/>(engine, partition mgr, matcher)
    participant W as Worker (SDK)

    C->>F: StartWorkflowExecution
    F->>F: interceptors + prepareStartWorkflowRequest
    F->>H: historyClient.StartWorkflowExecution (shard = hash(ns_wfid))
    H->>H: Starter.Invoke: NewWorkflowWithSignal → events [1 Started, 2 WFTScheduled], transfer WorkflowTask
    H->>P: shard.CreateWorkflowExecution (RangeID fenced, CAS/tx)
    P-->>H: ok
    H-->>F: RunId
    F-->>C: RunId
    Q->>P: GetHistoryTasks (transfer)
    Q->>Q: processWorkflowTask: stamp/version checks, release lock
    Q->>M: AddWorkflowTask
    W->>F: PollWorkflowTaskQueue (long poll)
    F->>M: PollWorkflowTaskQueue
    M->>M: sync match or backlog
    M->>H: RecordWorkflowTaskStarted
    H->>P: append [3 WFTStarted] + WorkflowTaskTimeoutTask(START_TO_CLOSE)
    H-->>M: history + token
    M-->>F: task
    F-->>W: task
    W->>W: replay + run workflow code → [ScheduleActivityTask]
    W->>F: RespondWorkflowTaskCompleted(token, commands)
    F->>H: RespondWorkflowTaskCompleted
    H->>H: AddWorkflowTaskCompletedEvent, handleCommandScheduleActivity
    H->>P: append [4 WFTCompleted, 5 ActivityTaskScheduled] + ActivityTask + activity timers
    Q->>M: AddActivityTask
    W->>F: PollActivityTaskQueue
    F->>M: PollActivityTaskQueue
    M->>H: RecordActivityTaskStarted
    H->>P: append [6 ActivityTaskStarted]
    M-->>W: activity task
    W->>F: RespondActivityTaskCompleted
    F->>H: RespondActivityTaskCompleted
    H->>P: append [7 ActivityTaskCompleted, 8 WFTScheduled] + WorkflowTask
    Q->>M: AddWorkflowTask (loop continues; workflow waits for approval signal)
```

## Mermaid — Worker crash on a workflow task, then retry

```mermaid
sequenceDiagram
    autonumber
    participant W1 as Worker A (crashes)
    participant M as Matching
    participant H as History shard
    participant T as Timer queue
    participant W2 as Worker B

    M->>H: RecordWorkflowTaskStarted (attempt 1)
    H->>H: persist WFTStarted + WorkflowTaskTimeoutTask(START_TO_CLOSE)
    M-->>W1: workflow task (token: scheduledEventID=2, attempt=1)
    Note over W1: crashes before responding
    T->>H: executeWorkflowTaskTimeoutTask (stamp/attempt match → real)
    H->>H: AddWorkflowTaskTimedOutEvent; failWorkflowTask: clear sticky / attempt++
    H->>H: schedule new WFT (transient if attempt>1) → transfer task
    H->>M: AddWorkflowTask (sticky → StickyWorkerUnavailable → normal queue)
    W2->>M: PollWorkflowTaskQueue
    M->>H: RecordWorkflowTaskStarted (attempt 2)
    H-->>M: history + new token
    M-->>W2: workflow task
    W2->>H: RespondWorkflowTaskCompleted (new token) → applied
    W1-->>H: (late) RespondWorkflowTaskCompleted (old token)
    H-->>W1: NotFound "Workflow task not found." (ignored, nothing applied)
```

## Glossary (only what the trace needs)

- **Namespace**: tenant boundary; name resolved to ID by `namespace.Registry`; part of the shard hash key.
- **Workflow ID / Run ID**: user-chosen ID vs. server-minted UUID per run; "current run" is the run the ID currently points to.
- **Shard / RangeID**: partition of executions owned by one History host; `RangeID` is the monotonically increasing lease number every write is fenced by.
- **Mutable state**: per-run projection (`MutableStateImpl`): execution info/state, pending activities/timers/children, current workflow task, next event ID, `DBRecordVersion`.
- **History event / event ID**: append-only records; `NextEventID` is the next ID to assign; a batch is what one transaction appends.
- **Transfer task / timer task**: History-internal persisted tasks (outbox rows) executed by per-shard queue processors; not user-visible.
- **Workflow Task (WFT)**: unit delivered to a Worker to advance workflow code; identified by its `ScheduledEventID`; has `Attempt` and `Stamp`.
- **Activity task**: unit delivered to a Worker to run one activity attempt; identified by `ScheduledEventID`, `Attempt`, `Stamp`, `RequestId`.
- **Stamp**: a counter in mutable state that is bumped when a task/activity is invalidated or reset, so old outbox rows can be recognized as stale.
- **Task queue / partition / sticky queue**: Matching-side named queue → N partitions; a sticky queue is a Worker-private queue used to skip full replay; falls back to the normal queue on timeout.
- **Sync match / spool**: hand a task straight to a waiting poller vs. persist it in the backlog.
- **Task token**: opaque bytes (`tokenspb.Task`) the Worker returns; identifies namespace, workflow, run, scheduled event ID, attempt.
- **Transient WFT**: attempt > 1; scheduled/started events are synthesized into the poll response, not persisted, until it completes.
- **Ack level / checkpoint**: how far a queue processor has durably confirmed; tasks after it may be re-run after restart.
- **Condition failed / ownership lost**: the two persistence error families that make optimistic concurrency and fencing visible.
