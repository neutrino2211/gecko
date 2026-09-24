# Memory Model

Gecko exposes raw memory operations and ordinary library types for ownership.
Compiler hooks connect those types to explicit borrow operations and optional
scope cleanup. There are no lifetime annotations or garbage collector.

## Raw memory

`T*` is nullable. `T*!` is non-nullable; `T readonly*` permits reads but not
writes. Taking an address and comparing pointers with `null` do not read memory.
All raw reads and writes require an `@unsafe` block or function, including
`@deref`, pointer indexing, pointer fields, and methods called through raw pointers.
The compiler-managed `self` receiver is exempt from that last requirement.

```gecko
let value: int32 = 42
let ptr: int32* = &value
@unsafe {
    let copy: int32 = @deref(ptr)
    @deref(ptr) = copy + 1
}
```

A non-null check does not grant unsafe permission. It also does not prove that
memory is live, aligned, initialized, or large enough.

| Operation | Return | Unsafe |
|---|---|---|
| `@alloc(size, align)` | Nullable `void*` | Yes |
| `@free(ptr)` | No value | Yes |
| `@deref(ptr)` | Pointee value | Yes |
| `@deref(ptr) = value` | Raw store | Yes |
| `@drop_in_place(ptr)` | No value | Yes |
| `@read_volatile`, `@write_volatile` | Read value / store | Yes |
| `@ptr_add`, `@ptr_sub`, `@copy`, `@zero` | Raw operation | Yes |
| `@size_of<T>()`, `@align_of<T>()` | `uint64` | No |
| `@is_null(ptr)`, `@is_not_null(ptr)` | `bool` | No |
| `@move(value)` | Transferred local value | No |
| `@borrow(owner)`, `@borrow_mut(owner)` | Hook-defined view | No |

`@drop_in_place` invokes the pointee type's Drop hook without freeing its storage.
For a type without a Drop hook it only evaluates the pointer argument. The caller
must ensure the pointee is initialized and will not be dropped again.

`@alloc` uses hosted C11 aligned allocation. Invalid alignment, size-rounding
overflow, and allocation failure produce null. It does not return a `Result`.
`@free` releases storage; it does not run a destructor. See [unsafe.md](unsafe.md)
for setup bindings and parameterized handlers.

## Ownership and cleanup

A trait annotated `@drop_hook(.method)` opts its implementations into automatic
cleanup. Local values are cleaned up in reverse declaration order at scope exit,
including early return and loop `break`/`continue`. Deferred expressions run before
automatic drops in each scope, in reverse registration order.

`@move(local)` explicitly transfers a named local value and disables its cleanup,
including when passing it to an owning constructor. The destination must take
responsibility for destruction; discarding the result can leak.

```gecko
let value: Resource = make_resource()
let owner: Box<Resource> = Box<Resource>::new(@move(value))
```

Direct local binding transfers, including parenthesized bindings, disable cleanup
of the source. Explicit `.drop()` disables its later automatic cleanup. Replacing
an owned local evaluates the replacement, drops the previous value, then installs
the replacement. Runtime ownership flags distinguish paths through branches.
Passing a local with a Drop hook by value also transfers it. A by-value parameter
with a Drop hook is cleaned up on exit unless its body transfers it onward. A
parameter without a Drop hook retains ordinary value-copy behavior. The generated
C makes each ownership flag, move, and Drop call visible.
`try owner` and `owner or fallback` likewise transfer a named Option or Result
with a Drop hook into the operator's temporary. The `or` failure path drops an
unused Err payload before evaluating the fallback. A named owning fallback
transfers only when that branch runs; its runtime flag keeps it available for
cleanup when the left side succeeds. The compiler conservatively rejects using
that fallback binding again after the `or` expression. When `try` propagates an
error, it runs the same pending defer and Drop cleanup as an explicit return.

