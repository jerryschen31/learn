# 01 — Part A: A first-principles mental model

Read this before opening any Go file. Every claim tagged **[source]** names the file where you can confirm it.

## A.1 Why SSH, cron, or a queue consumer is not enough

Imagine the running example: a fleet of agent-tool workers. A request arrives: "run this tool job; it needs 2 CPUs, 4 GB, and a GPU."

**Option 1: SSH into a box and start the process.**
The problem is not starting the process. The problems are everything after:
- Which box? You need an inventory of machines, what they currently have free, and whether they are alive. That inventory changes every second.
- Two operators SSH into the same box and both start a 4 GB job on a machine with 6 GB free. Nobody owned the "free memory" number, so it was double-spent. This is the **resource accounting** problem.
- The box dies. Who notices? Who restarts the job somewhere else? Who decides *not* to restart it a second time if the first replacement is already running? This is the **liveness detection + duplicate work** problem.
- The operator's laptop loses connectivity mid-command. Did the process start or not? This is the **at-least-once vs. at-most-once** problem.

**Option 2: cron on each machine.**
Cron has no cluster view. It cannot move work when a machine fails, cannot avoid overcommit, and expresses time, not intent ("I want 5 copies of this running").

**Option 3: a queue with consumers.**
A queue solves distribution of *messages* but not *placement of long-running processes with resource needs*. A consumer that pulls a job cannot know whether it has the GPU the job needs unless someone models capabilities; if the consumer crashes after pulling, the job is either lost (ack-on-pull) or duplicated (ack-on-finish with a slow consumer). Queues also have no notion of "keep 5 copies of this alive forever."

What is missing in all three is a **single, durable, authoritative statement of desired state**, a **durable record of each decision** ("this copy runs on that machine with these resources"), and a **loop that continuously compares desired against observed and repairs the difference**. That is what an orchestrator is. Nomad's specific answer is: desired state = `Job`; the decision record = `Allocation`; the trigger to re-compare = `Evaluation`; the proposed change = `Plan`; the loop = scheduler workers + plan applier on servers, plus runners on clients.

## A.2 Glossary with precise definitions

For each term: the definition, where it lives in this checkout, and a common wrong mental model.

