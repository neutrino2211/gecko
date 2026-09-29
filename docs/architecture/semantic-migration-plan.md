# Semantic validation migration

The compiler and language server share `frontend.Session` and `semantic.Program`. C lowering still discovers some Gecko language errors while producing C. Move those rules into semantic analysis so both clients receive the same result before lowering.

## Ownership and migration rule

| Owner | Responsibility |
| --- | --- |
| `parser` and `frontend` | Preserve source ranges, resolve and cache imports, and invalidate analyses when inputs change. |
| `semantic.Program` | Resolve declarations and calls, infer types, analyze flow, and retain validated facts and diagnostics. |
| `compiler` | Adapt semantic diagnostics to the existing CLI and editor format, then pass validated facts to lowering. |
| C backend | Choose C representations, emit code and runtime guards, and report C target or ABI limitations. |

Migrate one rule family at a time. First add the semantic facts and frontend-only tests. Compare CLI and LSP results, including source locations and warnings. Then remove the backend copy and verify generated C and runtime behavior. Passing compile tests alone is insufficient while the backend still reports the same error.

## 1. Diagnostic and resolution contract

Carry code, exact end offset, help, related locations, and suggested fixes from semantic analysis through cached views, compiler diagnostics, and LSP publication. Clone mutable diagnostic data when returning or copying a program. Record explicit outcomes for resolved, invalid, and incomplete expressions so a missing type does not hide why resolution failed. A resolved call should retain its selected symbol, receiver, substituted signature, and argument mapping. Candidate failures stay local until overload selection finishes.

**Done when:** cached and fresh results expose equal diagnostics; editing one view cannot change another; invalid calls do not leak errors from rejected overloads.

## 2. Declaration and relationship validation

Move duplicate and invalid implementations, coherence, trait inheritance, inherited overrides, required methods, hook conflicts, and visibility checks from `c_trait_declarations.go`, `c_implementations.go`, `c_generic_impl.go`, `c_calls.go`, and `c_chains.go` into semantic indexing and validation. Use symbol and module identities rather than names alone. Store the resolved trait and implementation graph for member lookup, constraints, editor queries, and lowering.

**Done when:** imported and local traits behave alike, including same-named types in different modules; the corresponding C backend diagnostics are removed.

## 3. Types and conversions

Consolidate type compatibility, numeric loss warnings, qualification changes, and casts into one semantic conversion decision. Apply it to initializers, assignments, struct fields, arguments, and returns. Give lowering the chosen conversion rather than asking it to infer the source type again. Include target-dependent type widths in the analysis input and cache key.

**Done when:** the same source and target types have the same validity and warning at every use site, and lowering follows the recorded conversion.

## 4. Calls and effects

Complete overload failure reporting and move remaining argument checks from `c_call_check.go`: generic constraints, `out` marker and target validity, repeated `out` targets, lossy conversions, and throwing-call requirements. Resolve a receiver and signature once. Validate after candidate selection so rejected overloads cannot publish diagnostics. Keep existing variadic and `out` behavior during migration.

**Done when:** calls through imports, methods, chains, generics, and overloads have one semantic result and no duplicate backend checks.

## 5. Intrinsics, unsafe, and hooks

Move intrinsic arity and operand checks, raw-access requirements, readonly stores, unsafe setup and handler requirements, operator-hook ambiguity, iterator requirements, and `try` operand rules into semantic validation. Record selected hook identities and unsafe setup facts. Lowering consumes these facts and emits runtime guards.

**Done when:** expression and statement forms share each rule and imported hooks resolve the same way in the compiler and editor.

## 6. Ownership and lifetime flow

Extend semantic flow facts from nullability to moves, borrows, reinitialization, local-address escapes, and cleanup obligations. Use symbol identities and explicit branch and loop merges. Move checks from `c_moves.go`, `c_borrows.go`, `c_literal_types.go`, and `c_return_check.go`. Preserve current language behavior; stronger ownership rules require separate design work.

**Done when:** branches, loops, shadowing, early returns, and deferred cleanup retain correct errors and generated cleanup.

## 7. Backend boundary and concurrency

Remove each migrated check after parity is proved. Require lowering to consume validated facts and report missing facts as compiler invariant failures. Keep C representation, foreign ABI support, and target capability checks at the backend boundary. Replace process-wide C lowering registries with per-compilation state. Remove the LSP backend mutex only after concurrent compilation and race tests pass.

**Done when:** ordinary editor diagnostics complete from frontend analysis, and concurrent backend checks have no shared mutable state.

## Required gate for each stage

1. Frontend-only tests demonstrate that the rule fires without C lowering.
2. Compiler and LSP tests agree on severity, code, range, help, related locations, and fixes where supplied.
3. Compile/run tests cover valid behavior and error cases; generated C is inspected.
4. Cached results and cloned editor/backend views retain the new facts without sharing mutable state.
5. The migrated backend check is removed and the full build, test, vet, and file-size checks pass.

## Current progress

Stage 1 now carries diagnostic metadata through cloned frontend views, the compiler adapter, and the LSP. Calls retain explicit resolved, invalid, or incomplete outcomes; resolved calls retain the selected symbol, receiver, substituted signature, and argument mapping. Expressions also record resolved, invalid, or incomplete outcomes. Overload candidates are analyzed in isolated probes, so rejected candidates cannot publish diagnostics, nested call facts, or occurrences. Cached and fresh call facts agree in frontend tests, and cloned views do not share mutable facts. Overload type and return mismatch reporting remains part of Stage 4, along with removing the corresponding backend call checks.

Stage 2 has begun with coherence validation. Semantic analysis now resolves the type and trait symbols for each implementation and reports foreign inherent implementations and foreign-trait/foreign-type pairs before lowering. The corresponding C backend checks have been removed. Duplicate implementations, trait inheritance and overrides, required methods, hook conflicts, visibility, and the resolved implementation graph remain in Stage 2.
