# 00 — Checkout report (what this tutorial is grounded on)

Everything in `jerry-learn/` was written against this exact checkout. If you pull new commits, symbols and line numbers can move; re-verify with `grep` before trusting a claim.

Legend used throughout the tutorial:

- **[source]** — observed directly in Go source in this checkout.
- **[repo-doc]** — stated in a document inside the repository (`contributing/`, `README.md`, Makefile comments, code comments).
- **[inference]** — my explanation or interpretation. Treat as a hypothesis to confirm by reading.

## Git state

| Item | Value |
|---|---|
| Commit | `aba089c4fa1c57b4caa4b362582ab6e1185509fe` |
| Commit date / subject | 2026-09-16 — `build: remove go-metrics compatibility shim (#28572)` |
| Branch | `main` |
| Working tree | clean at session start (the `jerry-learn/` folder is the only addition) |
| Recent history | `5ef596f5ae node identity: attempt to refresh node identity on expiration`, `0cbec72121 remove deprecated resource fields from Allocation struct`, `d94c05b44c agent: remove deprecated join configuration fields`, `508ea5a05c agent: remove unauthenticated server join` |

[inference] The recent commits show active removal of deprecated fields and compatibility paths. Expect this checkout to be *ahead* of any published Nomad release notes you read online.

## Go toolchain

| Where | Value |
|---|---|
| `go.mod` (root module `github.com/hashicorp/nomad`) | `go 1.26.7` [source] |
| `.go-version` | `1.27.1` [source] |
| `contributing/README.md` | "Install Go 1.27.1+ (gcc-go is not supported)" [repo-doc] |
| `api/go.mod` (separate module `github.com/hashicorp/nomad/api`) | `go 1.26` [source] |
| `contributing/golang.md` | policy: each Y release moves to the latest Go; the version table there is stale (stops at Nomad 1.2) [repo-doc] |

[inference] The `go` directive in `go.mod` is the minimum language version; `.go-version` is what CI and release builds use. Use 1.27.1 locally to match CI.

## Top-level directories (25)

`acl/ api/ ci/ client/ command/ contributing/ demo/ dev/ drivers/ e2e/ enos/ helper/ internal/ jobspec2/ lib/ nomad/ plugins/ scheduler/ scripts/ terraform/ testutil/ tools/ ui/ version/ website/`

Plus root files: `main.go`, `main_test.go`, `GNUmakefile`, `go.mod`, `go.sum`, `Vagrantfile`, `Dockerfile`, `CHANGELOG.md`, `CHANGELOG-unsupported.md`, `LICENSE` (BUSL-1.1, licensor IBM), `package.json` / `pnpm-*` (UI tooling).

## Contributor and architecture documentation

| Doc | What it gives you |
|---|---|
| `contributing/README.md` | Contribution process (open a Feature Request issue first), dev setup, `make bootstrap`, `make test`, `make dev`, protobuf regen, API compatibility statement, package map, the two control-flow one-liners (`Client -> HTTP API -> RPC -> StateStore` for reads; `... -> RPC -> Raft -> FSM -> StateStore` for writes) |
| `contributing/architecture-eval-lifecycle.md` | Job registration → eval broker → worker/scheduler → plan applier → deployment watcher → client pull, with Mermaid diagrams |
| `contributing/architecture-eval-states.md` | Evaluation `Status` values and the "quasi-statuses" (`scheduling`, `applying`, `delayed`, `deleted`) |
| `contributing/architecture-eval-triggers.md` | Every `EvalTriggerBy` value and what produces it; worked examples of eval fan-out |
| `contributing/architecture-state-store.md` | Raft-backed memdb; FSM determinism rules; copy-before-mutate; write-skew warning |
| `contributing/architecture-drainer.md` | Node drain subsystem (deferred in this tutorial) |
| `contributing/testing.md` | Test conventions: `shoenig/test` `must.*`, `ci.Parallel`, `ci.PortAllocator.Grab()`, `testlog.HCLogger`, `t.TempDir` |
| `contributing/checklist-rpc-endpoint.md` | Exact steps for a new RPC: struct + `MessageType` constant, `fsm.go` dispatch, state method, endpoint handler with `Authenticate`/`ResolveACL`, HTTP wrapper, changelog, docs |
| `contributing/checklist-jobspec.md` | Steps for a new jobspec field across `api/`, `nomad/structs`, conversion, diff, `tasksUpdated` |
| `contributing/checklist-command.md` | Steps for a new CLI command |
| `contributing/mock-driver.md` | The `mock_driver` task driver (dev builds only) |
| `contributing/ai.md` | AI usage: transparency, accountability, quality; you must be able to explain every line |
| `contributing/cgo.md`, `contributing/golang.md`, `contributing/issue-labels.md` | Build and process details |
| `scheduler/README.md` | Scheduler pipeline diagram and reconciler vocabulary ("buckets": migrating, lost, disconnecting, reconnecting, ignored, expiring) |
| `ci/README.md` | How the Core CI GitHub Action groups packages via `ci/test-core.json` and `tools/missing` |
| `e2e/README.md`, `e2e/framework/doc.go` | E2E suite requires a live cluster and `NOMAD_E2E=1` |
| `enos/README.md` | Upgrade testing with Enos (HashiCorp-internal infra) |
| `dev/README.md` | Local dev helpers: `dev/cluster/*.hcl` (3 servers + 3 clients configs, `cluster.sh`), `dev/docker-clients`, `dev/tls_cluster`, `dev/hooks` |
| `.github/pull_request_template.md` | PR checklist: changelog via `make cl`, tests, product docs live in the separate `web-unified-docs` repo, LLM disclosure |
| `.github/ISSUE_TEMPLATE/bug_report.md` | Required fields for a bug report |
| `website/README.md` | Product docs are **not** in this repo anymore |

