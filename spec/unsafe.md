# Unsafe Operations & Handlers

This document covers the handler-based safety model: how raw memory operations
are permitted (`@unsafe` regions) and how violations are handled by composable
handler instances. The API reference for the built-in handler types lives in
`stdlib/unsafe.gecko` (its `///` docs are generated into the API reference).

## Overview

Gecko separates two concerns:

1. **Permission** — which operations are raw/unsafe and where they may be used
   (the `@unsafe` region).
2. **Policy** — what happens when a guarded operation would violate a handler's
   precondition (`guard` / `catch`).

A fixed allowlist of unsafe intrinsics exists (`@write_volatile`,
`@read_volatile`, `@ptr_add`, `@ptr_sub`, `@copy`, `@zero`, `@trap`,
`@unreachable`). They may only be used inside an `@unsafe` region; elsewhere they
are a compile error. What happens on a violation is decided by handler instances,
not hardcoded.

## The `@unsafe` region

Marking a function or block `@unsafe` grants permission to call the unsafe
intrinsics. See [attributes.md](attributes.md) for the attribute itself; this
document focuses on the handler model layered on top of it.

## Handlers

A handler is an instance of a type that implements the `UnsafeHandler` trait:

```gecko
trait UnsafeHandler {
    func guard(self, op: UnsafeOp): bool
    func catch(self, op: UnsafeOp): void
}
```

- `guard` is called around every intrinsic the handler covers. Return `true` to
  permit the operation, `false` to reject it.
- `catch` runs when `guard` returns `false`. It MUST pick exactly one policy:
  - **recover** — record the failure and let execution continue, or
  - **trap** — abort the process (e.g. `@trap`).

`UnsafeOp` carries the pointer (`op.ptr`, `0` for control-flow intrinsics like
`@trap`) and the covered byte count (`op.size`) so `guard`/`catch` can inspect
the operation.

## Composing handlers

```gecko
@unsafe with ha & hb {
    @write_volatile(p, v)
}
```

Handlers are composed with `&`. The group passes only if **every** handler's
`guard` passes; otherwise the intrinsic is **skipped** and each failing
handler's `catch` runs.

## Attaching handlers globally

`@attach_handler("write_volatile", "read_volatile", ...)` on an
`impl UnsafeHandler for X` makes that handler apply to those intrinsics whenever
they appear inside any `@unsafe` region — no explicit `with` clause needed. This
is how the built-in `NonNull`, `Bounds`, and `Assume` handlers are wired up.

## Trap vs recover

A handler's `catch` decides the failure policy:

- **Recover** (record + continue): the surrounding `@unsafe with ...` expression
  yields `Err` and execution continues normally. Use this when you want to inspect
  the failure via a `Result`.
- **Trap** (e.g. `@trap`): a **hard abort** — the process stops and no `Result`
  is produced. A trapping handler therefore cannot be combined with the `Result`
  forms below; it is meant as a fatal "this must never happen" assertion.

The built-in `NonNull`, `Bounds`, and `Assume` handlers trap. A custom handler
can recover instead.

## Getting a `Result`

An `@unsafe with ...` block can yield a `Result<T, E>` instead of merely running
for side effects:

```gecko
let r = @unsafe with NonNull::handler(p as void*) {
    @write_volatile(p, v)
    99
}
if (r.is_error()) {
    // guarded operation was skipped / handler rejected
} else {
    let v = r.unwrap()
}
```

- `let r = @unsafe with ...` declares `r` as the `Result<T, E>` (T is the block's
  trailing expression, E is a synthesized error enum whose variants are the
  handler ids).
- `r = @unsafe with ...` reassigns an existing `Result` variable, reusing its
  error-enum type so the new value is type-compatible.
- A bare `@unsafe with H { }` with no binder simply runs the guarded body for its
  side effects (e.g. a `catch` that traps) and yields no `Result`.

On success the `Result` holds `Ok(trailing)`; on the first failing guard it holds
`Err(handler variant)`. The error enum is synthesized per block and named after
the handler set, so each distinct handler composition gets its own `Err` type.

## Placement

Implementation: `tokens/tokens.go` (`UnsafeBlock`), `semantic/analyzer.go`
(`analyzeUnsafeBlock`), `backends/c_backend/c_unsafe.go` (`NewUnsafeBlock`).
