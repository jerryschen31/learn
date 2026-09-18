# 07 — Part G: A bounded first session

## Recommended first 90 minutes

Read in this order. Each item has one concrete question to answer in writing before moving on.

| # | File / doc | Time | Question to answer |
|---|---|---|---|
| 1 | `contributing/architecture-eval-lifecycle.md` | 15 min | In the "Job Registration" diagram, what exists in the state store when the CLI gets its response, and what does *not* exist yet? |
| 2 | `nomad/job_endpoint.go` `Register` + `doRegister` (lines ~93–380 only) | 20 min | Which lines construct the `Evaluation`, and under what two conditions is *no* evaluation created? |
| 3 | `nomad/fsm.go` `Apply` (the switch) + `applyUpsertJob` | 10 min | Which `MessageType` cases touch allocations, and which one touches jobs? Why does `applyUpsertJob` call `Canonicalize` again? |
| 4 | `nomad/worker.go` `run` (lines ~398–470) | 15 min | What are the three ways this loop ends up calling `sendNack`, and what does `snapshotMinIndex` wait for? |
| 5 | `scheduler/generic_sched.go` `Process` + `process` (lines ~104–330) | 20 min | Where does the plan get submitted, and what two conditions cause `process` to return `(false, ...)` so that `retryMax` runs it again? |
| 6 | `nomad/plan_apply.go` `evaluateNodePlan` (lines ~784–855) | 10 min | List, in order, the reasons a node can reject a plan before `AllocsFit` is ever called. |

Stop at 90 minutes even if item 6 is unfinished. Write your six answers in `jerry-learn/notes/session-01.md`.

## Five invariants to keep in mind

1. **Job and its triggering evaluation are committed in one Raft entry** (`JobRegisterRequest{Job, Eval}` → `applyUpsertJob`). A job never exists without something scheduled to look at it.
2. **At most one evaluation per job is in a scheduler at a time** (`EvalBroker.jobEvals` dedupe; `TestEvalBroker_Serialize_DuplicateJobID`). Concurrency is across jobs, never within one.
3. **The scheduler never writes; it only proposes** (`Planner.SubmitPlan`; `Plan.SnapshotIndex`). Every durable effect of scheduling goes through the leader's `planApply`.
4. **Plan apply validates against state at least as new as the plan's snapshot and all previously applied plans, per node, with `AllocsFit`** (`planApply` → `snapshotMinIndex(max(prevPlanResultIndex, plan.SnapshotIndex))` → `evaluateNodePlan`). Allocated resources on a node never exceed capacity at commit.
5. **Desired state flows server → client; observed state flows client → server; they are separate fields and neither side overwrites the other's** (`Allocation.DesiredStatus` vs `ClientStatus`; `Node.UpdateAlloc` only merges client fields in `nestedUpdateAllocFromClient`).

## Three things not to learn yet

1. **Raft internals and Serf membership** (`hashicorp/raft`, `nomad/server.go` `setupRaft`, `reconcileMember`). You need only two facts: `raftApply` blocks until commit, and leader-only structures are rebuilt in `establishLeadership`. Learning the consensus algorithm now would consume the time budget without changing how you read any handler.
2. **Task drivers beyond `raw_exec` and `mock_driver`, and the executor** (`drivers/docker`, `drivers/exec`, `drivers/shared/executor`). They all sit behind the same `DriverPlugin` interface; the task runner's behavior does not change per driver. Reading them early teaches Docker and cgroups, not Nomad.
3. **Deployments, canaries, and the deployment watcher** (`nomad/deploymentwatcher/`, canary logic in the reconciler). They are a *second* control loop layered on evals and allocs. Until the base loop (job → eval → plan → alloc → client → status) is solid, deployment behavior will look like magic instead of like "a watcher that creates more evals." Session 10 is the right time.

## My first checkpoint

Before our next session, write in your own words (150–300 words, no code):

> Why does Nomad have both an **Evaluation** and an **Allocation**, rather than scheduling a submitted Job directly onto a client inside the `Job.Register` request?

Cover: what would go wrong for a *node failure* if there were no evaluation; what would go wrong for *resource accounting* if there were no allocation; and which of the five invariants above would be impossible to state. I will not give you the answer until you send yours.

## Checkout-specific uncertainties

Things I could not establish from this checkout alone, with where the dependency lives:

1. **Effective scheduler worker count.** `nomad/config.go` `DefaultConfig` sets `NumSchedulers: 1`, while `contributing/architecture-eval-lifecycle.md` says "typically one per core on followers and 1/4 that on the leader." The agent layer (`command/agent/agent.go` `serverConfig`) or scheduler configuration may override this; verify before citing a number.
2. **Which tests run on macOS.** Many client and driver tests are `//go:build linux` (e.g. `task_runner_linux_test.go`) or need root (`ci/skip_non_root.go`). The set that passes on your Mac is not determinable from source alone.
3. **Enterprise behavior.** 46 `*_ce.go` files with `//go:build !ent` are CE stubs (Sentinel, quotas, multiregion, NUMA, licensing). Enterprise implementations are absent; any statement about "what Nomad does" for those features is CE-only here.
4. **ACL enforcement.** Depends on `acl.enabled` in server config; with ACLs disabled, `ResolveACL` behavior differs. Tests use `TestACLServer` to exercise the enabled path.
5. **Health-check provider.** Whether checks come from Consul or Nomad-native services depends on the jobspec `service.provider` and on a Consul agent being configured; the health tracker has separate code paths (`watchConsulEvents` vs `watchNomadEvents`).
6. **Disconnect semantics.** `TaskGroup` still carries deprecated `MaxClientDisconnect`, `StopAfterClientDisconnect`, `PreventRescheduleOnLost` alongside `Disconnect *DisconnectStrategy`; the precedence rules are in `TaskGroup.Canonicalize`/`Validate` and depend on jobspec version.
7. **Heartbeat timing in practice.** Defaults are `MinHeartbeatTTL=10s`, `HeartbeatGrace=10s`, `MaxHeartbeatsPerSecond=50`; the actual TTL scales with node count (`resetHeartbeatTimer`), and `FailoverHeartbeatTTL` applies after leader change. Real detection latency depends on cluster size and config.
8. **`-dev` mode differences.** `DevConfig` sets a single-node server+client with specific defaults (see `command/agent/config.go`); production topology (3–5 servers, many clients, TLS, ACLs) changes forwarding, staleness, and failure behavior that dev mode hides.
9. **Mock driver availability.** `drivers/mock` is compiled only without the `release` build tag; a release binary will not have `mock_driver`.
10. **Go version.** `go.mod` `go 1.26.7` vs `.go-version` `1.27.1` vs README "1.27.1+"; which one CI actually uses is defined in `.github/workflows` (not read in detail here).
11. **Repository docs drift.** `contributing/architecture-state-store.md` references `nomad/msgtypes.go`; this checkout generates `helper/raftutil/msgtypes.go`. `contributing/golang.md`'s version table stops at Nomad 1.2. Treat `contributing/` prose as slightly behind the code.
12. **`task_runner_test.go` was modified today (2026-09-17 19:40) yet `git status` was clean and its last commit is a copyright-header change.** Likely a filesystem timestamp artifact from checkout, but I did not confirm; it holds only a mock helper, so it does not affect any claim here.
13. **Product documentation.** `website/` no longer publishes docs; user-facing docs live in the external `web-unified-docs` repo, so any user-level statement in this tutorial that is not traced to source or `contributing/` is an inference.
