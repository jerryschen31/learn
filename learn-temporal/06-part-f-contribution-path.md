# Part F — A contribution path (not a fake contribution plan)

Everything below is derived from `CONTRIBUTING.md`, `AGENTS.md`, `docs/development/testing.md`, `docs/development/new-rpcs.md`, `docs/development/temporal-cla.md`, `.github/PULL_REQUEST_TEMPLATE.md`, `.github/ISSUE_TEMPLATE/*`, `.github/CODEOWNERS`, `.github/workflows/*`, and the `Makefile` on this commit. Nothing here claims an issue is open or that any work is pre-approved.

## F.1 Local development setup and the test hierarchy

**Setup (observed in `CONTRIBUTING.md`):**
- Go per `go.mod` (`1.27.0`), `protoc` only if touching protos, Temporal CLI, Docker for optional dependencies.
- `make` (first build, installs tools into `.bin/`), then `make bins`.
- `make start` runs the server on in-memory SQLite; `make start-sqlite-file`, `make start-postgres`, `make start-mysql`, `make start-cass-es` need `make start-dependencies` and `make install-schema-*`.
- `temporal operator namespace create -n default`.
- Dynamic config for dev: `config/dynamicconfig/development-sql.yaml` (or `-cass.yaml`).
- GoLand run config: package `go.temporal.io/server/cmd/server`, args `--env development-sqlite --allow-no-auth start` (pattern from `CONTRIBUTING.md`).

**Three test tiers (observed):**

| Tier | What | Command | Where | Needs |
|---|---|---|---|---|
| Unit | No external deps; gomock | `make unit-test` (all) or `go test -tags test_dep ./pkg -run Suite -testify.m Name` | next to code (`*_test.go`) | nothing |
| Integration | Server ↔ dependency (Cassandra/SQL/ES) | `make integration-test` | `common/persistence/tests`, `common/persistence/persistence-tests`, `temporaltest` | `make start-dependencies` unless SQLite-only suites |
| Functional | End-to-end through an in-process cluster | `make functional-test` (Makefile defaults to Cassandra) or `go test -tags test_dep ./tests -run Suite -persistenceType=sql -persistenceDriver=sqlite` | `tests/`, `tests/ndc`, `tests/xdc` | SQLite works for most; ES/Cassandra for some |

Build tags: always `-tags test_dep` (per `AGENTS.md`; `Makefile` `ALL_TEST_TAGS := $(ALL_BUILD_TAGS),test_dep,$(TEST_TAG)`). `disable_grpc_modules` speeds unit compilation (`testing.md`). Env knobs: `TEMPORAL_TEST_LOG_LEVEL`, `TEMPORAL_TEST_LOG_FILE`, `TEMPORAL_TEST_OTEL_OUTPUT`, `TEMPORAL_TEST_TIMEOUT` (`testing.md`).

Test conventions (observed, enforced by linters): `require` over `assert`; no `time.Sleep` (use `await.Require` / `require.Eventually`); `parallelsuite.Suite` instead of testify `Suite`; `testvars` for identifiers; `t.Parallel()` everywhere (`make parallelize-tests`); `InDelta` for floats; in suites `s.Require().NoError(err)`.

Lint/format: `make lint-code-fast` (changed packages vs `GOLANGCI_LINT_BASE_REV`, config `.github/.golangci.yml`), `make lint-code`, `make fmt-imports`, `make lint-protos`, `make lint-api`, `make workflowcheck` (system workflows determinism), `make lint-nilaway` (CI job in `linters.yml`).

CI (`.github/workflows/run-tests.yml`): "Test setup" computes scope (smoke vs full), a matrix of unit / functional (`make functional-test-coverage`, per DB, sharded) / ndc / xdc jobs; `linters.yml` runs actions/protos/api/workflowcheck/nilaway lints; `govulncheck.yml`; `flaky-tests-report.yml`.

## F.2 Smallest newcomer-appropriate categories (in increasing risk)

