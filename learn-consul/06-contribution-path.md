# 06 — Part F: responsible future contribution path

Everything here is taken from `.github/CONTRIBUTING.md`, `.github/pull_request_template.md`, `.github/ISSUE_TEMPLATE/bug_report.md`, `Makefile`, `docs/`, `.release/versions.hcl`, `.github/workflows/`, and the test trees in this checkout. Where I infer, I say so.

## F.1 Local setup and the build/lint/test hierarchy

```
git clone <your fork>; cd consul
make dev                      # ./bin/consul and $GOPATH/bin/consul
make tools / make lint-tools  # installs supporting tools if you need lint locally
gofmt -s -w <files>; goimports -local github.com/hashicorp/consul/ <files>
```

Test hierarchy, smallest first (run from repo root unless noted):

| Level | Command | When |
|---|---|---|
| One test | `go test ./agent/local -run 'TestAgentAntiEntropy_Services$' -v` | every edit |
| One package, fast | `go test -short ./agent/consul/state` | before pushing |
| One package, full | `go test ./agent/consul -run 'TestCatalog_' -v` (boots in-process servers; slow) | when touching RPC/leader |
| Race | `go test -race ./agent/blockingquery` or `make test-race` | concurrency changes |
| Module set | `make test-internal` (sdk, api, root) / `make test-all` | before opening a PR |
| Lint | `make lint` (golangci-lint per module, `.golangci.yml`; plus `lint-consul-retry`, `lint-container-test-deps`) | before opening a PR |
| Envoy integration | `make test-envoy-integ` (docker + envoy) | mesh changes only |
| Container integration | `make test-compat-integ` (`test/integration/consul-container`) | upgrade/compat changes |
| Deployer integration | `make test-deployer-setup && make test-deployer` (`test-integ`, terraform+docker) | multi-cluster/peering |

CI mirrors this: `.github/workflows/go-tests.yml`, `reusable-unit-split.yml`, `reusable-lint.yml`, `test-integrations.yml`, nightly suites. `GOTAGS` defaults to `hashicorpmetrics` in the Makefile; plain `go test` without that tag still works for most packages **[inference; verify for metrics-related tests]**.

## F.2 Newcomer-appropriate contribution categories (from CONTRIBUTING "Some Ways to Contribute")

1. **Documentation fixes** — product docs live in `hashicorp/web-unified-docs`, not here; in-repo candidates are godoc comments, `docs/*.md` checklists, test READMEs (`test-integ/README.md`, `testing/deployer/README.md`).
2. **Test coverage** — explicitly invited ("Increase our test coverage"); `.github/go_test_coverage.txt` and the coverage badge show gaps.
3. **Issue reproduction** — CONTRIBUTING: "Provide a reproducible test case… dramatically lowers the chances it'll get fixed"; a failing test in the right package is the best reproduction.
4. **Narrowly bounded bugs** — labelled `type/bug`; comment on the issue with your approach first.
5. **Observability** — metric definitions (`getPrometheusDefs`, `RPCCounters`), log context; note the PR label `pr/no-metrics-test` implies metrics changes are expected to have tests.
6. **Developer tooling** — Makefile targets, test helpers under `sdk/testutil`, `testrpc`.

## F.3 Five contribution-shaped learning exercises (investigation and practice, not promises)

1. **Coverage probe on anti-entropy edge cases.** Read `agent/local/state_test.go`; identify a `syncService` branch without a test (e.g. `SkipNodeUpdate` when node info is already in sync, or the not-found ACL branch). Write a focused test locally, run it, keep it as a draft. Outcome: a test you understand deeply, possibly PR-able.
2. **Blocking-query header contract.** Write a table-driven test (scratch) over `agent/http.go:setMeta` covering `Index` 0/1/N, `KnownLeader`, `LastContact`, `Backend`; compare with existing `agent/http_test.go` coverage. Outcome: confidence in the caller contract; any discrepancy is a bug report candidate.
3. **Reproduce a closed issue.** Pick a closed `type/bug` in `agent/consul/leader_registrator_v1.go` or `blockingquery` history (`git log -- <path>`), check out the parent of the fix, run the test that was added, watch it fail, then pass. Outcome: fluency with the repo's test-as-reproduction culture.
4. **Metric naming audit.** List metrics emitted along the registration path (`catalog.register`, `fsm.register`, `raft.*` from the library, `leader.barrier`) and check each has a definition in `getPrometheusDefs`/`RPCSummaries`; note any emitted-but-undefined metric. Outcome: an observability finding you can verify against a running dev agent's `/v1/agent/metrics`.
5. **Docs-checklist dry run.** Read `docs/config/checklist-adding-config-fields.md`, then trace one existing field (e.g. `check_update_interval`) through `agent/config/config.go`, `builder.go`, `runtime.go`, `default.go`, `runtime_test.go`. Outcome: you can add a config field correctly without adding one.

## F.4 How to write a useful bug report (template = `.github/ISSUE_TEMPLATE/bug_report.md` + CONTRIBUTING)

