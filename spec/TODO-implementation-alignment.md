# Spec Alignment TODO

This document tracks implementation work needed to align compiler/LSP behavior with the language spec in `spec/`.

## 1) Coherence Enforcement (Traits/Inherent Impl)

Spec reference: `spec/traits.md` (Coherence Rules + Coherence Diagnostics)

- [x] Enforce local-only inherent impls (C backend): reject `impl Type { ... }` when `Type` is defined in another package
- [x] Enforce orphan rule for trait impls (C backend): allow `impl Trait for Type` only if current package defines `Trait` or `Type`
- [x] Reject foreign-foreign trait impls with explicit diagnostics (C backend)
- [x] Keep duplicate-method checks in inherent impl blocks (already present) as-is
- [x] Mirror coherence checks in LLVM backend implementation path (code added; execution currently blocked by existing LLVM feature gate that rejects `impl`)

Primary code paths:

- `backends/c_backend/c_backend.go`
  - `NewImplementation(...)`
  - `CImplementationForClass(...)`
  - `CInherentImplementation(...)`
- Resolution metadata:
  - `ast/ast.go` (`OriginModule`, package checks)
  - `ast/method.go` (package visibility helpers)

## 2) Diagnostic Wording Parity

Spec reference: `spec/traits.md` (Coherence Diagnostics)

- [x] Add compiler diagnostics matching spec templates (C backend)
- [x] Add actionable `help:` text for each coherence error (C backend)
- [x] Ensure errors include file/line and both trait/type names when relevant (C backend)
- [x] Ensure LLVM backend emits equivalent diagnostics where `impl` processing is reachable (currently gated by LLVM unsupported-feature check)

## 3) Test Coverage

Add compile tests under `test_sources/compile_tests/coherence/`:

- [x] `inherent_foreign_type_error.gecko`
- [x] `trait_impl_foreign_foreign_error.gecko`
- [x] `trait_impl_local_trait_foreign_type_ok.gecko`
- [x] `trait_impl_foreign_trait_local_type_ok.gecko`

And test assertions in:

- [x] `tests/compiler_test.go` (expected error substrings + runtime success cases)

## 4) LSP Behavior

- [ ] Ensure method completions/navigation do not assume illegal foreign inherent impls can exist
- [ ] Validate diagnostics surface coherence violations in editor context
- [ ] Add/update LSP tests as needed (`lsp/` tests)

## 5) Legacy Syntax Cleanup Tracking

Spec currently documents `default impl` as legacy/deprecated compatibility.

- [ ] Decide sunset plan for `default impl` compatibility
- [ ] Add warning path (if not already present)
- [ ] Update tests/docs when removal date is chosen

## 6) Spec-Tagging Run (Code <-> Spec Traceability)

Goal: each source file begins with a lightweight comment linking relevant spec docs.

Suggested tag format:

```go
// spec: spec/traits.md, spec/modules.md, spec/scoping.md
```

```gecko
// spec: spec/traits.md, spec/operators.md
```

Status:

- [x] Mapping file added: `spec/file-spec-map.json`
- [x] Script added: `scripts/spec-tags` with:
   - `audit` mode: report missing/extra tags
   - `apply` mode: insert/update top-of-file spec tags
   - `check` mode: CI-friendly nonzero exit when coverage drops
- [x] Audit/apply/check run completed for tracked `.go` and `.gecko` files
- [ ] Add CI check to keep tags current for changed files.

Initial high-value mapping targets:

- `tokens/`, `parser/` -> `types.md`, `functions.md`, `classes.md`, `traits.md`, `attributes.md`
- `compiler/`, `ast/`, `analysis/` -> `scoping.md`, `modules.md`, `traits.md`, `types.md`
- `backends/` -> `operators.md`, `control-flow.md`, `memory.md`, `c-interop.md`, `attributes.md`
- `lsp/` -> `scoping.md`, `modules.md`, `traits.md`, `visibility` sections
- `stdlib/` -> `stdlib.md`, `traits.md`, `operators.md`, `memory.md`

## Definition of Done

- Coherence behavior matches `spec/traits.md` examples and diagnostics.
- Tests cover all legal/illegal coherence combinations.
- LSP reflects same constraints.
- Spec-tag audit passes for all tracked source files.
