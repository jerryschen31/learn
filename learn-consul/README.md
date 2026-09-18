# jerry-learn — Consul source apprenticeship (study map)

Purpose: a bounded, source-grounded map for learning HashiCorp Consul's Go implementation from first principles. It is meant to be used interactively, one session at a time. It is **not** a tutorial document and it does **not** modify the repository.

Running example used everywhere: *a fleet of agent-tool workers starts and stops on unreliable machines; each worker advertises capabilities (service tags/meta) and health; a coordinator finds healthy workers for a capability, avoids unhealthy ones, and observes fleet changes.* This is an explanatory analogy only. Consul does not do agent orchestration, scheduling, workflow durability, or application-level authorization.

| File | Content |
|---|---|
| `00-checkout-facts.md` | Commit, Go version, layout, docs present/absent, build/test commands, generated/enterprise/compat conventions |
| `01-first-principles.md` | Part A: concepts, state categories, server vs client, consensus vs gossip, tradeoffs, glossary |
| `02-source-map.md` | Part B: subsystem table with paths, types, state, mechanism, priority, tests |
| `03-lifecycle-trace.md` | Part C: end-to-end registration → query → health transition, with Mermaid diagrams and assume/tolerate table |
| `04-design-notebook.md` | Part D: 12 design topics, each with problem, invariant, mechanism, code, failure, wrong model, check question, analogy |
| `05-apprenticeship.md` | Part E: 12 reading sessions (60–120 min each) |
| `06-contribution-path.md` | Part F: setup, test hierarchy, newcomer categories, five exercises, bug-report template, pre-change questions, process |
| `07-next-action.md` | Part G: first 90 minutes, five invariants, three things not to learn yet, first checkpoint, checkout uncertainties |

Conventions in these notes:

- **[code]** = observed directly in this checkout (path and symbol cited). **[inference]** = my explanation or a library-level behaviour not re-verified in this repo. **[library]** = behaviour that lives in a dependency (`hashicorp/raft`, `serf`, `memberlist`, `go-memdb`), not in this repo.
- Line numbers are from commit `b0296e557a` and will drift; symbols are the stable handle.
- Labels: *generated*, *test helper*, *compat*, *enterprise boundary*, *defer*.
