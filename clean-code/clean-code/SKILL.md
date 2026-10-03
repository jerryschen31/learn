---
name: clean-code
description: Language-agnostic Clean Code principles (Robert C. Martin et al.) for writing, modifying, refactoring, and reviewing code. Use whenever producing or changing code in any language — naming, functions, comments, formatting, objects vs. data structures, design (DI, polymorphism, Law of Demeter), error-prone conditionals, tests, and code-smell detection. Applies to every coding task unless the repo's own conventions say otherwise.
---

# Clean Code

Code is clean when **anyone on the team — not just its author — can understand it easily**. Understandability is the root property; readability, changeability, extensibility, and maintainability all follow from it. Code is read far more often than it is written, so optimize for the next reader (human or agent).

> "Programs must be written for people to read, and only incidentally for machines to execute." — Abelson & Sussman

Apply these principles to every line you write or change. They are language-agnostic: translate each one into the idioms of the language at hand rather than importing patterns from another language.

## How to apply this skill

1. **Repo conventions win.** If the codebase, its linter/formatter config, or its CLAUDE.md/AGENTS.md prescribes a style, follow it. Clean Code is the default when the repo is silent, and the tiebreaker when it is ambiguous. Consistency with surrounding code beats any individual rule here.
2. **Scope your cleanup.** Apply these rules fully to code you write. For existing code, follow the Boy Scout Rule — improve what you touch, in proportion to the task — but do not launch unrequested sweeping refactors or reformat untouched files. Large cleanups belong in their own change.
3. **Rules serve understandability.** When two rules conflict, or a rule would make a specific piece of code harder to read, choose whatever makes the code easiest to understand and change, and be able to say why.
4. **Order of work:** *Make it work → make it right (clean) → make it fast.* Never leave code at "works" if it isn't also "right."

---

## 1. General rules

- **Follow standard conventions** — of the language, the framework, the domain, and the team. Idiomatic code is easier to read than clever code.
- **KISS — Keep It Simple.** Reduce complexity as much as possible. Prefer the obvious solution over the clever one. If a newcomer could not follow it, simplify it.
- **YAGNI — You Aren't Gonna Need It.** Build only what the current requirement needs. No speculative features, unused fields, hypothetical extension points, or "just in case" parameters. (YAGNI targets speculative *functionality*; it does not excuse making code harder to change.)
- **DRY — Don't Repeat Yourself.** Every piece of knowledge should have one authoritative representation. When code is identical or nearly so, extract it and let parameters carry the differences. Deduplicate *knowledge*, not coincidental similarity — two snippets that look alike but change for different reasons should stay separate.
- **Apply them in order: YAGNI → KISS → DRY.** Do only the necessary → do it the simple way → then remove duplication.
- **Delete code rather than add it.** Less code means fewer bugs and less to read. Remove dead code, unused parameters, obsolete branches, and toggles nobody uses. Sometimes a feature is best implemented by deleting code.
- **Boy Scout Rule.** Leave the code a little cleaner than you found it: rename an unclear variable, extract a muddled conditional, remove a dead import — within the area you are already changing.
- **Always find the root cause.** Fix the underlying problem, not the symptom. Don't patch over a bug with a special case, a retry, a sleep, or a swallowed exception without understanding why it happens.

## 2. Design rules

- **Keep configurable data at high levels.** Constants, defaults, and settings belong at the top level (config, entry point, module top), passed down — not buried in low-level functions.
- **Prevent over-configurability.** Don't expose knobs no one needs. Every option is a branch to test and a decision forced on the caller.
- **Prefer polymorphism to `if/else` or `switch` on type.** When the same type-switch appears in more than one place, replace it with polymorphic dispatch (interfaces, subclasses, strategy objects, tagged-union handlers, function tables — whatever is idiomatic). A single switch in a factory that creates the polymorphic objects is fine.
- **Use dependency injection.** Pass collaborators (clients, stores, clocks, loggers) in rather than constructing or globally fetching them inside business logic. This decouples components and makes them testable.
- **Follow the Law of Demeter.** A unit should talk only to its direct collaborators — not reach through them (`a.getB().getC().doThing()`). Ask a collaborator to do the work instead of navigating its internals.
- **Separate concurrency code.** Keep threading, async coordination, locking, and channel plumbing apart from business logic so each can be understood and tested on its own.
- **Single Responsibility Principle.** Each module, class, and function should have one reason to change. Split code into simple, well-defined, well-named units.
- **Design for change.** Code that resists change is the core failure mode (see *Code smells*). Favor small, decoupled units with clear boundaries.

