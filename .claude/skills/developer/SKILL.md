---
name: developer
description: >
  Designs, implements, and reviews Go production code in the zetasql-wasm
  repository to minimize cognitive complexity. Covers separation of behavior
  and data (DTOs with public fields, no getters), deep modules with thin
  interfaces, information hiding, layering (boundary discipline, dependency
  direction, no leaks), pulling complexity downwards, defining errors out of
  existence, comments, naming, consistency, and writing obvious code. Use
  this skill whenever the user adds new types/APIs, refactors module
  boundaries, asks for design review, or says "design this", "refactor
  this", "review the API", "is this a good abstraction?", "split/merge this
  module", "this is getting complex", "add a new package/type/struct",
  "where should this code live?", "how should I structure X?". The TDD
  cycle belongs to the `tdd` skill; test-side conventions belong to the
  `tester` skill.
---

# Developer

Designs, implements, and reviews Go production code in zetasql-wasm. The goal is to minimize **cognitive complexity** — the load placed on someone reading, extending, or changing the code.

This skill covers two modes:

- **Author mode**: writing new production code or refactoring existing code (typically called from the `tdd` cycle's green/refactor phases).
- **Review mode**: reading existing production code against the principles and producing a structured report.

The same P1–P13 principle set governs both. Test conventions live in `tester`; the TDD cycle lives in `tdd`.

## What complexity actually is

Complexity is anything that makes a system hard to understand or modify. It shows up as three symptoms:

- **Change amplification**: a single conceptual change requires edits in many places.
- **Cognitive load**: a developer must hold many facts in mind to make a safe change.
- **Unknown unknowns**: it's not clear which lines must change for a given task — this is the worst kind because it can't be planned around.

Every principle below attacks one or more of these. When weighing two designs, prefer the one that reduces unknown unknowns first, then cognitive load, then change amplification.

## Principles

### P1: Separation of behavior and data

Data structures (DTOs) hold values. Behavior lives in functions or methods that operate on those values. A struct's fields are exposed as **public fields directly**, not behind getters or setters.

**Why**: A getter that just returns a private field is a public field with extra ceremony — no real encapsulation, just noise. Wrapping data in methods couples readers to the API surface (`obj.X()`) instead of the data shape (`obj.X`), which makes diffs noisier and full-struct comparisons less direct. Information hiding should hide *something that's likely to change*, not just the access path to a primitive.

**How to apply**:
- A new type that just carries data → flat struct with public fields. No constructor needed if zero-init is sensible; keep `NewX` for non-trivial defaults.
- A method that just reads or writes a single field is a smell. Replace with field access.
- Add a method only when there is genuine behavior: a computation, a transformation, or a state mutation that has its own invariant (e.g., `Reset`, `Clone`, `Validate`).
- Constructors and DTOs are different concerns. `NewX` is for "I need a sensible default"; the struct itself is for "this is the data shape."

**Examples in this repo**:
- `ParseResumeLocation { Input, BytePosition }` — public fields. The only method is `Reset()`, which carries an invariant (BytePosition becomes 0).
- `AnalyzerOptions { Language, ParseLocationRecordType }` — public fields. The only methods are `toProto()` (transformation) and `Clone()` (deep copy).
- `Statement { SQL, Root }` — public fields. No `SQL()` getter.

### P2: Modules should be deep

A module's value is **capability per unit of interface**. A deep module exposes a small, simple interface but provides a lot of useful behavior behind it. A shallow module forces callers to know about its internals to use it.

"Deep" is not "many lines of code" or "many layers stacked"; it is the *ratio* of capability to interface surface. Go's `io.Reader` is the canonical deep interface — one method (`Read([]byte) (int, error)`) unifies files, networks, decompression, and encryption.

**Why**: Shallow modules don't reduce complexity — they just shuffle it. Each shallow boundary is a place where unknown unknowns multiply.

**How to apply**:
- When defining a new package or type, ask: "What is the smallest contract I can give callers that still does the job?"
- Resist the urge to expose internal helpers, intermediate types, or implementation-specific options.
- A package whose public symbols all delegate one-line to internal symbols is shallow — collapse it.

**Examples in this repo**:
- `Analyzer.AnalyzeStatement(ctx, sql, cat, opts) → *AnalyzeOutput` — one signature; hides the WASM bridge protocol, proto wire format, and error parsing.
- `Parser.ParseStatement` — same shape.

**Anti-pattern**: pass-through methods that just forward to a field. If `A.X()` calls `a.b.X()` with no transformation, either the type system can do the work (embedding/interface) or the wrapper isn't earning its keep.

### P3: Information hiding ≠ getters

Hide what is *likely to change*. The internal representation of a private field that is read and written verbatim is **not** something likely to change in a way the caller cares about — exposing it as a public field is fine.

**What to actually hide**:
- Algorithm choices (e.g., the WASM bridge's exact memory protocol)
- Concurrency strategy (locks, channels, ordering)
- External dependencies (proto wire format, file layouts)
- State invariants enforced by methods (e.g., "if X is set, Y must be nil")

**What is not worth hiding**:
- A field that holds a primitive or a publicly-visible type (`string`, `int32`, `*Foo`) and has no invariant beyond "is the current value"
- The exact layout of a DTO

### P4: General-purpose modules are deeper

A general-purpose module covers more cases with less interface. Special-purpose modules tend to grow new APIs every time a slightly different need appears.

**How to apply**:
- When adding an API, sketch what 2–3 callers (current + plausible future) would need. If the API generalizes naturally to all of them with a single signature, ship that.
- If the only known caller is one specific use case, ship the special-purpose API but be ready to generalize later when the second caller appears.
- Don't speculate. "Maybe someone will need X" is YAGNI. Generalization happens *when the second use case actually shows up*.

### P5: Different layer, different abstraction

Each layer of code should expose a distinctly different abstraction from its neighbors. If two adjacent layers have the same shape, one of them is a pass-through.

**Smell**: a method that takes the same arguments, returns the same type, and just calls one thing in the layer below. Either inline it, or it should add real value (validation, transformation, batching).

(This is a tactical observation about adjacent layers; for the broader layering view see P13.)

### P6: Pull complexity downwards

When complexity is unavoidable, push it into the implementer rather than the caller. Many callers vs one implementer — make the implementer absorb the pain.

**How to apply**:
- A function that takes 8 optional parameters with intricate combinations forces every caller to learn them. Better: a function with 2 parameters and sensible defaults, plus an `Options` struct for the rare cases.
- A getter that returns an error for "not initialized" forces every caller to handle the error. Better: initialize at construction so the field is always valid.
- A method that requires careful pre-conditions ("call X then Y then Z") forces every caller to remember the order. Better: encode the protocol inside the type.

### P7: Define errors out of existence

The best error is the one that cannot occur. When designing an API, ask whether the error is a real condition the caller has to react to, or whether the API can be redefined so the error never arises.

**Examples**:
- `Reset()` doesn't return an error. It can't fail — assigning 0 to a field always works.
- `Statement.SQL` is just a string. There's no `GetSQL() (string, error)` — the value is always available.
- Lookup methods that return zero-value-for-not-found (`m[key]` style) often beat methods that return `(value, ok)` for callers that don't care about the distinction.

**When the error is real**, design the type signature to make it impossible to ignore (`(T, error)` rather than panicking, etc.).

### P8: Design it twice

Before committing to a design, sketch at least one alternative. Compare them on the principles above. Even a quick second sketch catches issues that the first idea hid.

**How to apply**:
- For non-trivial changes (new type, new package boundary, refactor of an existing API), draft two designs in conversation with the user before writing code.
- For trivial changes (one-line method, obvious DTO field), this is overkill. Use judgment.

### P9: Comments capture what code cannot

Code says *what*. Good comments say *why* and the *invariants*. They explain decisions that aren't obvious from the syntax — the choice between alternatives, the constraint that explains an odd-looking line, the reason a workaround exists.

**What to comment**:
- Invariants: "BytePosition is monotonically non-decreasing across `AnalyzeNextStatement` calls"
- Non-obvious choices: "Empty SupportedStatementKinds means *all* are supported (C++ side convention)"
- Why a "wrong-looking" thing is right: "We re-marshal the proto here because the wrapper consumes bytes, not the proto directly"

**What not to comment**:
- Restating what the code says (`// Reset BytePosition to 0`)
- Stale promises that the code no longer keeps
- Histories of past versions (use `git log` for that)

### P10: Names matter

A good name is **precise**, **specific**, and **consistent** with the rest of the codebase. A reader who sees the name in isolation should be able to predict what the thing does.

**Quick test**: can you describe the thing without using any synonym in its name? If you have to say "a Foo is a thing that Foos some Bar," the name has failed.

**Consistency rule**: if there's already a `NewX` in the codebase that takes the same shape of arguments, your new constructor should also be `NewX`, not `BuildX` or `MakeX`.

### P11: Code should be obvious

A reader should be able to follow the code without bouncing through three other files. Obscure code is the most expensive code to maintain — every reader pays for it.

**Indicators of obscurity**:
- A method whose effect depends on external state set in a different file
- A type whose meaning depends on a tagging convention not visible at the use site
- Naming that requires the reader to guess

**Cure**: rename, comment the invariant, or refactor so the dependency is visible at the call site.

### P12: Consistency reduces cognitive load

Following the existing patterns is, by itself, a complexity reducer. Familiar shapes let readers skim. Unfamiliar shapes force them to read carefully.

**How to apply**:
- Read at least one neighboring file before introducing a new pattern. If the new pattern is genuinely better, propose it as a separate refactor pass — don't smuggle it in.
- If the existing pattern is a violation of these principles, note it but don't propagate the violation. Surface it for a follow-up.

### P13: Layering — boundaries, dependency direction, leakage

A layer is a set of code that operates at one level of abstraction. Each layer:

- **Has a defined responsibility** — the concerns it owns
- **Knows about layers below it** — uses them
- **Does not know about layers above it** — no upward dependency
- **Hides the layer below** — upper layers do not directly touch lower-layer types

**Why**: Layering is how change gets contained. A change to a low-level layer shouldn't ripple upward; a change to a high-level layer shouldn't require touching low-level code. When dependencies go in both directions, every change becomes a system-wide change.

**Layers in this repo (rough)**:
- Lowest: `wasm/` — WASM bridge bytes/memory protocol
- Low: `wasm/generated/` — auto-generated proto types
- Mid: `ast/`, `resolved_ast/`, `types/`, `catalog/` — typed Go wrappers around the proto
- Highest: top-level `zetasql/` — `Parser`, `Analyzer`, the consumer-facing API

**How to apply**:
- When adding a new function, decide which layer it belongs to. If it operates on multiple layers' concepts, it might belong in a higher coordinator layer, or it might be a leak.
- If you find yourself importing a high-level package from a low-level package, stop. Invert the dependency or refactor.
- Cross-layer types: an API at layer N should accept and return types from layer N or higher. Returning a layer-(N−1) type leaks the lower layer.
- Boundary thickness: each layer's exposed API should be narrow. A package with 50 exported functions probably encompasses multiple concerns and should be split.

**Intentional leakages in this repo** (don't flag these as accidents):
- `LanguageOptions.ToProto() *generated.LanguageOptionsProto` — explicit serialization API, the proto type is the contract
- `JoinScanNode.JoinType() generated.ResolvedJoinScanEnums_JoinType` — wrapping every proto enum in a Go enum was judged not worth the cost

When you see a proto type in an upper-layer API, first decide: is this an intentional leak (serialization, enum reuse) or an accidental one (forgot to wrap)? Treat the two differently.

## Process (author mode)

### Step 1: Understand the change

Before designing:
- What is the user actually asking for? Restate it in one sentence.
- What pieces of the system does it touch? Which packages, types, methods?
- What constraints exist that aren't in the request? (existing API contracts, performance, callers)

### Step 2: Read what's there

Open the production files involved and at least one nearby existing test. Note:
- Public surface shapes (DTOs vs object-with-getters vs interface-driven)
- Naming conventions in this package
- Layer position of each touched file (P13)
- Any existing principles violations (note for later, don't propagate)

### Step 3: Sketch two designs (P8)

For non-trivial changes, draft two alternatives and compare on:
- Cognitive load on the caller (P2, P5, P11)
- Information hiding done in the right places (P3)
- Errors avoided where possible (P7)
- Generality vs specificity (P4)
- Layer placement and dependency direction (P13)

Pick the simpler one unless there's a strong reason to choose the more elaborate one. Document the rejected alternative briefly so the choice can be reconsidered later.

### Step 4: Implement

- Behavior and data separated (P1)
- Names are precise and consistent (P10, P12)
- Comments capture the *why* and invariants (P9)
- The change is obvious in isolation (P11)
- Layer boundaries respected (P13)

### Step 5: Re-read for cognitive load

After writing, read the diff as if you've never seen the file before. If anything trips you up, fix it now. Common offenders:
- A name that you understand only because you wrote it
- A "clever" line that needs a comment but doesn't have one
- A method that seems redundant given a public field nearby
- A type from one layer leaking into another's API

## Review mode

When the user says "review the design", "is this a good abstraction?", "review my refactor", etc., walk the changed code through P1–P13 and report findings.

### Review procedure

1. **Identify scope**: which files/types/packages? Use `git status` / `git log` if unspecified.
2. **Read fully**: load the touched files plus their callers (briefly).
3. **Check principle by principle**: walk P1–P13. Record every violation as a finding.
4. **Output**:

```
## Design review: <scope>

### Finding 1: P<n> <principle name>
- **Location**: foo.go:42
- **Issue**: <what specifically is wrong>
- **Fix**:
```go
// before
<violating code>

// after
<corrected code>
```
- **Why this principle matters**: <one-line rationale>

### Finding 2: ...
```

### Priority order when many violations stack up

1. **P1, P3** (DTO / no getters) — design-level, cascades into tests
2. **P13** (layering) — boundary issues are expensive to fix later
3. **P2, P5** (depth, layer abstraction) — structural
4. **P7** (errors out) — affects every caller
5. **P9, P10, P11, P12** (comments, names, obviousness, consistency) — readability
6. **P4, P6, P8** (generality, complexity-down, design-twice) — process-level

### Review etiquette

- Communicate the **why**, not blind directives. Each finding cites the principle and explains what cognitive cost it imposes.
- Show concrete fixes — no vague "please refactor."
- Acknowledge intentional trade-offs (e.g., the proto leaks under P13). If the reviewer is sure something is intentional but worth noting, say so.

## Failure modes

These are concrete bad outcomes to watch for, with the principle each violates.

| # | Mode | Example | Principle |
|---|---|---|---|
| D1 | Getter for a primitive field | `func (s *S) X() string { return s.x }` | P1, P3 |
| D2 | Setter for a primitive field | `func (s *S) SetX(v string) { s.x = v }` | P1, P3 |
| D3 | Pass-through method | `func (a *A) X() T { return a.b.X() }` | P5 |
| D4 | Shallow module | a package whose public API mirrors a single internal type one-to-one | P2 |
| D5 | Speculative generality | options/parameters that no current caller uses | P4 |
| D6 | Caller-burdening errors | `(T, error)` for an operation that cannot actually fail | P7 |
| D7 | Implicit pre-condition protocol | "call X before Y or it panics", with no API enforcement | P6 |
| D8 | What-comment instead of why-comment | `// add 1 to count` above `count++` | P9 |
| D9 | Inconsistent naming | `MakeFoo` next to `NewBar` for the same shape | P10, P12 |
| D10 | Obscure call dependency | code whose meaning depends on state set elsewhere with no comment | P11 |
| D11 | Leaky abstraction | high-level API returns a low-level type (e.g., a proto) without intent | P13 |
| D12 | Inverted dependency | low-layer package imports high-layer package | P13 |
| D13 | Layer ping-pong | a call repeatedly bounces between layers | P13 |
| D14 | Wide boundary | a layer/package exposes far more entry points than it owns concerns | P2, P13 |

If you spot any of these in code you're about to commit, fix them before commit.

## Anti-patterns

### AP1: "I'll add a getter for now, refactor later"
Getters accumulate. Once one exists, the next becomes natural. Once a few exist, the type *is* a getter-shell type and refactoring later means changing every caller. Pay the cost up front: public field, no getter.

### AP2: Wrapping every type in a constructor
Not every struct needs `NewX`. If `&X{Field: v}` is sensible at the call site and there are no defaults to apply, the constructor adds noise without value. Reserve `NewX` for cases where the construction has real logic (defaults, validation, allocation of internals).

### AP3: Mirror-of-the-proto types
A package whose types are 1:1 mirrors of generated proto types (same fields, same names) is a shallow module (P2). The mirror earns its keep when it adds real value: hiding the proto wire format, exposing typed accessors, enforcing invariants. If it just renames `ProtoFoo` to `Foo`, delete it and use the proto directly.

### AP4: Premature abstraction
Defining an interface for "future flexibility" that has only one implementation is speculative (P4). Wait until the second implementation exists, then extract the interface from the actual divergence.

### AP5: Reading the test instead of the type
Don't infer the spec of a type from its tests. The test exercises some cases; the type's contract is what the type should do for *all* inputs. Read the type's doc comments, the field types, and the constructor — these are the spec.

## How this skill interacts with others

- `tdd`: drives the cycle. When green-phase implementation needs design (new type, new method shape, new package boundary), it consults this skill. The cycle's "minimal implementation" rule still holds — but "minimal" is judged through P1–P13, not against an arbitrary baseline.
- `tester`: reviews tests against R1–R11. When a test review surfaces a production-code smell (a getter that should be removed, a test-only field, a leaky type), it refers to this skill for the production fix. When in doubt: tests are the consumer of the production design — if the test is awkward, the production design is probably awkward too.
