# 06 — Part F: Responsible contribution path

Everything here is taken from this checkout's `contributing/`, `GNUmakefile`, `.github/`, `ci/`, and `LICENSE`. Tags: **[repo-doc]**, **[source]**, **[inference]**.

## F.1 Local environment, build, format, lint, tests

| Step | Command / file | Source |
|---|---|---|
| Toolchain | Go 1.27.1+ (`.go-version`); `go.mod` says `go 1.26.7`; gcc-go unsupported | [repo-doc `contributing/README.md`], [source] |
| Bootstrap tools | `make bootstrap` (= `deps lint-deps git-hooks`: installs `gotestsum@v1.10.0`, `golangci-lint`, `hclogvet`, `misspell`, `buf`, `copywrite`, and a git pre-push hook) | [source `GNUmakefile`] |
| ulimit | raise to ≥1024 open files | [repo-doc] |
| Dev build | `make dev` → `bin/nomad`; `make dev-debug` for delve-friendly builds; UI needs `make dev-ui` (Ember) | [source] |
| Run locally | `sudo bin/nomad agent -dev` (Linux; sudo needed for `exec`/cgroups); optional `consul agent -dev` for service checks; `dev/cluster/*.hcl` + `cluster.sh` for a 3+3 topology | [repo-doc], [source `dev/`] |
| Format | `make hclfmt` (HCL); Go formatting is enforced by `golangci-lint` (`.golangci.yml`) | [source] |
| Lint | `make check` = golangci-lint (root and `api/` with `modernize`), `hclogvet`, `misspell`, `buf breaking` against `PROTO_COMPARE_TAG`, proto and HCL in-sync checks, **`api/` isolation**, **`jobspec2/` isolation**, **`command/` must not import `nomad/structs`**, `go mod tidy` clean, `helper/raftutil` msgtypes in sync | [source] |
| Targeted test | `go test ./<pkg> -run '<TestName>$' -count=1 -v` — the README explicitly recommends testing the subsystem you touch and letting CI run the rest | [repo-doc] |
| Package-group tests | `make test` (no reruns) or `make test-nomad GOTEST_GROUP=<nomad|client|command|drivers|quick>`; groups resolved through `ci/test-core.json` by `tools/missing` | [source `GNUmakefile`, `ci/README.md`] |
| `api/` tests | `make test-nomad-module GOTEST_MOD=api` after `make dev` (they spawn the real binary) | [repo-doc `contributing/testing.md`] |
| Test conventions | `github.com/shoenig/test` (`must.*`, `test.*`), `ci.Parallel(t)` in every top-level test, `ci.PortAllocator.Grab()`, `t.TempDir`, `t.Setenv`, logging via `helper/testlog.HCLogger(t)` | [repo-doc `contributing/testing.md`] |
| Root-required tests | `ci/skip_non_root.go` gates tests that need root; many client/driver tests are `//go:build linux` | [source] |
| E2E boundary | `make e2e-test` requires a provisioned cluster (`e2e/terraform`) and `NOMAD_E2E=1`; suites in `e2e/*` use `e2e/framework`; upgrade tests use Enos from a private repo | [repo-doc `e2e/README.md`, `enos/README.md`] |
| Integration boundary | `make integration-test` (Vault compat), `integration-test-consul`, `integration-test-client-intro`; env-gated | [source] |
| Regenerate | `make proto` (buf), `make generate-structs`, `go generate ./helper/raftutil/` | [source] |
| Changelog | `make cl` → `.changelog/<PR>.txt` via `tools/cl-entry`; required for user-facing changes | [repo-doc PR template] |

## F.2 Public import surfaces vs internal implementation

[repo-doc `contributing/README.md`] "Only the `api/` and `plugins/` packages are intended to be imported by other projects. The root Nomad module does not follow semver and is not intended to be imported directly by other projects."

[source] `api/` is its own Go module (`api/go.mod`), and `make check` fails if `api/` depends on any internal package. `jobspec2/` is likewise isolated (it may import `api/` only). `command/` may not import `nomad/structs`.

Why it matters for contribution scope [inference]:
- Changes to `api/` types are **compatibility-sensitive**: external tools (Terraform provider, autoscaler, community CLIs) depend on them. Field renames or JSON changes need care and a changelog entry; `checklist-jobspec.md` describes the api ⇄ structs conversion and JSON encoding decisions.
- `plugins/` defines the gRPC contracts for external drivers/devices/CSI; `make check` runs `buf breaking` so proto changes cannot silently break third-party plugins.
- `nomad/`, `scheduler/`, `client/` are internal: you may refactor freely *within the project's review process*, but there is no external API promise. RPC and Raft message compatibility still matters *within* a cluster during rolling upgrades — hence `ServersMeetMinimumVersion` guards and "append, never renumber" for `MessageType` [repo-doc `checklist-rpc-endpoint.md`].

