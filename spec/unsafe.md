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

A fixed allowlist of unsafe intrinsics exists (`@alloc`, `@free`, `@deref`, `@drop_in_place`, `@write_volatile`,
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
  - **recover** — record the failure and continue after the guarded block, or
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
`guard` passes; otherwise each failing handler's `catch` runs and execution leaves
the block. The rejected operation and the remaining body are skipped. Registered
defers and automatic drops run before leaving, unless a catch traps.

## Declaring handler coverage

`@attach_handler("write_volatile", "read_volatile", ...)` on an
`impl UnsafeHandler for X` declares which intrinsics instances of that type cover.
It does not construct or activate a handler. Activation is explicit through
`with`, including the arguments supplied to the handler constructor. Importing
`NonNull`, `Bounds`, or `Assume` does not automatically enable checks.

## Trap vs recover

A handler's `catch` decides the failure policy:

- **Recover** (record + continue): the surrounding `@unsafe with ...` expression
  yields `Err` and execution continues normally. Use this when you want to inspect
  the failure via a `Result`.
- **Trap** (e.g. `@trap`): a **hard abort** — the process stops and no `Result`
  is produced on failure. A trapping handler may be used with the `Result`
  forms below, but a violation aborts instead of returning `Err`.

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
`Err(handler variant)`. The compiler synthesizes the error enum for a new result
binding; reassignment reuses the existing result's error type.

## Placement

Implementation: `tokens/control_flow.go` (`UnsafeBlock`), `semantic/inference.go`
(`analyzeUnsafeBlock`), `backends/c_backend/c_unsafe.go` (`NewUnsafeBlock`).

## Setup bindings

The bracketed form declares values before constructing handlers that need them:

```gecko
let allocation = @unsafe [
    size = @size_of<int32>(),
    ptr: int32* = @alloc(size, @align_of<int32>()) as int32*
] with NonNull::handle(ptr as void*) & Bounds::handle(ptr as void*, size) {
    @deref(ptr) = 42
    ptr
}
```

This is a declaration list, not a closure capture. Each binding requires an
initializer; its type may be explicit or inferred. Bindings are visible to later
initializers, handler arguments, and the body, and remain local to this block.
Duplicate names in one setup list are errors. Forward references are not allowed.

Execution has four stages:

1. Evaluate setup initializers exactly once, from left to right.
2. Construct the handlers from left to right, using the initialized values.
3. Present each setup allocation's returned pointer and requested byte count to
   the handlers covering `alloc`, in allocation order.
4. Run the body, checking covered raw intrinsics before their effects. Evaluate
   the trailing value only if execution reaches it.

Allocation must run before its returned pointer can be checked. Other raw
intrinsics cannot appear directly in setup initializers. A setup `@alloc` must
be the entire initializer, optionally parenthesized or cast; nested allocations
in conditional expressions or arithmetic are rejected. Calls from setup obey
the normal call rules; handlers are not propagated into called functions.

When a guard rejects an operation and `catch` returns, the setup form exits the
block, skips the operation and remaining body, and produces `Err` when bound to
a result. Deferred expressions already registered in the block run on this exit.
A failing allocation check skips the body entirely. A trapping `catch` aborts.
The same block-exit behavior applies with or without setup bindings.

`let r = @unsafe [...] with ... { value }` requires at least one handler and a
trailing value. Reassignment uses the same syntax. A bare `@unsafe [...] { ... }`
may omit handlers and has no result. Checks are opt-in and governed by each
handler's coverage; the setup syntax alone does not validate memory accesses.

### Raw allocation and access

- `@alloc(size, align)` returns a nullable `void*`, not a `Result`. The C backend
  uses the hosted C11 `aligned_alloc` runtime, rounds the size up to the requested
  alignment, and raises small alignments to the platform's fundamental alignment.
  Zero size requests reserve at least one alignment unit. Zero or non-power-of-two
  alignment, rounding overflow, and allocator failure return null.
- `@free(ptr)` returns no value and releases memory allocated by `@alloc`.
- `@deref(ptr)` and `@deref(ptr) = value` are raw reads/stores and require an unsafe region.
- `@drop_in_place(ptr)` runs the pointee Drop hook without releasing storage.
  In a setup block, both `@deref` reads and stores are checked by handlers covering
  `deref`, with the pointee's byte size in `UnsafeOp`.

There is no implicit initialization or freeing of raw allocations. The caller
must explicitly transfer or release them, including allocations that succeeded
before a later setup check failed. The setup form does not introduce ownership
or borrow types. All raw reads outside this form also require unsafe permission.

`NonNull::handle(ptr)` and `Bounds::handle(ptr, size)` are parameterized
constructors; their existing `handler(...)` spellings remain available.

### Cleanup across multiple allocations

Raw setup allocations are not automatically owned. To give successful resources
cleanup before a later fallible allocation, use nested blocks and register the
release before entering the next block, or first adopt each allocation into an
owning library type. A `defer` in the body cannot clean up an allocation rejected
before that body starts. A recovering allocation handler can explicitly release
resources it owns in `catch`. This keeps raw allocation/free separate from owning
types and their Drop hooks.

Handlers cover intrinsics in their lexical block, not operations inside called
functions or lambdas. Pointer field/index syntax still requires unsafe permission,
but handler coverage is attached to named intrinsics; use `@deref` when a handler
must inspect that read or write.
