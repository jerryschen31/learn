# Temporal Server: first-principles study map

Generated against this checkout, not against generic Temporal knowledge.

| Fact | Value (observed) |
|---|---|
| Commit | `736e89ab9bc26d69ebe489e3db6c7ab144f2429e` (2026-09-17, "Use compatible metric capture API in user data test (#12142)") |
| `git describe` | `v1.29.0-135.0-2259-g736e89ab9` (i.e. ~2259 commits past the v1.29.0 tag on `main`) |
| Go | `go 1.27.0` in `go.mod`; module `go.temporal.io/server` |
| Key deps | `go.temporal.io/api v1.63.6-0.20260909222256-20151aa90480` (public protos, external repo), `go.temporal.io/sdk v1.48.0` (used by the internal worker service and tests), `go.uber.org/fx v1.24.0` (DI/bootstrap), `github.com/temporalio/ringpop-go` (membership), `github.com/gocql/gocql` (Cassandra), `go.opentelemetry.io/otel`, `github.com/uber-go/tally/v4`, `github.com/prometheus/*`, `github.com/stretchr/testify`, `go.uber.org/mock` |
| Top-level dirs | `api/` (generated Go from internal protos), `chasm/`, `client/`, `cmd/`, `common/`, `config/`, `docs/`, `proto/`, `schema/`, `service/`, `temporal/`, `temporaltest/`, `tests/`, `tools/`, `nexusworkflowref/` |
| Architecture docs | `docs/architecture/README.md`, `history-service.md`, `matching-service.md`, `workflow-lifecycle.md`, `speculative-workflow-task.md`, `in-memory-queue.md`, `retry.md`, `chasm.md`, `worker-commands.md`, `workflow-update.md`, `message-protocol.md`, `nexus.md`, `schedules.md`, `circuit-breaker.md`, `effect-package.md` |
| Dev docs | `CONTRIBUTING.md`, `docs/development/testing.md`, `docs/development/new-rpcs.md`, `docs/development/tracing.md`, `docs/development/temporal-cla.md`, `AGENTS.md` (agent-oriented dev guide, also useful for humans) |
| Build/test | `make bins`, `make unit-test`, `make integration-test`, `make functional-test`, `make lint-code-fast`, `make proto`; single test: `go test -tags test_dep ./path -run TestSuite -testify.m TestName` |

Note: `AGENTS.md` lists a `/components` directory that does not exist in this checkout. See ambiguities in `07-part-g-start-here.md`.

## Files

1. `01-part-a-mental-model.md` — the problem, the guarantees, the vocabulary, one running example
2. `02-part-b-repo-map.md` — where things live, what to read now vs later
3. `03-part-c-trace-start-workflow.md` — StartWorkflowExecution → Worker → Activity → crash recovery, stage by stage, with diagrams and glossary
4. `04-part-d-design-notebook.md` — twelve design ideas, each with problem/invariant/mechanism/code/failure/misconception/check question
5. `05-part-e-apprenticeship-plan.md` — twelve 60–120 minute sessions
6. `06-part-f-contribution-path.md` — local dev, test hierarchy, newcomer-shaped work, bug-report quality, pre-change questions
7. `07-part-g-start-here.md` — first 90 minutes, five invariants, three deferrals, first checkpoint, ambiguities found

## How to read citations

- Paths are repository-relative. Line numbers are as of this commit and will drift.
- "Observed" means I read the code. "Inference" means I am explaining a reason the code does not state. I mark inferences explicitly.
- Things marked **defer** are real and important but not needed for the conceptual core: generated code (`api/`, `*_mock.go`, `*_gen.go`), fx wiring (`*/fx.go`), CHASM (`chasm/`), replication/XDC, Nexus, worker versioning/deployments, archival.