## F.3 Newcomer-appropriate contribution categories (with evidence they are welcome)

1. **Documentation fixes** in `contributing/` — e.g. the Go version table in `contributing/golang.md` stops at Nomad 1.2; `architecture-state-store.md` references `nomad/msgtypes.go` whereas the generated file in this checkout is `helper/raftutil/msgtypes.go`. Product docs live in `web-unified-docs`, not here.
2. **Test improvements** — `contributing/testing.md` explicitly invites refactoring testify tests to `shoenig/test` ("consider separate commits / PRs"), and asks for parallel-safe, `TempDir`-based tests.
3. **Issue reproduction** — a minimal unit test at the scheduler harness (`scheduler/tests.NewHarness`) or `nomad.TestServer` level attached to an existing issue is high value and low risk.
4. **Narrowly bounded bug fixes** — after an issue discussion, per `contributing/README.md` ("post in the issue before starting").
5. **Observability / debuggability** — log context (`eval_id`, `alloc_id`, `node_id` keys), metrics documented in the metrics reference (checklist mentions metrics docs), clearer placement-failure descriptions.
6. **Developer tooling** — `tools/`, `dev/`, Makefile targets, `ci/test-core.json` grouping (`tools/missing` enforces every package is covered).

## F.4 Five contribution-shaped learning exercises (practice, not open issues)

1. **Placement-failure explanation audit.** For each `FeasibilityChecker` in `scheduler/feasible/feasible.go`, find the string it records in `EvalContext.Metrics()` (`ConstraintFiltered`, `ClassFiltered`, etc.). Write a table mapping checker → metric key → what `nomad eval status` shows. Propose (locally) one clearer message. Test: `scheduler/feasible/feasible_test.go`.
2. **Restart/reschedule boundary test.** Write a scratch table-driven test for `structs.Allocation.NextRescheduleTime` (`nomad/structs/alloc_test.go` already has cases) covering `constant`, `exponential`, `fibonacci` delay functions with `MaxDelay`. Compare with `ReschedulePolicy.Validate`.
3. **Heartbeat failover latency.** Read `TestHeartbeat_Server_HeartbeatTTL_Failover`; compute, from `nomad/config.go` defaults, the worst-case time to mark a node down after a leader change; write it up with the exact fields (`MinHeartbeatTTL`, `HeartbeatGrace`, `FailoverHeartbeatTTL`, `MaxHeartbeatsPerSecond`).
4. **Client restore matrix.** Using `client/allocrunner/taskrunner/task_runner_linux_test.go` `TestTaskRunner_Restore_*` as the model, enumerate the (task state × driver recoverable?) matrix and which branch of `TaskRunner.Restore`/`restoreHandle` each hits. Identify any cell without a test.
5. **Plan-apply partial commit walkthrough.** Instrument (locally only) `nomad/plan_apply.go` `evaluatePlanPlacements` with a debug log of `RejectedNodes` and run `TestPlanApply_EvalPlan_Partial`; write the sequence of `RefreshIndex` values the worker would see. Revert the instrumentation.

## F.5 Writing a high-quality issue report

Fields required by `.github/ISSUE_TEMPLATE/bug_report.md` [repo-doc], with what to put in each [inference]:

- **Nomad version**: output of `nomad version` for every server and client (they can differ during upgrades); if built from source, the commit (`git rev-parse HEAD`, here `aba089c4fa`).
- **Operating system and environment details**: OS/kernel per role, topology (how many servers/clients, `-dev` or not), task drivers involved, Consul/Vault present or not, ACLs on/off, node pools.
- **Issue**: one paragraph; name the subsystem (scheduler, plan apply, client restore, deployment watcher).
- **Reproduction steps**: numbered, minimal; include the **job file** (template asks for it), relevant agent config blocks (`server { }`, `client { }`, `telemetry`), and CLI commands.
- **Expected vs actual result**: state the invariant you expected (e.g. "allocation should be rescheduled after `delay`").
- **IDs**: evaluation IDs (`nomad eval status`), allocation IDs (`nomad alloc status -verbose`), deployment ID, node ID; these let a maintainer follow `PreviousEval`/`NextEval`/`PreviousAllocation` chains.
- **Logs**: server logs around the eval IDs (grep `eval_id=`), client logs around the alloc ID (`alloc_id=`), at `debug` or `trace` level; `nomad operator debug` bundle if you can (`command/operator_debug.go`).
- **Metrics**: relevant `/v1/metrics` counters (`nomad.worker.*`, `nomad.plan.*`, `nomad.heartbeat.*`, `nomad.client.*`).
- **Minimal test case**: if you can express it as a scheduler harness test or `TestServer` test, attach it; it is the fastest path to a fix.
- The template also notes logs may be emailed to a listed address referencing the issue; use it for sensitive logs.

## F.6 Questions to answer before changing each area