- **Version/commit**: `consul version` output (includes `VersionMetadata`), and whether it reproduces on the latest release (CONTRIBUTING asks you to test against latest).
- **Topology**: number of servers/clients, DCs, agentless dataplanes, whether ACLs/TLS/Connect are enabled.
- **Configuration**: `consul info` and agent HCL for client and server (template asks for both), secrets removed (gossip keys, tokens).
- **Reproduction**: numbered steps; ideally a Go test using `agent.NewTestAgent`/`consul.testServer` or a `sdk/testutil.TestServer` script.
- **Expected vs actual**: state which layer you expected to change (local agent, gossip, catalog, cache, proxy) — this map's vocabulary.
- **Request/response**: exact `curl -i` with headers (`X-Consul-Index`, `X-Consul-KnownLeader`, `X-Consul-LastContact`, `X-Consul-Query-Backend`, `X-Cache`).
- **Logs/metrics**: `-log-level=debug` excerpts with logger names; relevant metrics from `/v1/agent/metrics`; `consul operator raft list-peers` for leader questions.
- **Minimal test case**: a failing `_test.go` in the owning package is the strongest artifact.
- Do not include internal ticket IDs (CI blocks them, per PR template).

## F.5 Questions to answer before touching sensitive subsystems

**Consensus / state store (`agent/consul/fsm`, `agent/consul/state`, `raftApply`)**
- Is my change to `fsm.Apply` deterministic and backward compatible with entries already in existing Raft logs and snapshots? Do I need a new `structs.MessageType` and the `IgnoreUnknownTypeFlag` behaviour for mixed-version clusters?
- Does a schema/index change alter which index `blockingQuery` returns, and could it cause busy loops (index 0) or missed wake-ups?
- Is the transaction still atomic and does `IsSame`-style churn avoidance still hold?
- Does the snapshot/restore path (`fsm/snapshot*.go`) round-trip the new field?

**Membership (`server_serf.go`, `client_serf.go`, `router`, `leader_registrator_v1.go`)**
- Which events are leader-only and which run on every server/client? Could a follower now attempt a Raft write?
- Does the change alter tag parsing (`metadata.IsConsulServer`) and thus mixed-version joins?
- What is the false-positive behaviour under partition, and is reap/left handling still idempotent?

**API compatibility (HTTP/DNS/gRPC, `api/` module)**
- Is the new field optional and omitted-when-empty in JSON? Do old clients ignore it safely? Does `api/` need a matching change (separate module, separate versioning)?
- Do headers/status codes stay identical for existing callers? Is there a changelog entry (`docs/contributing/add-a-changelog-entry.md`) and backport label consideration (`.release/versions.hcl`)?

**ACL / security (`acl/`, `agent/consul/acl*.go`, `aclfilter`, `vetRegisterWithACL`)**
- Which enforcement point am I changing (agent vet, leader vet, read filter, xDS authorize) and are the others still consistent?
- Could a token gain a capability implicitly (see the 2.0.4 `mesh:write` changes for precedent)? Is this a security disclosure rather than a PR (CONTRIBUTING: `security@hashicorp.com`)?
- Are `ResultsFilteredByACLs` semantics preserved?

**Service mesh (`proxycfg`, `xds`, `leader_connect_ca.go`)**
- Which Envoy versions are supported (`.github/workflows/verify-envoy-version.yml`, `reusable-get-envoy-versions.yml`) and do golden files in `agent/xds/testdata` need regeneration?
- Does the change affect the data plane's fail-static behaviour or certificate rotation?
- Is there an integration case under `test/integration/connect/envoy/case-*` to extend?

## F.6 Required contributor process (as found in this checkout)

- **Before coding**: open/find an issue; comment with your approach (CONTRIBUTING "talk to us").
- **PR**: from your fork against `hashicorp/consul` `main`; keep it small; draft PRs welcome; link the issue; fill `.github/pull_request_template.md` (Description, Testing & Reproduction steps, Links, PR checklist: updated test coverage, docs updated, backport labels, "not a security concern"; PCI review checklist).
- **Labels**: `pr/no-changelog`, `pr/no-backport`, `pr/no-metrics-test`, `backport/<x.y>` or `backport/all`; active lines from `.release/versions.hcl` (2.0 CE-active, 1.22, 1.21 LTS).
- **Changelog**: `docs/contributing/add-a-changelog-entry.md` (`.changelog/` entries) when users need to know.
- **Formatting/lint**: `gofmt -s`, `goimports -local github.com/hashicorp/consul/`, `go mod tidy`; CI runs lint and tests.
- **Backport policy**: CE backports to the current major; Enterprise to N-2 and LTS (CONTRIBUTING "Backport Policy").
- **License**: BUSL-1.1 headers (`// Copyright IBM Corp. 2024, 2026` / `SPDX-License-Identifier: BUSL-1.1`) on new files; **no CLA text found in this checkout** — check the PR flow on GitHub for any CLA bot **[not established from repo]**.
- **Architecture review**: no formal RFC/ADR process is documented in this checkout; CONTRIBUTING's "talk to us in the issue" is the stated mechanism. `CODEOWNERS` routes reviews to `@hashicorp/consul-selfmanage-maintainers` and `@hashicorp/team-consul-core`.