| Term | Precise definition | Where [source] | Wrong mental model |
|---|---|---|---|
| **Job** | The user's declaration of desired state: a name, a type (`service`, `batch`, `system`, `sysbatch`), priority, datacenters/node pool, constraints, and one or more task groups. Versioned; stored durably on servers. | `nomad/structs/structs.go` `type Job struct` (fields `Type`, `Priority`, `Datacenters`, `NodePool`, `Constraints`, `Affinities`, `Spreads`, `TaskGroups`, `Update`, `Status`, `Version`, `Stable`); `JobType*` constants | "A job is a running thing." No: a job is a *document*. It can exist with zero running processes. |
| **Task group** | A set of tasks that must be co-located on the same node and scheduled as a unit, with a `Count`, shared networking, volumes, restart/reschedule/update/disconnect policies. | `type TaskGroup struct` fields `Count`, `Tasks`, `Update`, `Migrate`, `RestartPolicy`, `ReschedulePolicy`, `Disconnect`, `Networks`, `Services`, `Volumes`, `Constraints`, `Affinities`, `Spreads` | "Count is per task." No: `Count` is on the *group*; each instance of the group runs all its tasks. |
| **Task** | One process-like unit executed by a task driver, with `Driver`, driver-specific `Config`, `Resources`, `Env`, `Artifacts`, `Templates`, `Services`, `Lifecycle`, `KillTimeout`, `Identity`. | `type Task struct` | "A task is a container." Only if the driver is docker; `raw_exec`, `exec`, `java`, `qemu`, `mock_driver` are also tasks. |
| **Allocation** | A durable server-side record that *one instance of one task group has been assigned to one node*. It carries the job snapshot, the node ID, allocated resources, `DesiredStatus` (what servers want: `run`/`stop`/`evict`), `ClientStatus` (what the client reports: `pending`/`running`/`complete`/`failed`/`lost`/`unknown`), task states, deployment health, reschedule history. | `nomad/structs/alloc.go` `type Allocation struct`; `AllocDesiredStatus*` and `AllocClientStatus*` constants | "An allocation is a process." No: it is a *ledger entry* that says where a process *should* run and what the client last said about it. |
| **Evaluation** | A durable "please reconsider this job" request with a `TriggeredBy` reason, a `Status` (`pending`/`blocked`/`complete`/`failed`/`canceled`), and pointers to previous/next/blocked evals. It carries **no** placement decision itself. | `nomad/structs/eval.go` `type Evaluation struct`; `EvalTrigger*`, `EvalStatus*` constants | "An eval is the scheduling result." No: it is the *work item* that causes a scheduler run. |
| **Plan** | The scheduler's *proposal* for one evaluation: allocations to create per node (`NodeAllocation`), to stop (`NodeUpdate`), to preempt (`NodePreemptions`), plus deployment changes, tagged with the `SnapshotIndex` the scheduler used. Not durable by itself. | `nomad/structs/plan.go` `type Plan struct`, `type PlanResult struct`, `(*Plan).AppendAlloc`, `AppendStoppedAlloc`, `IsNoOp`; `(*PlanResult).FullCommit` | "A plan is what gets executed." No: the *leader* validates it against current state and may reject part or all of it. |
| **Deployment** | A durable tracker for a rolling update of a `service` job version: per-group placed/healthy/unhealthy counts, canaries, promotion, status (`running`/`paused`/`failed`/`successful`/...). Deployments create evaluations; they do not create allocations. | `nomad/structs/deployment.go` `type Deployment struct`, `type DeploymentState struct`, `DeploymentStatus*` | "Deployment == job version." Related but distinct; the deployment is the *progress record* of moving to a version. |
| **Node** | The server-side record of one client agent: fingerprinted attributes, `NodeResources`, `ReservedResources`, `Status` (`initializing`/`ready`/`down`/`disconnected`), `SchedulingEligibility`, `ComputedClass`, drain state, events. | `nomad/structs/structs.go` `type Node struct`; `NodeStatus*`, `NodeSchedulingEligible` | "Node == machine." A node is the *server's belief about* a machine, refreshed by heartbeats and fingerprints; it can be stale. |
| **Nomad server** | An agent running the control plane: Raft member, state store, RPC endpoints, scheduler workers; the *leader* additionally runs the eval broker, plan applier, heartbeat timers, deployment watcher, periodic dispatcher, GC. | `nomad/server.go` `type Server struct`, `NewServer`; leader duties in `nomad/leader.go` `establishLeadership` | "All servers do the same thing." Followers replicate state and run scheduler workers, but only the leader serializes plans and owns heartbeat timers. |
| **Nomad client** | An agent that registers as a node, heartbeats, pulls allocations, runs allocation and task runners, invokes task drivers, and reports status. Keeps local state in BoltDB. | `client/client.go` `type Client`, `NewClient`, `registerAndHeartbeat`, `watchAllocations`, `runAllocs` | "The client decides what to run." No: it executes what the servers assigned; it decides only local matters (restart backoff, hooks, driver calls). |
| **Task driver** | A plugin implementing `DriverPlugin` (`StartTask`, `WaitTask`, `StopTask`, `RecoverTask`, `Fingerprint`, ...). Built-in drivers live in `drivers/`; external drivers are separate binaries speaking gRPC via go-plugin. | `plugins/drivers/driver.go` `type DriverPlugin interface`; `drivers/rawexec`, `drivers/mock`, `drivers/docker` | "Nomad runs the process." The *driver* runs it; Nomad supervises the driver's handle. |
| **Desired state** | What servers want to be true: the job spec plus each allocation's `DesiredStatus`/`DesiredTransition`. | `Job`, `Allocation.DesiredStatus`, `Allocation.DesiredTransition` | "Desired state is only the job." Per-allocation desired status matters too (e.g. `stop`). |
| **Observed state** | What clients report: `Allocation.ClientStatus`, `TaskStates`, `DeploymentStatus` (health), `NetworkStatus`; plus `Node.Status` from heartbeats. | `Allocation.ClientStatus`, `TaskState`, `AllocDeploymentStatus`; `Node.UpdateAlloc` RPC | "Observed state is always fresh." It is *eventually* reported, batched (`allocSyncIntv = 200ms` on client, `batchUpdateInterval = 50ms` on server). |
| **Reconciliation** | Computing the difference between desired and observed and producing actions. In Nomad this happens *inside the scheduler* for each evaluation (`scheduler/reconciler`), and again *inside the client* between the pulled allocation set and local runners (`client.runAllocs` / `diffAllocs`). | `scheduler/reconciler/reconcile_cluster.go` `(*AllocReconciler).Compute`; `client/client.go` `runAllocs` | "Reconciliation is a background loop that runs periodically." No: it is *event-driven* by evaluations; there is no periodic "resync all jobs" loop. |
| **Scheduling** | Processing one evaluation: reconcile → feasibility → ranking → plan → submit. | `scheduler/generic_sched.go` `(*GenericScheduler).Process`/`process`/`computeJobAllocs`/`computePlacements` | "Scheduling happens when the job is submitted." It happens *asynchronously* on a worker after the eval is dequeued. |
| **Placement** | Choosing one node for one allocation: feasibility filtering then scoring. | `scheduler/feasible/stack.go` `(*GenericStack).Select`; `scheduler/feasible/feasible.go`; `scheduler/feasible/rank.go` | "Placement examines every node." For service jobs it stops after finding a small number of feasible nodes (`LimitIterator`), unless spread requires more. |
| **Bin packing** | Scoring that prefers nodes already more utilized, to pack work densely. Configurable to spread instead. | `scheduler/feasible/rank.go` `BinPackIterator`; `nomad/structs/funcs.go` `ScoreFitBinPack`, `ScoreFitSpread`; `SchedulerConfiguration` | "Bin packing means fill one node completely." It is a *score*, combined with anti-affinity and other scores, not a hard rule. |
| **Constraints** | Hard filters on node attributes (must match). | `type Constraint struct`; `scheduler/feasible/feasible.go` `ConstraintChecker` | "Constraints influence scoring." No: they *exclude* nodes. |
| **Affinities** | Soft preferences with weights that adjust the score. | `type Affinity struct`; `rank.go` `NodeAffinityIterator` | "Affinity guarantees placement." No: it only biases. |
| **Resource reservations** | Per-node reserved resources (`ReservedResources`) subtracted from capacity before fit checks; plus allocated resources of existing allocations. | `structs.Node.ReservedResources`, `NodeResources`; `nomad/structs/funcs.go` `AllocsFit` | "Reservation happens when the process starts." No: resources are accounted at *plan apply* time on the server, before any process exists. |
| **Restart vs reschedule** | *Restart*: same allocation, same node; task runner restarts the task per `RestartPolicy`. *Reschedule*: allocation is marked failed; the scheduler creates a *new* allocation (possibly on another node) per `ReschedulePolicy`. | `client/allocrunner/taskrunner/restarts/restarts.go` `RestartTracker`; `structs.Allocation.ShouldReschedule`, `RescheduleEligible`, `NextRescheduleTime`; `scheduler/generic_sched.go` `UpdateRescheduleTracker` | "Restart and reschedule are the same knob." They are two policies at two layers (client vs. scheduler). |
| **Health and liveness** | *Liveness of a node*: heartbeats within a TTL. *Health of an allocation*: task states plus service checks, evaluated by the client's health tracker during a deployment. | `nomad/heartbeat.go`; `client/allochealth/tracker.go` `Tracker`; `client/allocrunner/health_hook.go` | "Healthy means the process is running." For deployments, healthy means *running for `MinHealthyTime` and checks pass*; and it only matters when a deployment is active. |

