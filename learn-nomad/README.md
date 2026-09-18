# jerry-learn — Nomad Server and Client internals, from first principles

A source-grounded learning map for the HashiCorp Nomad codebase, written against commit `aba089c4fa` on `main` (2026-09-16). It is not a user guide and not a Kubernetes comparison; it is a reading plan for someone who wants to investigate issues, understand tests, review scoped changes, and eventually contribute.

Every source-level claim is tagged **[source]** (verified in this checkout), **[repo-doc]** (from `contributing/` or other in-repo docs), or **[inference]** (my interpretation). File paths are repository-relative. Re-verify with `grep` after pulling new commits.

## Files

| File | Part | What it is |
|---|---|---|
| `00-checkout-report.md` | — | Git state, Go toolchain, directories, contributor docs, build/lint/test entry points, generated/mock/CE boundaries |
| `01-mental-model.md` | A | Why orchestrators exist; precise glossary with wrong mental models; server/client/scheduler/driver separation; Raft and leader; correctness tradeoffs |
| `02-architecture-map.md` | B | Subsystem table: responsibility, owned state, key symbols, communication mechanism, priority, paths and tests; why each "Read now" item is on the core path |
| `03-job-trace.md` | C | Thirteen-stage trace of a service job from CLI to running, through a client failure and replacement; three Mermaid diagrams; "may assume vs must tolerate" table |
| `04-design-notebook.md` | D | Sixteen design topics in dependency order, each with problem, invariant, abstraction, paths, failure scenario, wrong model, test, Socratic question |
| `05-apprenticeship-sessions.md` | E | Twelve 60–120 minute code-reading sessions with exact files, tests, exercises, checkpoints, stopping criteria |
| `06-contribution-path.md` | F | Local setup, public vs internal packages, newcomer contribution categories, five practice exercises, issue-report quality, pre-change questions, process and license facts |
| `07-first-session.md` | G | Recommended first 90 minutes, five invariants, three things to defer, first checkpoint question, checkout-specific uncertainties |

## How to use this over many sessions

1. Do `07-first-session.md` first. Send me your checkpoint answer before we go further.
2. Then follow `05-apprenticeship-sessions.md` one session at a time. Keep notes in `jerry-learn/notes/session-NN.md` (create the folder; nothing in the repo references it).
3. Use `03-job-trace.md` as the spine: whenever a session's code seems disconnected, find its stage number there.
4. Use `04-design-notebook.md` for the "why," and its Socratic questions as prompts for our tutoring sessions.
5. Use `02-architecture-map.md` as the index when you need a symbol or a test name.
6. When you are ready to write code, start with `06-contribution-path.md` F.3 and F.6.

## The running example

"A platform runs a fleet of agent-tool workers. Each job needs CPU, memory, maybe a GPU or a runtime. The control plane must choose an eligible machine, avoid overcommitting, start the work, observe health and completion, recover after a machine failure, and place replacement work safely."

Mapping used throughout (analogy only; Nomad has no agent reasoning, tool protocols, prompt memory, workflow durability, or application-level exactly-once):

| Fleet concept | Nomad concept |
|---|---|
| "I want N tool workers of this kind" | `Job` with a `TaskGroup{Count: N}` |
| One worker sandbox | `Allocation` (one instance of a task group on one node) |
| The sandbox runtime | Task driver (`raw_exec`, `docker`, ...) |
| "Something changed; recompute" | `Evaluation` |
| A proposed assignment | `Plan` |
| The machine inventory entry | `Node` |
| Worker went silent | Missed heartbeat → `down`/`disconnected` → `lost`/`unknown` allocs → replacement |

## Ground rules I followed

- I inspected the checkout before every claim; symbols and test names were confirmed with `grep`.
- I did not modify any repository file outside `jerry-learn/`, did not commit, and did not open issues or PRs.
- Where behavior depends on config, OS, build tags, or enterprise edition, the dependency is named (see `07-first-session.md`, "Checkout-specific uncertainties").