## 3. Understandability

- **Be consistent.** If you do something one way, do all similar things the same way — naming, error handling, structure, patterns.
- **Use explanatory variables.** Break complex expressions into named intermediate values that say what each piece means.
- **Encapsulate boundary conditions.** Off-by-one adjustments, edge limits, and range checks are easy to get wrong; compute them in one named place (`last_index = length - 1`) rather than scattering `+1`/`-1` throughout.
- **Prefer dedicated value objects/types to primitives.** Use a `Money`, `EmailAddress`, `UserId`, `Duration`, or enum instead of raw strings, ints, and floats when the value has meaning, units, or invariants. This prevents mixing up arguments and centralizes validation.
- **Avoid hidden logical dependencies (temporal coupling).** A function should not silently rely on another function having been called first or on some mutable state being set up elsewhere. Make dependencies explicit through parameters, return values, or construction.
- **Express conditionals positively.** `if is_valid` reads better than `if !is_invalid`. Name booleans positively (`is_enabled`, not `is_not_disabled`) and avoid double negatives.
- **Encapsulate complex conditionals.** Extract compound boolean logic into a well-named function or variable that states the intent:
  ```
  // Before
  if (user.age > 18 && !user.hasChildren && (user.isPremium || user.hasCoupon)) ...
  // After
  if (isEligibleForOffer(user)) ...
  ```
- **Avoid deep nesting ("Hadouken IFs").** Use guard clauses and early returns to handle invalid/edge cases first, leaving the main path un-indented:
  ```
  // Before: if valid { if inStock { if paid { ok } else {...} } else {...} } else {...}
  // After
  if (!order.isValid)   return error("Order is invalid")
  if (!order.inStock)   return error("Item is out of stock")
  if (!order.isPaid)    return error("Payment failed")
  return process(order)
  ```
  Guard clauses checking for failure are the accepted exception to "prefer positive conditionals."

## 4. Names

