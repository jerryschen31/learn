# 05 — Part E: A 12-session code-reading apprenticeship

Each session is 60–120 minutes. Read in the order given. "Run" means the narrowest `go test` invocation; all listed test names exist in this checkout. Run tests from the repo root with Go 1.27.1. Some client/driver tests are `//go:build linux`; on macOS read them instead of running them (noted per session).

A shared convention for exercises: keep notes in `jerry-learn/notes/session-NN.md` (create the folder yourself; nothing in the repo depends on it).

---

## Session 1 — Repository map, contribution docs, local dev topology

- **Question answered:** Where does each concern live, and what does one process look like in `-dev` mode?
- **Prerequisites:** Go toolchain installed; `make bootstrap` run once.
- **Read in order:**
  1. `contributing/README.md`
  2. `jerry-learn/00-checkout-report.md` (this tutorial's verified map)
  3. `contributing/architecture-eval-lifecycle.md`
  4. `GNUmakefile` targets `check`, `test`, `test-nomad`, `dev`
  5. `command/agent/config.go` `DevConfig` and `DefaultConfig`
  6. `dev/cluster/server1.hcl`, `dev/cluster/client1.hcl`, `dev/cluster/cluster.sh`
- **Symbols:** `DevConfig`, `DefaultConfig`, `Config.Merge`; the package list in `contributing/README.md`.
- **Trace manually:** the two one-liners in `contributing/README.md` (`Client -> HTTP API -> RPC -> StateStore` and `... -> Raft -> FSM -> StateStore`); map each arrow to a directory.
- **Exercise:** build with `make dev`, run `bin/nomad agent -dev` (on Linux use `sudo` per the README; on macOS `raw_exec` and `mock_driver` still work without sudo for simple experiments), run `bin/nomad job init -short example.nomad.hcl` then `bin/nomad job run example.nomad.hcl`, and capture `nomad job status`, `nomad eval status <id>`, `nomad alloc status <id>` outputs into your notes. Note which fields are *desired* and which are *observed*.
- **Explain it back:** in ≤150 words, what happens between the CLI returning and the allocation showing `running`.
- **Socratic:** (1) Why is `command/` forbidden from importing `nomad/structs`? (2) What would you lose if `-dev` mode were the only topology you ever ran? (3) Which of the five `GOTEST_GROUP` groups would a scheduler change touch? (4) Where would a new RPC's `MessageType` constant go, and why must old values never change?
- **Stop here:** you do not need to understand Raft internals, the UI, or any driver besides `raw_exec`/`mock_driver`.
- **Agent-infra connection:** your harness today is one process; this session shows how Nomad separates "API in-process" from "control plane elsewhere" with the same binary. Limit: Nomad's dev mode collapses that separation, so it teaches shape, not failure behavior.
- **Contribution-relevant area:** `contributing/` docs (stale Go version table in `contributing/golang.md` is an example of a doc-only fix).

## Session 2 — Agent startup: server vs client distinction

- **Question:** How does one binary become a server, a client, or both, and what long-lived goroutines does each start?
- **Prerequisites:** Session 1.
- **Read:**
  1. `command/agent/command.go` `Run`, `setupAgent`, `setupTelemetry`, `handleSignals`
  2. `command/agent/agent.go` `NewAgent`, `setupServer`, `setupClient`, `serverConfig`, `clientConfig`, `RPC`
  3. `nomad/server.go` `NewServer` (skim), `setupRaft`, `setupRPC`, `setupWorkers`
  4. `nomad/leader.go` `monitorLeadership`, `establishLeadership` (list what it enables)
  5. `client/client.go` `NewClient`, `init`, and the goroutines launched at the end of `NewClient`
  6. `command/agent/http.go` `NewHTTPServers`, `registerHandlers`, `wrap`
- **Symbols/tests:** `agent.NewTestAgent` (`command/agent/testagent.go`), `nomad.TestServer` (`nomad/testing.go`), `client.TestClient` (`client/testing.go`).
- **Trace:** `(*Agent).RPC("Job.Register", ...)` when the agent is (a) server, (b) client-only.
- **Exercise:** run `go test ./command/agent -run 'TestAgent_ServerConfig' -count=1 -v` (verify the exact name with `grep -n 'func TestAgent_ServerConfig' command/agent/agent_test.go` first; if it differs, pick the nearest `TestAgent_*Config*` test) and read the assertions.
- **Explain it back:** list the leader-only goroutines from `establishLeadership` in two columns: "rebuilt from durable state" vs "purely in-memory."
- **Socratic:** (1) Why do scheduler workers run on followers too? (2) What does `revokeLeadership` have to stop, and what happens to a worker holding a dequeued eval? (3) Why does the client's `registerNode` use an unauthenticated RPC? (4) What is `NumSchedulers` defaulted to in `nomad/config.go`, and where might the agent layer override it?
- **Stop here:** Serf gossip, TLS wiring, and keyring initialization can remain black boxes.
- **Agent-infra connection:** the same "one binary, role by config" pattern is a cheap way to get a control plane and workers from one codebase. Limit: Nomad's roles are static per process; agent frameworks often want dynamic role changes.
- **Contribution area:** agent config validation and error messages.

## Session 3 — Job registration and the authoritative state write

- **Question:** What exactly is durable when `nomad job run` returns?
- **Prerequisites:** Sessions 1–2.
- **Read:**
  1. `command/agent/job_endpoint.go` `jobUpdate`, `ApiJobToStructJob` (skim)
  2. `nomad/job_endpoint.go` `Register` and `doRegister` (full)
  3. `nomad/job_endpoint_hooks.go` `admissionControllers`, `admissionMutators`, `admissionValidators`, `jobValidate.Validate`
  4. `nomad/rpc.go` `rpcHandler.forward`, `(*Server).raftApply`, `raftApplyFuture`
  5. `nomad/fsm.go` `Apply` (switch), `applyUpsertJob`
  6. `nomad/state/state_store.go` `UpsertJob`, `upsertJobImpl`, `UpsertEvals`
  7. `contributing/architecture-state-store.md`
- **Symbols/tests:** `structs.JobRegisterRequest{Job, Eval, Deployment, EnforceIndex, JobModifyIndex, IdempotencyToken}`; `TestJobEndpoint_Register`, `TestJobEndpoint_Register_NonOverlapping`, `TestFSM_RegisterJob`, `TestStateStore_UpsertJob_Job`.
- **Trace:** the `JobRegisterRequestType` entry from `raftApply` to the memdb transaction commit; note every table touched in `upsertJobImpl`.
- **Exercise:** run `go test ./nomad -run 'TestJobEndpoint_Register$' -count=1 -v` and annotate each assertion with the state-store call that makes it true.
- **Explain it back:** why the eval ID and `SubmitTime` are generated in the handler, not the FSM.
- **Socratic:** (1) What does `EnforceIndex` protect against, and is it enforced before or after Raft? (2) Why is the eval written in the *same* Raft entry as the job? (3) What happens on re-registering an identical spec? (4) What does `IdempotencyToken` add beyond upsert semantics?
- **Stop here:** Sentinel, multiregion, Consul config entries, scaling policies.
- **Agent-infra connection:** "write intent + trigger atomically" is the pattern for an agent job queue where the request and its first scheduling signal must not be split by a crash. Limit: Nomad needs Raft for this; a single-node harness can use one DB transaction.
- **Contribution area:** validation error messages in `Job.Validate` and admission hooks; tests in `nomad/job_endpoint_test.go`.

## Session 4 — Evaluation broker and scheduler entry

- **Question:** How does a durable evaluation become work on exactly one worker, and what happens when that worker fails?
- **Prerequisites:** Session 3.
- **Read:**
  1. `contributing/architecture-eval-states.md`
  2. `nomad/structs/eval.go` (statuses, triggers, `ShouldEnqueue`, `ShouldBlock`, `MakePlan`)
  3. `nomad/eval_broker.go` `Enqueue`, `processEnqueue`, `Dequeue`, `Ack`, `Nack`, `nackReenqueueDelay`
  4. `nomad/eval_endpoint.go` `Dequeue`, `getWaitIndex`, `Ack`, `Nack`
  5. `nomad/worker.go` `run`, `dequeueEvaluation`, `snapshotMinIndex`, `invokeScheduler`, `sendAck`/`sendNack`
  6. `nomad/leader.go` `restoreEvals`, `reapFailedEvaluations`, `createFailedFollowup`
  7. `nomad/blocked_evals.go` `Block`, `Unblock`, `UnblockNode` (skim)
- **Symbols/tests:** `EvalNackTimeout`, `EvalDeliveryLimit` (`nomad/config.go`); `TestEvalBroker_Enqueue_Dequeue_Nack_Ack`, `TestEvalBroker_Serialize_DuplicateJobID`, `TestEvalBroker_AckAtDeliveryLimit`, `TestWorker_waitForIndex`, `TestLeader_ReapFailedEval`.
- **Trace:** an eval nacked three times: broker queues → failed queue → `reapFailedEvaluations` → new eval with `WaitUntil`.
- **Exercise:** run `go test ./nomad -run 'TestEvalBroker_Serialize_DuplicateJobID|TestEvalBroker_AckAtDeliveryLimit' -count=1 -v`; write down the queue names an eval passes through in each.
- **Explain it back:** why the broker serializes per job even though the plan applier already validates capacity.
- **Socratic:** (1) What does `getWaitIndex` compute and why? (2) What is lost when the leader changes: queue order, in-flight tokens, or the evals themselves? (3) Why does `Dequeue` reject mismatched `SchedulerVersion`? (4) When is an eval `canceled` rather than `complete`?
- **Stop here:** the delay heap implementation details, quota-based blocking.
- **Agent-infra connection:** this is a lease-based work queue with per-key serialization and bounded redelivery; you will need the same three properties for tool-run dispatch. Limit: Nomad's payload is "reconsider this job," not "do this task"; do not copy it as a task queue.
- **Contribution area:** broker metrics and stats (`nomad/blocked_evals_stats.go`), eval GC thresholds.

## Session 5 — A focused scheduler placement trace

- **Question:** Given an eval and a snapshot, how does `GenericScheduler` decide *how many* and *where*?
- **Prerequisites:** Session 4.
- **Read:**
  1. `scheduler/README.md`
  2. `scheduler/structs/interfaces.go` (`Scheduler`, `State`, `Planner`)
  3. `scheduler/generic_sched.go` `Process`, `process`, `computeJobAllocs`, `computePlacements`, `selectNextOption`
  4. `scheduler/reconciler/reconcile_cluster.go` `Compute`, `computeGroup`, `placeAllocs`, `computeStop` (skim the rest)
  5. `scheduler/feasible/stack.go` `NewGenericStack`, `Select`
  6. `scheduler/feasible/feasible.go` `ConstraintChecker`, `DriverChecker`; `scheduler/feasible/rank.go` `BinPackIterator`, `NodeAffinityIterator`; `scheduler/feasible/select.go`
  7. `scheduler/util.go` `retryMax`, `progressMade`, `setStatus`, `taintedNodes`
- **Symbols/tests:** `EvalContext.Metrics()`, `failedTGAllocs`, `maxServiceScheduleAttempts`; `TestServiceSched_JobRegister`, `TestServiceSched_JobRegister_CreateBlockedEval`, `TestServiceStack_Select_ConstraintFilter`, `TestBinPackIterator_NoExistingAlloc`.
- **Trace:** `count = 3`, two ready nodes, one constraint that excludes a third node; follow `Select` for the first placement and record which iterators run.
- **Exercise:** run `go test ./scheduler -run 'TestServiceSched_JobRegister$' -count=1 -v`; then temporarily change the mock node count in your local copy of the test (not committed) to 1 and observe which assertions fail and why. Revert.
- **Explain it back:** the difference between "reconcile decides count" and "stack decides node."
- **Socratic:** (1) Why does `process` return `(false, nil)` on `RefreshIndex` rather than an error? (2) Where does `LimitIterator` stop scanning and what does spread change? (3) What ends up in `Allocation.Metrics`? (4) How does a *blocked* eval get reused vs newly created?
- **Stop here:** preemption, spread scoring math, system scheduler, NUMA.
- **Agent-infra connection:** the iterator pipeline is a clean model for "filter workers by capability, then score by load"; the `EvalContext` cache by computed class is the trick that makes it scale. Limit: Nomad scores on static resources; agent workers may need dynamic signals (queue depth, model warm cache) that Nomad does not model.
- **Contribution area:** placement failure messages and metrics (`failedTGAllocs`), scheduler tests using `nomad/mock`.

## Session 6 — Plan application and allocation creation

- **Question:** How does a proposal become durable allocations without two schedulers overcommitting a node?
- **Prerequisites:** Session 5.
- **Read:**
  1. `nomad/structs/plan.go` (`Plan`, `PlanResult`, `FullCommit`, `IsNoOp`)
  2. `nomad/worker.go` `SubmitPlan`
  3. `nomad/plan_endpoint.go` `Submit`
  4. `nomad/plan_queue.go` (`Enqueue`, `Dequeue`, priority ordering)
  5. `nomad/plan_apply.go` `planApply`, `snapshotMinIndex`, `evaluatePlan`, `evaluatePlanPlacements`, `evaluateNodePlan`, `applyPlan`, `asyncPlanWait`
  6. `nomad/structs/funcs.go` `AllocsFit`, `RemoveAllocs`, `AllocSubset`
  7. `nomad/state/state_store.go` `UpsertPlanResults`, `upsertAllocsImpl`
- **Symbols/tests:** `Plan.SnapshotIndex`, `PlanResult.RefreshIndex`, `RejectedNodes`, `IneligibleNodes`, `PlanApplyPipeline`; `TestPlanApply_EvalPlan_Partial`, `TestPlanApply_EvalNodePlan_NodeFull`, `TestPlanEndpoint_ApplyConcurrent`, `TestStateStore_UpsertPlanResults_AllocationsCreated_Denormalized`.
- **Trace:** two plans for two jobs targeting the same nearly-full node; follow `planApply` twice and identify where the second is rejected and what the worker does next.
- **Exercise:** run `go test ./nomad -run 'TestPlanApply_EvalNodePlan_NodeFull|TestPlanEndpoint_ApplyConcurrent' -count=1 -v`; write the invariant `AllocsFit` enforces in one sentence with the exact inputs.
- **Explain it back:** why the applier keeps an optimistic snapshot while plans are in flight.
- **Socratic:** (1) Why are stops always valid on a down node but placements never? (2) What does `NormalizeAllocations` remove and why is it safe? (3) When are nodes marked ineligible by the applier? (4) Which timestamp is set in `applyPlan` and why not in the FSM?
- **Stop here:** preemption accounting, identity signing details, deployment canary correction.
- **Agent-infra connection:** a single serialization point validating optimistic proposals against durable reservations is the minimal design for a fleet allocator. Limit: it only reasons about declared resources.
- **Contribution area:** plan rejection diagnostics (`RejectedNodes` reasons), `plan_apply_test.go` coverage.

## Session 7 — Client reconciliation and the allocation runner

- **Question:** How does a client learn about, persist, and start an allocation, and how does it survive its own restart?
- **Prerequisites:** Session 6.
- **Read:**
  1. `client/client.go` `watchAllocations`, `runAllocs`, `addAlloc`, `updateAlloc`, `removeAlloc`, `allocSync`, `AllocStateUpdated`, `restoreState`, `saveState`
  2. `nomad/node_endpoint.go` `GetClientAllocs`, `UpdateAlloc`, `batchUpdate`
  3. `nomad/alloc_endpoint.go` `GetAllocs`
  4. `client/state/interface.go` (`StateDB`), `client/state/db_bolt.go` (skim)
  5. `client/allocrunner/alloc_runner.go` `NewAllocRunner`, `Run`, `runTasks`, `handleTaskStateUpdates`, `clientAlloc`, `Restore`, `Update`, `handleAllocUpdates`
  6. `client/allocrunner/alloc_runner_hooks.go` `initRunnerHooks`, `prerun`, `postrun`, `allocHealthSetter`
- **Symbols/tests:** `allocSyncIntv`, `batchUpdateInterval`; `TestClient_WatchAllocs`, `TestClient_SaveRestoreState`, `TestClient_UpdateAllocStatus`, `TestAllocRunner_AllocState_Initialized`, `TestAllocRunner_Update_Semantics`, `TestClientEndpoint_GetClientAllocs_Blocking`.
- **Trace:** a new allocation from `GetClientAllocs` response to `go ar.Run()`, listing every BoltDB write.
- **Exercise:** run `go test ./client -run 'TestClient_SaveRestoreState' -count=1 -v` (Linux preferred; on macOS check the build tags at the top of `client/client_test.go` and pick a portable test such as `TestClient_WatchAllocs` if needed). Then, with a local `-dev` agent, run a `raw_exec` job, `kill -9` the agent, restart it, and confirm from `nomad alloc status` and the client log that the task was recovered rather than restarted.
- **Explain it back:** the order of "restore local" vs "pull from server" and why it matters.
- **Socratic:** (1) Why is `PutAllocation` called before `NewAllocRunner`? (2) What does the client do with allocs the server no longer lists? (3) Why must `UpdateAlloc` be rejected while the node is `down`? (4) Which alloc field does the client compute rather than copy from the server?
- **Stop here:** networking hooks, CSI hooks, identity hook internals.
- **Agent-infra connection:** "persist assignment, then start; report status in batches; re-attach on restart" is directly applicable to a worker daemon running tool sandboxes. Limit: re-attachment depends on the runtime exposing a recoverable handle.
- **Contribution area:** client restore edge cases and log messages; `client/state` upgrade tests.

## Session 8 — Task runner and one deliberately chosen task driver (`raw_exec`, with `mock_driver` for tests)

- **Question:** How does a task become a process, how is its exit interpreted, and where does the restart policy live?
- **Prerequisites:** Session 7.
- **Read:**
  1. `client/allocrunner/taskrunner/task_runner.go` `Run` (the `MAIN:` loop), `runDriver`, `initDriver`, `handleTaskExitResult`, `shouldRestart`, `handleKill`, `UpdateState`, `persistLocalState`, `Restore`, `restoreHandle`
  2. `client/allocrunner/taskrunner/task_runner_hooks.go` `prestart`, `poststart`, `exited`, `stop`
  3. `client/allocrunner/taskrunner/restarts/restarts.go`
  4. `plugins/drivers/driver.go` `DriverPlugin`, `TaskHandle`, `ExitResult`
  5. `drivers/rawexec/driver.go` `StartTask`, `WaitTask`, `StopTask`, `RecoverTask`, `Fingerprint`
  6. `drivers/mock/driver.go` `StartTask`, `WaitTask` and `contributing/mock-driver.md`
  7. `client/pluginmanager/drivermanager/manager.go` `Dispense` (skim)
- **Symbols/tests:** `RestartTracker.GetState`, `structs.RestartPolicy{Attempts, Interval, Delay, Mode}`; `TestClient_RestartTracker_ModeDelay`, `TestClient_RestartTracker_ModeFail`, `TestTaskRunner_Stop_ExitCode`, `TestTaskRunner_Restore_Running`, `TestTaskRunner_Run_RecoverableStartError` (all in `task_runner_linux_test.go`, `//go:build linux`).
- **Trace:** a task exits with code 1 twice under `attempts = 1`; follow `handleTaskExitResult` → `RestartTracker` → `shouldRestart` → `TaskStateDead` and the events emitted.
- **Exercise:** run `go test ./client/allocrunner/taskrunner/restarts -count=1 -v` (portable). Then write a `mock_driver` job (`exit_code = 7`, `run_for = "2s"`, `restart { attempts = 1 }`) against a dev agent built without the `release` tag and watch `nomad alloc status` events.
- **Explain it back:** which layer decides restart vs reschedule, with the field names.
- **Socratic:** (1) What makes a start error "recoverable"? (2) Why persist the handle before reporting `running`? (3) What does `KillTimeout` bound? (4) How does the driver manager learn that a driver is healthy?
- **Stop here:** docker, exec (cgroups/namespaces), executor internals, logmon.
- **Agent-infra connection:** the driver interface is a good template for a "sandbox runtime" abstraction (start/wait/stop/recover/stats). Limit: exit code and signals are the only completion vocabulary.
- **Contribution area:** `drivers/rawexec` and `drivers/mock` tests; task event messages.

## Session 9 — Node/client failure and rescheduling

- **Question:** From a missed heartbeat to a replacement allocation, what is decided where, and how are duplicates bounded?
- **Prerequisites:** Sessions 4–8.
- **Read:**
  1. `nomad/heartbeat.go` (all)
  2. `nomad/node_endpoint.go` `UpdateStatus` (with the ASCII transition table), `nodeStatusTransitionRequiresEval`, `createNodeEvals`, `UpdateAlloc` reconnect eval
  3. `nomad/structs/alloc.go` `ShouldReschedule`, `RescheduleEligible`, `NextRescheduleTime`, `DisconnectTimeout`, `Expired`, `NeedsToReconnect`; `nomad/structs/group.go` `DisconnectStrategy`
  4. `scheduler/util.go` `taintedNodes`
  5. `scheduler/reconciler/filters.go` `classifyAllocs`, `filterByRescheduleable`, `updateByReschedulable`, `delayByLostAfter`
  6. `scheduler/reconciler/reconcile_cluster.go` `computeStop`, `computeDisconnecting`, `computeReconnecting`, `reconcileReconnecting`, `createRescheduleLaterEvals`, `createTimeoutLaterEvals`
  7. `nomad/plan_apply.go` `isValidForDownNode`, `isValidForDisconnectedNode`
- **Symbols/tests:** `NodeStatusDisconnected`, `AllocClientStatusLost/Unknown`, `EvalTriggerNodeUpdate/MaxDisconnectTimeout/Reconnect`, `FailoverHeartbeatTTL`; `TestHeartbeat_InvalidateHeartbeat`, `TestHeartbeat_InvalidateHeartbeat_DisconnectedClient`, `TestClientEndpoint_UpdateStatus_GetEvals`, `TestServiceSched_NodeDown`, `TestReconciler_LostNode`, `TestReconciler_Disconnected_Client`, `TestClient_ReconnectAllocs`.
- **Trace:** node with one service alloc (`lost_after = 5m`) misses heartbeats; follow to `unknown`, replacement, then reconnect at minute 2 with `reconcile = keep_original`.
- **Exercise:** run `go test ./scheduler -run 'TestServiceSched_NodeDown|TestServiceSched_Client_Disconnect_Creates_Updates_and_Evals' -count=1 -v` and `go test ./nomad -run 'TestHeartbeat_InvalidateHeartbeat' -count=1 -v`. In a dev cluster (`dev/cluster` configs, or two agents on one host), stop a client with `SIGSTOP`, observe `nomad node status` and `nomad job status`, then `SIGCONT` it.
- **Explain it back:** who marks an alloc `lost`, and why it is not the client.
- **Socratic:** (1) Why is `invalidateHeartbeat` guarded by `IsLeader`? (2) What is the difference between `down` and `disconnected` for plan validity? (3) When can two copies of one instance legitimately run? (4) What bounds the number of replacements?
- **Stop here:** node drain (`contributing/architecture-drainer.md`), node pools, multi-region.
- **Agent-infra connection:** this is the canonical "worker went silent" playbook: TTL → mark → bounded replacement → reconcile on return. Limit: application idempotency is out of scope for Nomad.
- **Contribution area:** disconnect/reconnect tests in `scheduler/reconciler`, node event messages.

## Session 10 — Health, deployments, updates, canaries, rollback

- **Question:** How does Nomad decide a new version is good, and what happens when it is not?
- **Prerequisites:** Sessions 5–9.
- **Read:**
  1. `nomad/structs/structs.go` `UpdateStrategy`; `nomad/structs/deployment.go`
  2. `scheduler/reconciler/reconcile_cluster.go` `createDeployment`, `computeCanaries`, `computeDestructiveUpdates`, `computeUnderProvisionedBy`, `isDeploymentComplete`
  3. `nomad/deploymentwatcher/deployments_watcher.go` `SetEnabled`, `watchDeployments`, `add`
  4. `nomad/deploymentwatcher/deployment_watcher.go` `watch`, `handleAllocUpdate`, `shouldFail`, `createBatchedUpdate`, `PromoteDeployment`, `FailDeployment`, `latestStableJob`
  5. `client/allocrunner/health_hook.go`; `client/allochealth/tracker.go` `Start`, `watchTaskEvents`, `watchNomadEvents`, `setTaskHealth`
  6. `nomad/state/state_store.go` `updateDeploymentWithAlloc`, `upsertDeploymentUpdates`
- **Symbols/tests:** `MaxParallel`, `Canary`, `AutoPromote`, `AutoRevert`, `MinHealthyTime`, `HealthyDeadline`, `ProgressDeadline`, `AllocDeploymentStatus.Healthy`; `TestWatcher_SetAllocHealth_Unhealthy_Rollback`, `TestWatcher_AutoPromoteDeployment`, `TestReconciler_NewCanaries`, `TestHealthHook_SetHealth_healthy`, `TestTracker_NomadChecks_Healthy`, `TestServiceSched_JobModify_Canaries`.
- **Trace:** `count = 3, max_parallel = 1, canary = 1`: enumerate the evals and deployment state after each alloc becomes healthy (compare to `contributing/architecture-eval-triggers.md`'s worked example).
- **Exercise:** run `go test ./nomad/deploymentwatcher -run 'TestWatcher_SetAllocHealth_Unhealthy_Rollback' -count=1 -v`; with a dev agent, deploy a `raw_exec` job with `update { max_parallel = 1, min_healthy_time = "5s", healthy_deadline = "30s", auto_revert = true }`, then push a version whose command exits immediately, and watch `nomad deployment status` and `nomad job history`.
- **Explain it back:** where each of `PlacedAllocs`, `HealthyAllocs`, `Promoted` is written, and by whom.
- **Socratic:** (1) Why does the deployment watcher create evals instead of allocs? (2) What is a "stable" job version? (3) What does `ProgressDeadline` measure that `HealthyDeadline` does not? (4) Why is health set only once per alloc?
- **Stop here:** Consul Connect, service mesh, multiregion deployments (`multiregion_ce.go`).
- **Agent-infra connection:** a staged rollout with health gating and auto-revert is what you want when shipping a new tool-worker image. Limit: Nomad's health signal must be expressible as task state or an HTTP/TCP/script check.
- **Contribution area:** deployment status descriptions, watcher tests.

## Session 11 — A focused test / issue-reproduction exercise

- **Question:** Can you take a suspected behavior, write a narrowly scoped failing test using the repo's own helpers, and explain the result?
- **Prerequisites:** Sessions 3–10.
- **Read:**
  1. `contributing/testing.md`
  2. `nomad/mock/job.go`, `nomad/mock/alloc.go`, `nomad/mock/node.go` (builder functions)
  3. `nomad/testing.go` `TestServer`; `testutil/wait.go` `WaitForResult`, `WaitForLeader`
  4. `scheduler/tests/testing.go` (harness used by scheduler tests) and one full test: `scheduler/generic_sched_test.go` `TestServiceSched_Reschedule_Later`
  5. `nomad/node_endpoint_test.go` `TestClientEndpoint_UpdateAlloc_Evals_ByTrigger`
  6. `.github/ISSUE_TEMPLATE/bug_report.md`
- **Symbols/tests:** `ci.Parallel`, `must.*`, `testlog.HCLogger`, `mock.Job()`, `mock.Alloc()`, `state.TestStateStore` (in `nomad/state/testing.go`).
- **Trace:** in `TestServiceSched_Reschedule_Later`, map each setup line to the state it creates and each assertion to the reconciler function that produces it.
- **Exercise:** in a scratch copy of `scheduler/generic_sched_test.go` (do not commit), write a new test `TestServiceSched_Reschedule_Later_JerryScratch` that sets `ReschedulePolicy{Attempts: 1, Interval: time.Hour, Delay: 30 * time.Second, DelayFunction: "constant"}`, fails one alloc, and asserts a follow-up eval with a non-zero `WaitUntil` exists. Run only that test. Delete it afterwards. Then draft (in your notes, not on GitHub) a bug-report-shaped write-up of *any* surprising behavior you saw in Sessions 7–10, using the template fields.
- **Explain it back:** the difference between reproducing with a unit test at the scheduler layer vs. an end-to-end run.
- **Socratic:** (1) Which test helper would you use to assert eventual state on a real `TestServer`? (2) Why do scheduler tests use a harness instead of a server? (3) What would make your scratch test flaky, per `contributing/testing.md`? (4) Which build tags could hide the code path you think you are testing?
- **Stop here:** E2E framework, Enos.
- **Agent-infra connection:** the harness pattern (fake `State`, fake `Planner`) is how you will test your own scheduler logic without infrastructure. Limit: harness tests do not exercise Raft or RPC timing.
- **Contribution area:** test improvements are the most welcome newcomer PRs (see `06-contribution-path.md`).

## Session 12 — A small, contribution-shaped capstone

- **Question:** Can you produce a change that a maintainer could review in ten minutes, with a test, following the repo's process?
- **Prerequisites:** all prior sessions.
- **Read:**
  1. `contributing/README.md` ("Contributing to Nomad" section: open an issue first)
  2. `.github/pull_request_template.md`
  3. `contributing/ai.md`
  4. `GNUmakefile` `check` (what CI will run), `cl` (changelog)
  5. `contributing/checklist-rpc-endpoint.md` (to understand why you will *not* touch RPCs in a first PR)
  6. The file you chose for the capstone (see below)
- **Symbols/tests:** whatever your target touches; keep it to one package.
- **Trace:** the full path from your change to the assertion that proves it.
- **Exercise (pick one, keep it local until you have maintainer feedback):**
  - Improve a log message or error string on the core path that lacked context you needed (e.g. a `logger.Debug` in `client/client.go` `watchAllocations` or `nomad/worker.go`), plus a test if one covers it.
  - Add a missing table-driven case to an existing test (`nomad/structs/alloc_test.go` around `NextRescheduleTime`, or `scheduler/feasible/feasible_test.go` for a constraint operator).
  - Fix a stale statement in `contributing/` (e.g. the Go version table in `contributing/golang.md`, or the `nomad/msgtypes.go` reference in `contributing/architecture-state-store.md` that does not match `helper/raftutil/msgtypes.go` in this checkout).
  Run `make check` (or at least `golangci-lint run ./<pkg>/...` and `go test ./<pkg> -run <Name>`), write a `make cl` entry only if user-facing.
- **Explain it back:** a PR description in the template's sections, including honest "Testing & Reproduction steps."
- **Socratic:** (1) Why does the project ask for an issue before a feature PR? (2) What in `make check` would fail for a change to `api/` that imports `nomad/structs`? (3) When is a changelog entry required? (4) How will you satisfy `contributing/ai.md`'s "deep understanding" requirement for this change?
- **Stop here:** anything touching Raft, plan apply, or ACLs; multi-package refactors.
- **Agent-infra connection:** the discipline here (issue → small PR → test → checklist) is the same you will want for your harness's contributors. Limit: none; this is process, not architecture.
- **Contribution area:** docs in `contributing/`, tests, log/diagnostic improvements.

---

## Deferred until after the core path (by design)

Every built-in task driver (`docker`, `exec`, `java`, `qemu`); Consul Connect / service mesh (`client/allocrunner/consul_*`, `envoy_*` hooks, `command/agent/consul/connect.go`); `ui/`; federation and multi-region (`nomad/leader.go` replication goroutines, `multiregion_ce.go`); plugin internals (`plugins/base`, executor gRPC); advanced ACL (roles, auth methods, binding rules, SSO); CSI and device drivers (`plugins/csi`, `plugins/device`, `client/allocrunner/csi_hook.go`); OS-specific paths (`*_linux.go`, `*_windows.go`, cgroups, landlock); performance tuning (`PlanApplyPipeline`, worker counts, `scheduler/benchmarks`); legacy/compat (`jobspec` HCL1, `CHANGELOG-unsupported.md`); enterprise-only features (Sentinel, quotas, multiregion, licensing — only `*_ce.go` stubs exist here).