**Scheduler logic (`scheduler/`)**
- Which job types are affected (`service`, `batch`, `system`, `sysbatch`)? Does `SchedulerVersion` need bumping (mixed-version clusters dequeue only matching versions)?
- Does the change alter which nodes are *feasible* (correctness) or only *ranked* (preference)?
- Is the reconciler's bucket classification affected (`classifyAllocs`)? Does it interact with deployments/canaries?
- Does `tasksUpdated`/`inplaceUpdate` need to learn about a new field?
- Which harness tests cover it, and is there a property test (`reconcile_cluster_prop_test.go`)?

**Plan application (`nomad/plan_apply.go`)**
- Does the change alter what is valid on down/disconnected/ineligible nodes? Can it cause a plan to be accepted that overcommits?
- Does it change `RefreshIndex` semantics or the optimistic snapshot handling (`inFlightPlans`)?
- Does it require a new field in `ApplyPlanResultsRequest`, and therefore a compatible FSM change and `ServersMeetMinimumVersion` guard?

**State store / Raft / FSM (`nomad/state`, `nomad/fsm.go`, `nomad/structs`)**
- Is the FSM change deterministic (no clocks, no randomness)? Are inputs copied before mutation?
- Is a new `MessageType` appended (never renumbered)? Is the `helper/raftutil` mapping regenerated?
- Do `Snapshot`/`Restore` need to persist a new table (`state_store_restore.go`, `schema.go`)? Is there an upgrade path for old snapshots?
- Which single component owns each new field (write-skew rule)?

**Client / driver execution (`client/`, `drivers/`, `plugins/drivers`)**
- Does local state (`StateDB`) change? Is there an upgrade in `client/state/upgrade.go`? Can old BoltDB files be read?
- Is the hook order or restore path affected? Is the change OS-specific (build tags)?
- Does the driver protocol change (proto → `buf breaking`)?

**API compatibility (`api/`, `command/agent/*_endpoint.go`)**
- Is the JSON shape stable? Are new fields optional and canonicalized? Is `ApiJobToStructJob` updated with tests?
- Do old servers/clients still work when the field is absent?

**ACL / identity (`acl/`, `nomad/auth`, `nomad/encrypter.go`)**
- Does every affected RPC still call the right `Authenticate*` before forwarding and `ResolveACL` after? Is the nil-ACL rule respected? Is the change security-reviewed (PR template asks "Changes to Security Controls")?

**Consul integration (`command/agent/consul`, `client/serviceregistration`)**
- Which Consul versions and namespaces are supported (`version_checker.go`, `namespaces_client.go`)? Is the change gated behind `NOMAD_E2E_CONSULCOMPAT` tests? Is Nomad native service discovery (`nsd`) affected symmetrically?

## F.7 Contributor process, licensing, review, testing, generated code — as stated by the repo

- **Process** [repo-doc `contributing/README.md`]: open a Feature Request issue with problem, proposed solution, test plan; wait for discussion; for existing issues, post before starting.
- **License** [source `LICENSE`]: Business Source License 1.1, licensor IBM, "Nomad Version 1.7.0 or later", with an Additional Use Grant restricting competitive hosted offerings. There is **no CLA document in this checkout** (`.github/` has `CODE_OF_CONDUCT.md`, `CODEOWNERS`, issue and PR templates, workflows). Whether a CLA bot runs on PRs cannot be determined from the checkout alone.
- **Code owners** [source `.github/CODEOWNERS`]: default `@hashicorp/github-nomad-core @hashicorp/nomad-eng`.
- **PR requirements** [repo-doc `.github/pull_request_template.md`]: description, testing/reproduction steps, links to issues, changelog entry via `make cl` for user-facing changes, tests for new functionality and bug fixes, product docs in `web-unified-docs`, upgrade-guide notes if needed, **LLM usage disclosure** per `contributing/ai.md`; reviewers add backport labels and squash-merge; a security-controls section must be answered.
- **AI usage** [repo-doc `contributing/ai.md`]: disclose AI use specifically; human-owned accounts only; you must be able to explain every line; no low-effort pasted output.
- **Testing** [repo-doc `contributing/testing.md`]: new features, fixes, refactors include tests; conventions listed in F.1.
- **Generated code** [source `GNUmakefile` `check`]: protobufs (`make proto`), structs codegen, `helper/raftutil` msgtypes must be regenerated and committed; CI fails on drift. Never hand-edit `*.pb.go`.
- **Copyright headers** [source]: files carry `// Copyright IBM Corp. 2015, 2026` and `// SPDX-License-Identifier: BUSL-1.1`; a `copywriteheaders` Makefile target exists.
- **CI** [repo-doc `ci/README.md`]: `make check`, `make missing` (every package must be in a test group), matrix of `test-*` groups, `test-api`, compile on Linux/macOS/Windows.