## A.3 Why "submitted job" is not "running workload", and why the allocation is the right durable unit

**Observed chain [source]:** `Job.Register` writes the job and an evaluation via Raft and *returns* (`nomad/job_endpoint.go` lines ~295–370). No allocation exists yet. The scheduler runs later on some worker; the plan applier writes allocations; a client pulls them later still; a driver starts a process even later. Each hop is asynchronous and can fail or be delayed.

So at least four distinct facts exist, each with its own truth-holder:

1. "The user wants X" — `Job` (servers, durable).
2. "We decided instance 3 of group web goes on node N with 2 CPU/4 GB" — `Allocation` (servers, durable).
3. "Node N has started/finished/failed it" — `Allocation.ClientStatus` and `TaskStates` (reported by the client, persisted on servers when received).
4. "The process is actually alive right now" — only the driver on node N knows; servers know it *eventually*.

The allocation is the useful control-plane unit because:
- It is the *smallest thing a server can assign* and the *smallest thing a client can be told about*.
- It makes resource accounting possible *before* execution: the plan applier checks fit against existing allocations on the node (`AllocsFit`), not against live process metrics.
- It gives failure handling an identity: rescheduling produces a *new* allocation with `PreviousAllocation` set and a `RescheduleTracker`, so "how many times has this been retried" is a durable fact, not a memory in some process.
- It decouples the job's lifetime from the process's lifetime: an allocation can be `complete` while the job is still `running` (other instances), or the job can be stopped while allocations are still draining.