1. **Documentation corrections** in `docs/` where the doc and code disagree (the architecture docs pin symbols to an older commit `ef49189…`; several line numbers and a few names have drifted). Low risk, high signal, still reviewed by owners.
2. **Targeted unit tests** for an existing, untested branch in a pure function or small handler (validators, retry math, task generator, timer executor cases). Must follow the test conventions above.
3. **Issue reproduction**: a functional test (or a minimal driver using `taskpoller`) that reproduces a reported behavior, attached to the issue even if no fix is proposed.
4. **Observability improvements**: a missing structured tag on an existing log line, a metric on an existing error branch. Check `CODEOWNERS` (e.g. `service/history/queues/`, `service/history/shard/` → `@temporalio/oss-foundations`).
5. **Narrowly bounded bug fixes** where the fix is local, the test is deterministic, and no persistence/proto/state-machine contract changes.

## F.3 Five contribution-shaped learning exercises (investigations, not claims of accepted work)

1. **Doc drift audit.** Take `docs/architecture/history-service.md` and `workflow-lifecycle.md`; for each linked symbol, find its current location on `main` and note renames (e.g. the doc links `history_engine.go#L288 Start()`; now `history_engine.go:377`; the doc's `update_workflow_util.go#L37` is now `GetAndUpdateWorkflowWithNew` at `:14`). Outcome: a table you could turn into a docs PR after discussing with maintainers.
2. **Stale-task test coverage map.** For every `Stamp`/`Attempt`/`ScheduledEventID` comparison in `transfer_queue_active_task_executor.go` and `timer_queue_active_task_executor.go`, find the unit test that exercises it (`TestProcessWorkflowTask_StampMismatch`, `TestWorkflowTaskTimeout_StampMismatch`, `TestActivityRetryTimer_Fire`, ...). Identify any comparison with no test and write one locally.
3. **Retry-state truth table.** From `MutableStateImpl.RetryActivity` and `workflow/retry.go`, build a table (retry policy set? cancel requested? timeout type? max attempts? non-retryable error type?) → `RETRY_STATE_*`. Compare with `service/history/workflow/retry_test.go`; note untested rows.
4. **Dynamic config default audit.** Pick 10 settings in `common/dynamicconfig/constants.go` used on the paths you traced (`FrontendRPS`, `MatchingLongPollExpirationInterval`, `HistoryTaskDLQEnabled`, ...); confirm each description matches how the code uses it. Description fixes are doc-class changes (there is a `gendynamicconfig` tool in `cmd/tools`; check whether descriptions are generated before editing).
5. **Worker-crash reproduction harness.** Using `tests/testcore.FunctionalTestBase` and `taskpoller.TaskPoller`, write a local functional test that polls a workflow task, does *not* respond, and asserts the `WorkflowTaskTimedOut` event and transient retry (model on `tests/transient_task_test.go`). Then vary sticky vs non-sticky.

## F.4 Turning an observation into a high-quality bug report

Use `.github/ISSUE_TEMPLATE/bug_report.md` (labels `potential-bug`) and fill it with:

- **Version/commit**: server version (`temporal-server --version` or `git rev-parse HEAD`), persistence type/driver, SDK language+version, whether dynamic config differs from defaults (attach the relevant keys).
- **Reproduction**: minimal workflow + worker (or a functional test using `FunctionalTestBase`), exact CLI/SDK calls, timing (timeouts set), and whether it reproduces on SQLite and on Cassandra/Postgres (say which you tried).
- **Expected vs actual**: state it in terms of history events and mutable state (e.g. "expected `WorkflowTaskTimedOut` then transient WFT; observed workflow stuck at `WorkflowTaskScheduled` with no pollers metric").
- **Logs/metrics**: server logs filtered by `wf-id`/`wf-run-id` tags (`common/log/tag`), the specific queue-executor lines (`"Fail to process task"`, `"Critical error processing task, retrying."`, DLQ lines), and metrics such as `TaskFailures`, `TaskAttempt`, task-queue poller counts. Attach `tdbg` output for the shard/task if relevant.
- **Minimal test case**: a failing unit or functional test, following `testing.md` conventions, named after the behavior.
- **Affected subsystem**: name the package (`service/history/queues`, `service/matching`, `common/persistence/sql`, ...) and the handler/executor function, and tag the likely owner team per `CODEOWNERS`.

## F.5 Questions to answer before proposing a change in a sensitive area

**Persistence** (`common/persistence/**`, `schema/**`):
- Does it change what is stored or only how it is read? Is a schema migration needed (`schema/*/versioned`) and is it backward compatible with a running older version?
- Does it hold on every backend (Cassandra CAS batch vs SQL tx)? Which suites in `common/persistence/tests` cover it, and do they run on all backends?
- Does it affect `RangeID`/`DBRecordVersion` fencing or transaction size limits?

**History state machines** (`service/history/workflow/**`, `api/**`, `tasks/**`):
- Which `Add*Event` / `Apply*Event` pairs change? Do replay/rebuild (`mutable_state_rebuilder.go`) and replication still reconstruct the same state?
- Which tasks are generated, and do the stale-task checks (`Stamp`, `Attempt`, version) still identify obsolete tasks?
- Is the change safe when the task is executed twice, or after a shard reload?

**Task dispatch** (`service/history/queues/**`, `service/matching/**`):
- What are the new error classes and how does `executable.HandleErr` classify them (drop / retry / DLQ)?
- Does the change alter sync-match vs spool behavior, ack levels, or forwarding? Which matching suites (classic/pri/fair) must pass?

**API / proto**:
- Public API changes go to the `api` repo first (`docs/development/new-rpcs.md`, `CONTRIBUTING.md` "Working with local API changes"); internal ones to `proto/internal` + `make proto`; both need `lint-api`/`lint-protos` and `buf` breaking checks (`develop/buf-breaking.sh`).
- Are new fields optional and defaulted for old clients and old servers?

**Compatibility**:
- Rolling upgrade: can an old History host process tasks written by a new one and vice versa? Is the feature gated by dynamic config with a safe default?
- Does it change metrics/log names that dashboards depend on (`develop/docker-compose/grafana`)?

## F.6 Repository contribution requirements you must follow

- **CLA**: required before merge (`CONTRIBUTING.md`, `docs/development/temporal-cla.md`).
- **PR template** (`.github/PULL_REQUEST_TEMPLATE.md`): What changed / Why / How did you test it (checkboxes: built, ran locally, existing tests, new unit tests, new functional tests) / Potential risks.
- **Titles**: PR titles become commit messages; start uppercase, no trailing period, not generic (`CONTRIBUTING.md` "Commit Messages And Titles"; Chris Beams style).
- **Tests**: add tests for new behavior, both success and failure paths (`AGENTS.md`); follow `testing.md` conventions (require, await, parallelsuite, testvars, `t.Parallel()`).
- **Style**: mimic surrounding code; comments explain *why*; no new third-party deps; handle all errors; `logger.Fatal` for invariant violations, `DPanic` for important-but-non-fatal (`AGENTS.md`).
- **Lint/format before pushing**: `make lint-code-fast`, `make fmt-imports`; CI additionally runs `lint-actions`, `lint-protos`, `lint-api`, `workflowcheck`, `lint-nilaway`.
- **Codegen**: if you touch `.proto` or `//go:generate`-annotated interfaces, run `make proto` / `make go-generate` and commit the generated output (`AGENTS.md` "Regenerate").
- **Owners**: `.github/CODEOWNERS` routes review; expect `@temporalio/server` plus area teams (`oss-foundations`, `oss-matching`, `act`, `nexus`).
- **Placeholders check**: `.github/workflows/check-pr-placeholders.yml` exists; fill the template, do not leave its italics.