## Canonical build / lint / test entry points (from `GNUmakefile`)

| Purpose | Command | Notes [source] |
|---|---|---|
| Install tooling | `make bootstrap` | = `deps lint-deps git-hooks`; installs `gotestsum`, linters, `buf`, git pre-push hook |
| Dev binary | `make dev` | runs `hclfmt` first, builds to `pkg/<os>_<arch>/nomad` and copies to `bin/nomad` |
| Debug binary | `make dev-debug` | no optimizations, debug symbols |
| Lint | `make check` | `golangci-lint`, `hclogvet`, `misspell` on website, `buf breaking`, proto-in-sync, HCL format, **api package isolation**, **jobspec2 isolation**, **`command` must not import `nomad/structs`**, `go mod tidy` clean, `helper/raftutil` msgtypes in sync |
| Unit tests (human) | `make test` | = `test-nomad` with `GOTEST_RERUN_FAILS=0`; default `GOTEST_GROUP := nomad client command drivers quick` resolved through `ci/test-core.json` |
| Unit tests (CI style) | `make test-nomad GOTEST_GROUP=<group>` | `gotestsum --rerun-fails=3 -count=1 -timeout=25m -cover` |
| Submodule tests | `make test-nomad-module GOTEST_MOD=api` | `api/` needs a built binary (see `contributing/testing.md`) |
| Narrow test (what you will actually use) | `go test ./nomad -run 'TestJobEndpoint_Register$' -count=1` | plain `go test`; `contributing/README.md` explicitly recommends this over the smoke suite |
| E2E | `make e2e-test` | needs a live cluster and `NOMAD_E2E=1`; not runnable on a laptop without provisioning |
| Integration (Vault / Consul / client intro) | `make integration-test`, `make integration-test-consul`, `make integration-test-client-intro` | env-gated (`NOMAD_E2E_VAULTCOMPAT=1` etc.) |
| Regenerate code | `make proto`, `make generate-structs`, `make generate-all` | protobuf via `buf`; `go generate` for structs codegen and `helper/raftutil` msgtypes |
| Changelog entry | `make cl` | `tools/cl-entry`; entries live in `.changelog/` (1,600+ files) |
| Local cluster | `make testcluster` (Vagrant) or `dev/cluster/cluster.sh` with `dev/cluster/*.hcl` | |

## Generated, mock, test-helper, UI, CE/enterprise boundaries (verified)

| Kind | Where | How to recognize |
|---|---|---|
| Protobuf-generated | 16 `*.pb.go` files under `plugins/`, `drivers/shared/executor`, etc. | never hand-edit; `make proto` |
| Codegen | `nomad/structs/structs_codegen.go`, `nomad/structs/generate.sh`; `helper/raftutil/msgtypes.go` via `helper/raftutil/generate.go` (`//go:generate ./generate_msgtypes.sh`) | `make check` fails if stale |
| Test mocks | `nomad/mock/` (`job.go`, `alloc.go`, `node.go`, ...) — builders for `structs.*` used by nearly every server test | `mock.Job()`, `mock.Alloc()`, `mock.Node()` |
| Test servers | `nomad.TestServer` / `nomad.TestACLServer` / `nomad.TestJoin` (`nomad/testing.go`); `client.TestClient` (`client/testing.go`); `agent.NewTestAgent` (`command/agent/testagent.go`); `testutil.NewTestServer` (spawns the real binary, `testutil/server.go`) | |
| Wait helpers | `testutil.WaitForResult`, `WaitForLeader`, `WaitForClient`, `WaitForClientStatus` (`testutil/wait.go`) | |
| Test log | `helper/testlog.HCLogger(t)` | required by `contributing/testing.md` |
| Parallel / slow gates | `ci.Parallel(t)`, `ci.SkipSlow(t, reason)` (`ci/slow.go`), `ci.PortAllocator` (`ci/ports.go`) | |
| CE vs Enterprise | 46 `*_ce.go` files with `//go:build !ent` (e.g. `nomad/plan_apply_ce.go`, `nomad/fsm_ce.go`, `scheduler/scheduler_ce.go`, `nomad/structs/structs_ce.go`, `nomad/state/state_store_ce.go`, `nomad/deploymentwatcher/multiregion_ce.go`); **zero** `*_ent.go` files exist here | Enterprise implementations are not in this repo; CE files are stubs or CE-only behavior |
| Mock task driver | `drivers/mock/` — compiled in only when **not** built with the `release` tag (`contributing/mock-driver.md`) | ideal for local experiments |
| UI | `ui/` (Ember app; `package.json`, `pnpm-workspace.yaml`) — completely separate from Go | defer |
| Docs website | `website/` — only a README saying docs moved | ignore |
| Deprecated | HCL1 `jobspec` package is deprecated per `contributing/checklist-jobspec.md`; new fields go to `jobspec2/` | |
| Public import surface | only `api/` and `plugins/` (`contributing/README.md`) | root module "does not follow semver" |

## Things I could not establish from this checkout alone

See `07-first-session.md`, section "Checkout-specific uncertainties".