[inference] For your agent-worker platform: the equivalent of an allocation is "a durable record that task T was assigned to worker W with these resource grants." Without it, you cannot answer "is this being retried or duplicated?" after a crash. The analogy stops at the process boundary: Nomad's allocation says nothing about application-level exactly-once side effects.

## A.4 The essential separation of responsibilities

| Layer | Owns | Does **not** own | Where [source] |
|---|---|---|---|
| Server / control plane | Authoritative state (jobs, evals, allocs, nodes, deployments), Raft replication, RPC endpoints, scheduling workers, plan serialization, heartbeat timeouts, deployment progression, GC | Running processes; local filesystem of a node | `nomad/` |
| Client / data plane | Registering itself, fingerprinting, heartbeating, pulling assigned allocations, running allocation and task runners, invoking drivers, reporting status, local persistence | Deciding *where* work goes; changing a job | `client/` |
| Scheduler decision logic | Pure-ish computation from a *state snapshot* to a *plan*; no direct writes | Persisting anything; it must go through `Planner.SubmitPlan` | `scheduler/` (interfaces in `scheduler/structs/interfaces.go`: `Scheduler`, `State`, `Planner`) |
| Task driver / runtime | Starting, waiting on, stopping, recovering a task; reporting exit results and stats; fingerprinting its own availability | Allocation-level hooks (artifacts, templates, services, identity); restart policy | `plugins/drivers/driver.go`, `drivers/*` |
| Authoritative cluster state | The Raft log applied to the memdb `StateStore` on every server; only the leader accepts writes | Anything that only a client observed and has not yet reported | `nomad/fsm.go` `nomadFSM.Apply`; `nomad/state/state_store.go` |
| Local client state | BoltDB under the client data dir: allocations, task local state, task handles for driver recovery, acknowledged states, identities | Cluster-wide truth; it is a cache plus recovery info | `client/state/interface.go` `StateDB`, `client/state/db_bolt.go` |

Two hard boundaries enforced by tooling [source `GNUmakefile` `check`]: `api/` must not import internal packages; `command/` must not import `nomad/structs`. [inference] These keep the CLI and SDK on the public HTTP contract rather than on internal structs, which is why you will see `ApiJobToStructJob` conversions in `command/agent/job_endpoint.go`.

## A.5 Raft and the leader

**What needs consensus [source]:** every write in `nomad/fsm.go` `Apply` — node register/status, job register/deregister, eval updates, alloc client updates, plan results, deployments, ACL objects, keyring, and so on. The `MessageType` enum in `nomad/structs/structs.go` (line ~68 onward) is the complete list of Raft log entry kinds. Writes go through `(*Server).raftApply` in `nomad/rpc.go`.

**What does not need consensus [source]:** reads (served from the local state store, optionally stale via `QueryOptions.AllowStale`), scheduler computation (runs on a snapshot), the eval broker queue and plan queue (in-memory on the leader, rebuilt on leadership change via `restoreEvals`).

**Why leader changes matter [source `nomad/leader.go` `establishLeadership` / `revokeLeadership`]:** the leader owns in-memory coordination structures: `EvalBroker`, `BlockedEvals`, `PlanQueue` + `planApply` goroutine, heartbeat timers (`initializeHeartbeatTimers`), `deploymentWatcher`, `nodeDrainer`, `periodicDispatcher`, and the reaper goroutines (`reapFailedEvaluations`, `reapDupBlockedEvaluations`, ...). On a new leader these are rebuilt from durable state. An eval that was dequeued but not acked on the old leader is simply pending again in state, so it will be re-enqueued; the worker that held it will fail its `Eval.Ack` and the work is redone. Heartbeat timers are re-armed with `FailoverHeartbeatTTL` [source `nomad/config.go`] to avoid mass "down" marks right after failover. [repo-doc `contributing/architecture-eval-lifecycle.md`] "When a leader transition occurs, the leader queries all the Evaluations in the state store and enqueues them in its new Eval Broker."

