# LLVM `llir` Gap Register

This file tracks places where Gecko backend work needs functionality that is not directly available as a high-level `llir/llvm` convenience API.  
Policy: keep emitting valid pure llir IR (no ad-hoc text IR templates), and document the workaround here.

## Gap 1: No high-level enum modeling helper

- `llir` exposes low-level integer/aggregate types, but no dedicated enum abstraction with case registration.
Gecko workaround:
- Represent enums as integer-backed values.
- Register enum case symbols in Gecko AST/value maps and lower case access (`Enum.Case`) into integer constants.
- Perform all comparisons and casts using integer instructions.
- Status: implemented workaround (LLVM emitter now lowers enums with integer-backed case symbols).

## Gap 1b: No object/member-chain lowering helper

- `llir` does not provide language-level chain lowering (`obj.field.method(args)`), only primitives (GEP/load/call).
Gecko workaround:
- Lower field hops through explicit pointer/GEP steps.
- Materialize terminal method invocation by emitting direct LLVM calls with explicit `self` receiver argument.
- Keep deterministic diagnostics for unsupported deep module symbol chains.
- Status: implemented workaround for field hops + terminal method invocation.

## Gap 2: No trait/impl dispatch abstraction

- `llir` does not provide trait/vtable/object-model abstractions; only functions, pointers, aggregates, and calls.
Gecko workaround:
- Use explicit mangled function symbols.
- Pass `self` as explicit pointer argument.
- Build dispatch tables/maps in Gecko lowering when dynamic dispatch is needed.
- Status: in progress (non-trivial trait/impl parity work is backend-owned).

## Gap 3: No monomorphization support

- `llir` is an IR builder and does not manage generic instantiation or symbol specialization.
Gecko workaround:
- Perform monomorphization in Gecko lowering.
- Emit concrete specialized functions/types and call those symbols directly.
- Status: open (LLVM backend generic parity milestone).

## Gap 4: No language-level intrinsic facade

- `llir` exposes instruction building, but language intrinsics (`@deref`, volatile helpers, runtime hooks) require Gecko-side lowering contracts.
Gecko workaround:
- Lower intrinsics into primitive load/store/call/gep instructions.
- Declare runtime helpers explicitly where needed and call them from generated IR.
- Status: in progress (intrinsic facade breadth and volatile parity still open).
