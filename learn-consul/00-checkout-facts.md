# 00 — Checkout facts (verified before anything else)

## Identity of this checkout

| Item | Value | Source |
|---|---|---|
| Commit | `b0296e557a0584c933a723c94be25c35086d12d8` (2026-09-16, "Add release hygiene workflow (#23829)") | `git log -1` |
| Branch | `main`, clean tree | `git status` |
| Go directive | `go 1.26.7` | `go.mod:3` |
| Local toolchain | `go1.26.7 darwin/arm64` | `go version` |
| Module | `github.com/hashicorp/consul` | `go.mod:1` |
| Version string | `2.1.0-dev` | `version/VERSION` |
| Latest changelog entry | `2.0.4 (September 10, 2026)` | `CHANGELOG.md:1` |
| Active release lines | `2.0` (CE active), `1.22`, `1.21` (LTS) | `.release/versions.hcl` |
| License | BUSL-1.1 (Business Source License) | `LICENSE`, file headers `SPDX-License-Identifier: BUSL-1.1` |
| Enterprise marker | `version.VersionMetadata` set to `ent` in enterprise builds | `version/version.go:78-81` |

Key dependency versions (from `go.mod`): `hashicorp/raft v1.7.3`, `hashicorp/raft-wal v0.4.2`, `hashicorp/raft-boltdb/v2 v2.3.1`, `hashicorp/raft-autopilot v0.1.6`, `hashicorp/serf v0.10.4`, `hashicorp/memberlist v0.6.0`, `hashicorp/go-memdb v1.3.5`, `hashicorp/go-raftchunking v0.7.0`, `google.golang.org/grpc v1.83.2`, `envoyproxy/go-control-plane/envoy v1.37.0`.

## Architecture docs: what exists and what does not

**[code]** `docs/` contains only three files: `docs/config/checklist-adding-config-fields.md`, `docs/contributing/add-a-changelog-entry.md`, `docs/contributing/fork-the-project.md`. There is **no** in-repo architecture guide in this checkout. Other markdown found: `.github/CONTRIBUTING.md`, `.github/pull_request_template.md`, `.github/ISSUE_TEMPLATE/bug_report.md`, `test-integ/README.md`, `testing/deployer/README.md`, `test/integration/consul-container/test/debugging.md`, `agent/uiserver/README.md`, `internal/go-sso/README.md`, `agent/consul/testdata/v2-resource-dependencies.md`.

Consequence: everything in this study map is grounded in Go source and tests, not prose docs. Product docs live outside the repo (`hashicorp/web-unified-docs`, per CONTRIBUTING).

## Top-level layout (what matters for the core model)

| Dir | Role |
|---|---|
| `main.go`, `command/` | CLI entry; `command/agent/agent.go` is the `consul agent` command |
| `agent/` | The agent process: HTTP/DNS/gRPC surfaces, local state, checks, cache, proxycfg, xds |
| `agent/consul/` | The **server** implementation: RPC endpoints, Raft, Serf, leader, ACL resolver, state store (`state/`), FSM (`fsm/`), streaming (`stream/`) |
| `agent/structs/` | Wire/data types shared by agent and server (`RegisterRequest`, `NodeService`, `HealthCheck`, `QueryOptions`…) |
| `acl/` | Authorizer, policies, enforcement decisions |
| `api/` (own module) | Go client library used by tests and users |
| `sdk/` (own module) | Test utilities: `testutil`, `retry`, `freeport` |
| `proto/`, `proto-public/` | Protobuf definitions; `proto-public` is its own module |
| `connect/`, `envoyextensions/` | Mesh identity helpers and Envoy extension module |
| `internal/` | `resource`, `controller`, `storage` (resource-API remnants), `gossip/libserf`, `dnsutil`, `go-sso` |
| `lib/`, `logging/`, `tlsutil/`, `ipaddr/`, `types/` | Utilities |
| `test/`, `test-integ/`, `testing/deployer/`, `testrpc/` | Integration test frameworks and helpers |
| `ui/` | Ember web UI (defer) |
| `sentinel/` | Enterprise policy hook stubs (`sentinel_ce.go`) |
| `troubleshoot/` (own module) | `consul troubleshoot` helpers |