**Parallel/optimistic work with safe coordination [source]:** scheduler workers run on *all* servers in parallel (`nomad/worker.go`), each against its own snapshot. Their plans are serialized through the leader's `PlanQueue` and evaluated by `planApply` against a fresh snapshot (`nomad/plan_apply.go` `evaluatePlan` → `evaluateNodePlan` → `structs.AllocsFit`). A conflicting plan is rejected in whole or in part; the worker gets a `RefreshIndex`, waits for its state to catch up (`snapshotMinIndex`), and retries (`retryMax` in `scheduler/util.go`, up to `maxServiceScheduleAttempts = 5`). This is optimistic concurrency control with a single serialization point.

**What clients tolerate [source `client/client.go`]:** RPC failures trigger server rediscovery (`triggerDiscovery`) and retries with backoff; `watchAllocations` uses blocking queries with `AllowStale: true` after the first fetch, so followers can serve them; heartbeats retry; allocation runners keep running regardless of server reachability. The client never stops tasks just because it cannot reach servers (unless a `Disconnect.StopOnClientAfter` policy is set — that is per-task-group configuration).

## A.6 Central correctness tradeoffs (with the invariant each one protects)

| Tension | How Nomad resolves it | Invariant |
|---|---|---|
| Desired vs. actual state | Separate fields on the same record (`DesiredStatus` vs `ClientStatus`); the scheduler reasons over both; the client reports, never sets desired | The server's desired state is never silently overwritten by a client report |
| Resource accounting vs. imperfect knowledge | Accounting is done against *allocations* (durable promises), not live usage; over-subscription is possible only if `memory_max`/oversubscription is configured | Sum of allocated resources on a node never exceeds node capacity minus reservations at plan-apply time (`AllocsFit`) |
| Fast scheduling vs. correct placement | Workers use snapshots and limited node scans; the leader re-validates at apply | A plan is never applied against a node state older than the plan's own `SnapshotIndex` (`planApply` refreshes to `max(prevPlanResultIndex, plan.SnapshotIndex)`) |
| Node liveness uncertainty | Heartbeat TTL; on miss the leader marks the node `down` or `disconnected` (if any alloc allows disconnect) via `Node.UpdateStatus`; evals are created per affected job | A node is never marked down by a non-leader (`invalidateHeartbeat` checks `IsLeader`) |
| Duplicate / delayed messages | Client alloc updates are idempotent upserts keyed by alloc ID; evals are deduplicated per job in the broker (`jobEvals`); `Job.Register` supports `IdempotencyToken` (checked in the FSM) | Re-delivering an alloc status update cannot create a second allocation |
| Task execution vs. job completion | Task exit → task runner → alloc runner aggregates `ClientStatus` → client reports; the *scheduler* decides if the job needs more allocs; `JobStatus` (`pending`/`running`/`dead`) is derived on the server | A job is `dead` only when all its evals and allocs are terminal (comment on `JobStatusDead`) |
| Availability vs. strongly coordinated writes | Reads can be stale and served anywhere; writes always go through the leader and Raft | No write is acknowledged to the caller before it is committed to Raft (`raftApply` waits for the future) |

## A.7 Common incorrect mental models (collected)

1. "`nomad job run` returns when the job is running." It returns when the job *and an evaluation* are durably stored. Nothing has been placed yet.
2. "The scheduler writes allocations." It *proposes* them in a plan; the leader's plan applier writes them.
3. "There is one scheduler." There are N workers on every server; the leader has fewer workers by default in docs but `NumSchedulers` defaults to `1` in the server config [source `nomad/config.go` `DefaultConfig`]; the agent layer may override based on CPU count (verify in `command/agent/agent.go` `serverConfig` before relying on this).
4. "Servers push work to clients." Clients *pull* with blocking queries (`Node.GetClientAllocs`, then `Alloc.GetAllocs`).
5. "A missed heartbeat immediately reschedules everything." It creates evaluations; the scheduler decides, subject to `ReschedulePolicy`, `Disconnect` settings, and node-pool/datacenter feasibility.
6. "Task restart is a scheduler decision." It is a *client* decision (`RestartTracker`); the scheduler only sees the allocation once it fails past its restart policy.
7. "A blocked evaluation is an error." It is the normal way of saying "no room right now; retry when capacity changes" (`BlockedEvals.Unblock` is called from the FSM when nodes change).
8. "Deployments place allocations." They create *evaluations*; schedulers place.
9. "Local client state is just a cache." It also holds driver task handles needed to *re-attach* to running processes after the client agent restarts (`TaskRunner.Restore` → `restoreHandle` → `RecoverTask`).
10. "Health = process alive." During deployments, health = task states + service checks over `MinHealthyTime` before `HealthyDeadline`.