- **Descriptive and unambiguous.** A name should reveal intent: why it exists, what it does, how it's used. If it needs a comment to explain it, rename it. `elapsedTimeInDays`, not `d`; `factorial(number)`, not `fact(x)`.
- **Make meaningful distinctions.** Don't differentiate names with noise (`data`/`info`, `a1`/`a2`, `ProductData`/`Product`). Different names must mean different things.
- **Pronounceable.** You should be able to say it in a conversation (`generationTimestamp`, not `genymdhms`).
- **Searchable.** Avoid single letters and bare numbers outside tiny scopes. Name length should scale with scope size; a loop index `i` is fine, a module-level `i` is not.
- **Replace magic numbers and strings with named constants.** `MAX_LOGIN_ATTEMPTS`, not `5`.
- **Avoid encodings.** No Hungarian notation, type prefixes/suffixes, or member prefixes (`strName`, `m_count`, `IShape` unless the language's convention requires it).
- **Use the domain's vocabulary.** Prefer terms from the problem domain and the team's ubiquitous language; use solution-domain terms (Queue, Visitor, Factory) for technical concepts. One word per concept — don't mix `fetch`, `retrieve`, and `get` for the same idea.
- **Parts of speech.** Classes/types are nouns; functions/methods are verbs or verb phrases; booleans read as predicates (`isReady`, `hasAccess`, `canRetry`).

## 5. Functions

- **Small.** Then smaller. A function should fit comfortably on a screen and be graspable at a glance.
- **Do one thing**, do it well, and do only that. If you can extract another function with a name that isn't a restatement of the original, it was doing more than one thing.
- **One level of abstraction per function.** Don't mix high-level orchestration with low-level details. A top-level function should read like a list of well-named steps:
  ```
  generateReport(data):
      processed = process(data)
      json      = toJson(processed)
      save(json)
      send(json)
  ```
- **Descriptive names.** Long and descriptive beats short and cryptic. Small, single-purpose functions are easy to name; struggling to name one is a sign it does too much.
- **Fewer arguments.** Zero is ideal, one or two is good, three needs justification, more should be grouped into a parameter object/struct with a meaningful name.
- **No side effects.** A function should do what its name says and nothing hidden — no surprise mutation of arguments, globals, or unrelated state. Separate commands (change state) from queries (return information).
- **No flag arguments.** A boolean parameter announces that the function does two things. Split it into separate, clearly named functions (`renderForPrint()` / `renderForScreen()`). If a variant truly must be a parameter, use a named enum/type rather than a bare `true`/`false`.
- **Prefer many small functions to passing code in to select behavior.** Don't pass selectors, mode strings, or callbacks whose only purpose is to choose a branch inside the function.
- **Handle errors cleanly.** Prefer the language's idiomatic error mechanism (exceptions, result types, error returns) over sentinel/magic return values. Don't swallow errors silently. Error handling is "one thing" — keep it separate from the happy-path logic where practical.

## 6. Comments

- **Explain yourself in code first.** Before writing a comment, try to make it unnecessary by renaming or extracting a function or variable.
  ```
  // Before
  // check if employee is eligible for full benefits
  if (employee.flags & HOURLY && employee.age > 65) ...
  // After
  if (employee.isEligibleForFullBenefits()) ...
  ```
- **Comment the *why*, not the *what*.** Good comments capture:
  - **Intent** — why this approach was chosen.
  - **Clarification** — the meaning of an obscure argument or return value you can't change (e.g., a third-party API).
  - **Warnings of consequences** — "not thread-safe," "takes ~10 minutes," "order matters because…".
  - Legal headers, public API documentation, and TODOs where the project's conventions call for them.
- **Don't be redundant.** A comment that restates the code adds reading cost and will rot.
- **Don't add noise.** No obvious comments (`// increment i`, `// constructor`), journal/changelog comments, or attribution banners — version control records that.
- **No closing-brace comments** (`} // end if`). If you need them, the block is too long.
- **Never leave commented-out code.** Delete it; version control remembers.
- **Keep comments accurate.** When you change code, update or remove the comments describing it. A wrong comment is worse than none.

## 7. Source code structure and formatting

- **Use the project's formatter/linter** if one exists; never fight it.
- **Separate concepts vertically.** Use blank lines between distinct ideas (functions, logical sections within a function).
- **Keep related code vertically dense.** Lines that belong together should sit together without gratuitous blank lines or comments breaking them up.
- **Declare variables close to their usage**, in the narrowest scope possible.
- **Dependent functions close together.** A caller should sit just above its callees.
- **Similar functions close together.** Group functions that perform related operations.
- **Order top-down (the "stepdown rule").** Higher-level functions first, then the details they call, so the file reads like a newspaper — headline first, details below — without jumping around.
- **Keep lines short.** Respect the project's limit; otherwise keep lines comfortably readable (~80–120 chars).
- **Don't use horizontal alignment** of assignments or declarations into columns; it emphasizes the wrong things and breaks on every edit.
- **Use whitespace to show relationships** — associate tightly related things, separate weakly related ones.
- **Don't break indentation.** Even one-line bodies get their own properly indented block when the language or style guide expects it; structure must be visible at a glance.

## 8. Objects and data structures

- **Choose deliberately between objects and data structures, and avoid hybrids.**
  - *Objects* hide their data behind behavior. Expose operations, not internals; don't add getters/setters for every field by reflex.
  - *Data structures* (records, structs, DTOs) expose data and have no meaningful behavior.
  - A *hybrid* — half object, half data bag with public fields plus business logic — gets the worst of both. Pick one.
- **Prefer simple data structures** when you are just moving data between layers or across boundaries; reach for objects when you need to protect invariants or vary behavior.
- **Hide internal structure.** Callers should not depend on how a type stores its data.
- **Small, and do one thing.** A class/module should have a single responsibility and a small number of instance variables. Many fields usually means it should be split.
- **High cohesion.** Most methods should use most of the instance variables; if subsets of methods use subsets of fields, there are hidden classes waiting to be extracted.
- **Base types know nothing about their derivatives.** A parent type must never reference or switch on its subtypes.
- **Prefer instance (non-static) methods** when behavior depends on an object's state or may need to vary polymorphically. Pure, stateless utilities may be static/free functions where that is idiomatic.

## 9. Tests

Test code deserves the same care as production code.

- **One concept per test.** Each test verifies a single behavior; prefer a single logical assertion (multiple asserts are fine when they jointly check one outcome). A failure should point at one cause.
- **Readable.** Clear names describing the behavior and expected result; arrange-act-assert (given-when-then) structure; no clever logic in tests.
- **Fast.** Slow tests don't get run.
- **Independent.** Tests must not depend on each other or on execution order; each sets up its own state.
- **Repeatable.** Same result in any environment, any time — no reliance on network, wall-clock time, randomness, or shared state unless controlled (inject clocks, seed RNGs, fake I/O).
- **Self-validating.** Pass or fail automatically; no manual inspection of output.
- **Write or update tests alongside the code** they cover; a bug fix should come with a test that fails without the fix.

## 10. Performance vs. clarity

- **Clarity first.** Write the code that best expresses intent. Don't trade readability for micro-optimizations in non-critical paths.
- **"Premature optimization is the root of all evil"** (Knuth). Optimize only when there is a measured or clearly known performance requirement, and only the hot path.
- When an optimization does make code less obvious, isolate it behind a well-named function and leave a comment explaining *why* it's necessary.

## 11. Code smells — signs the code is dirty

Watch for these in code you write and code you touch. Each signals a violation of the rules above.

| Smell | Symptom | Typical cause / remedy |
|---|---|---|
| **Rigidity** | A small change cascades into changes across many unrelated places. | Tight coupling; missing abstractions → apply SRP, DI, polymorphism. |
| **Fragility** | A change in one place breaks things elsewhere ("fix login, break registration"). | Hidden dependencies, shared mutable state, duplication → encapsulate, make dependencies explicit. |
| **Immobility** | Can't reuse a part elsewhere without dragging along its entanglements. | Mixed responsibilities, hard-wired dependencies → separate concerns, inject collaborators. |
| **Viscosity** | Doing it right is harder than hacking it in. | Poor structure, slow feedback → simplify, speed up tests/builds. |
| **Needless complexity** | Speculative generality, unnecessary layers, clever code. | YAGNI, KISS, delete code. |
| **Needless repetition** | Copy-paste code, parallel switch statements. | DRY, extract functions, polymorphism. |
| **Opacity** | Code is hard to understand; changes require lots of research. | Better names, smaller functions, explanatory variables, encapsulated conditionals. |

Also watch for: long functions, long parameter lists, flag arguments, deep nesting, magic numbers, dead or commented-out code, misleading names or comments, feature envy (a method more interested in another class's data than its own), train-wreck call chains, and primitive obsession.

---

## Pre-completion checklist

Before declaring a coding task done, review the code you wrote or changed:

- [ ] Follows the repo's existing conventions and is consistent with surrounding code.
- [ ] Every name reveals intent; no magic numbers/strings; no encodings or noise words.
- [ ] Each function is small, does one thing at one level of abstraction, has few arguments, no flag arguments, and no hidden side effects.
- [ ] No deep nesting — guard clauses/early returns used; complex conditionals extracted and named positively.
- [ ] No duplication of knowledge; nothing speculative (YAGNI); dead and commented-out code removed.
- [ ] Comments explain *why*, not *what*; no redundant, noisy, or stale comments.
- [ ] Dependencies are injected; no reaching through objects (Law of Demeter); concurrency kept separate from logic.
- [ ] Types are either objects that hide data or plain data structures — not hybrids; meaningful values use dedicated types.
- [ ] File reads top-down; related code is close; formatting matches the project's formatter.
- [ ] Errors are handled explicitly — not swallowed; root cause addressed, not symptoms.
- [ ] Tests cover the change and are readable, fast, independent, and repeatable.
- [ ] The code you touched is at least a little cleaner than you found it — without unrequested sweeping rewrites.