Multiple Go modules in one repo: root, `api`, `sdk`, `proto-public`, `envoyextensions`, `troubleshoot`, `test-integ`, `test/integration/consul-container`, `testing/deployer`. The Makefile iterates `GO_MODULES` for `fmt`, `lint`, `go-mod-tidy`.

## Supported local build/test commands (from `Makefile` and `.github/CONTRIBUTING.md`)

```
make dev            # build ./bin/consul for the local platform (alias: make dev-build)
make fmt            # gofmt -s across modules
make lint           # golangci-lint across modules (.golangci.yml), plus lint-consul-retry, lint-container-test-deps
make test           # other-consul dev-build lint test-internal
make test-internal  # go test ./... in sdk, api, root (GOTAGS default: hashicorpmetrics)
make test-race      # test-internal with -race
make test-all       # every module
make test-envoy-integ         # test/integration/connect/envoy (docker + envoy)
make test-compat-integ        # test/integration/consul-container (docker)
make test-deployer-setup && make test-deployer   # test-integ via testing/deployer (terraform+docker)
make codegen        # deep-copy generation; make proto (buf + mockery)
```

Smallest useful loops (CONTRIBUTING examples):

```
go test -v ./connect
go test -v -run TestRetryJoin ./command/agent
go test -short ./agent/local          # -short skips slower tests
```

CONTRIBUTING also asks for `goimports -local github.com/hashicorp/consul/` import grouping and `go mod tidy` after dependency changes.

## Generated code and conventions (label: *generated*)

| Convention | Where | Regenerate with |
|---|---|---|
| `*.deepcopy.go` (4 files) | `agent/structs`, `agent/proxycfg`, `agent/consul/state`, `agent/config` (each has `deep-copy.sh`) | `make codegen` |
| `*.pb.go` (60 files) | under `proto/` and `proto-public/` (`buf.work.yaml` lists both) | `make proto` |
| `mock_*.go` (99 files) | mockery output next to interfaces (e.g. `agent/blockingquery/mock_FSMServer.go`, `agent/cache/mock_Type.go`) | `mockery --config .grpcmocks.yaml` (see Makefile `proto-mocks`) |
| `.github/go_test_coverage.txt` | coverage snapshot, not code | CI |

## Enterprise boundary (label: *enterprise boundary*)

**[code]** 141 files end in `_ce.go` and carry `//go:build !consulent` (e.g. `agent/consul/server_ce.go`, `agent/consul/acl_ce.go`, `acl/acl_ce.go`). These are the Community Edition (CE) halves of split files; the enterprise halves are not in this repo. Examples relevant to the core model:

- `agent/consul/server_ce.go` holds `(*Server).reconcile()` in CE.
- `acl/enterprisemeta_ce.go` makes `acl.EnterpriseMeta` (partition/namespace) a no-op; many signatures still carry `*acl.EnterpriseMeta` and `peerName` parameters.
- `agent/consul/state/catalog_ce.go`, `agent/structs/service_ai_ce_test.go`, `sentinel/sentinel_ce.go`.

When reading, treat an `EnterpriseMeta`/partition/segment argument as "tenancy plumbing that is inert in CE".

## Compatibility code (label: *compat*)

`agent/consul/fsm/decode_downgrade.go`, `structs.IgnoreUnknownTypeFlag` handling in `fsm.Apply`, `structs.DeprecatedACLRequestType`, `agent/config/deprecated.go`, `agent/cacheshim/`, `agent/consul/v2_config_entry_exports_shim.go`, and the resource-API remnants (`internal/resource`, `internal/controller`, `internal/storage`, `agent/grpc-external/services/resource`, `proto-public/pbresource`, `agent/consul/type_registry.go`). Defer all of these.

## Checkout-specific novelty worth knowing exists (then deferring)

**[code]** `agent/structs/service_ai.go` defines `ServiceAI` with roles `inference-model`, `mcp-server`, `ai-agent`; `NodeService.AI *ServiceAI` (`agent/structs/structs.go` ~line 1443) and `api.AgentServiceAI` (`api/agent.go:219`). Its comment refers to a "CAMP design summary" that is not in the repo. For the core model this is just extra metadata carried through the same registration/catalog path; do not learn it first.