`gecko compile`, `build`, and `run` accept `--no-auto-drop`. This suppresses
compiler-inserted Drop calls, including replacement cleanup. Explicit drops,
`@drop_in_place`, and `defer` still run. Move diagnostics and raw unsafe requirements
remain enabled. No flag inserts implicit clones or enables unsafe operations.
`--print-expanded` prints the generated C, including ownership flags and inserted
Drop calls. This is the actual backend expansion; it is not Gecko source that can
be compiled again. `--ir-only` writes that C to a `.gecko.c` artifact for inspection.

## Standard ownership types

- `Box<T>` stores one value. Replacing or releasing it drops the contained value
  before releasing storage. `into_inner()` moves out the value and frees only its
  storage. `into_raw()` transfers storage responsibility.
- `Rc<T>` counts strong handles. `clone()` increments the strong count. The final
  strong release drops the contained value exactly once.
- `Weak<T>` retains the control block without keeping the contained value alive.
  The control block is released after the final strong and weak references end.
  `weak_count()` includes one implicit weak reference while strong owners exist.
  `upgrade()` returns an owning `Option<Rc<T>>` when the value is still alive.
- Raw adoption methods (`Box::from_raw`, `Box::from_raw_checked`,
  `Rc::from_inner_checked`, `Weak::from_rc_ptr`) are unsafe. Null/count checks do
  not establish pointer validity, allocation provenance, or ownership. Rebuilding
  an Rc from a live control block increments its strong count.
- `Box.get()`, `Rc.get()`, and `Weak.try_get()` are unsafe raw value copies.
  Copying a payload with a Drop hook can duplicate ownership. `Box.into_inner()`
  is the safe owned extraction path for a unique owner.
- `Option<T>` drops only an active Some payload. `unwrap()` clears the active tag
  and returns that payload. `Result<T, E>` tracks whether a payload is active and
  which variant owns it; its Drop implementation visits only that variant.

These reference counts are single-threaded and non-atomic.

## Typed borrows

`BorrowCell<T>`, `Ref<T>`, and `RefMut<T>` currently require `T is Copy`.
This permits `get()` to return a value without duplicating an owned resource.

```gecko
import std.core.traits use { Borrow, BorrowMut }
import std.memory.borrow_cell use { BorrowCell }
import std.memory.ref use { Ref }
import std.memory.ref_mut use { RefMut }

let cell: BorrowCell<int32> = BorrowCell<int32>::new(42)
let reader: Ref<int32> = @borrow(cell)
let second: Ref<int32> = reader.clone()
second.drop()
reader.drop()
let writer: RefMut<int32> = @borrow_mut(cell)
writer.set(43)
writer.drop()
```

`@borrow` dispatches the visible `@borrow_hook`; `@borrow_mut` dispatches
`@borrow_mut_hook`. Both require a named owner binding. The methods are also
callable directly: `cell.borrow()` and `cell.borrow_mut()`.

The stdlib cell enforces either multiple readers or one writer at runtime.
`can_borrow()` and `can_borrow_mut()` inspect availability; conflicting acquisitions
trap. Ref provides `get()` and `clone()`; RefMut provides `get()` and `set()` and
has no clone. Dropping a view releases its access reservation. Views retain the
allocation, so returning a view from a function with a local cell is supported.
Raw view construction requires unsafe permission and valid retained storage.

These are library-managed runtime borrows. Compiler hooks do not prove arbitrary
user-written borrow implementations sound and do not implement a static lifetime
or alias checker.

## Current limits

Ownership analysis is incomplete for general aggregate fields, temporary values,
and closure captures. Direct by-value calls transfer locals that have Drop hooks,
but nested expressions and ordinary structs without Drop hooks still use value
copies. Use `@move(local)` when explicitly handing a named resource to an owning
API. `Option<T>` and `Result<T, E>` manage their own active payloads in visible
stdlib code; arbitrary user aggregates still need their own Drop implementation.
Raw pointers, FFI, and unchecked collection operations remain programmer-managed.
`--no-auto-drop` requires explicit release of every owning handle and borrow view.
