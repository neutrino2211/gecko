---
title: std.memory.borrow_cell
description: Runtime checked immutable and mutable views
---

```gecko
import std.core.traits use { Borrow, BorrowMut }
import std.memory.borrow_cell use { BorrowCell }
import std.memory.ref use { Ref }
import std.memory.ref_mut use { RefMut }

let cell: BorrowCell<int32> = BorrowCell<int32>::new(42)
let read: Ref<int32> = @borrow(cell)
let value: int32 = read.get()
read.drop()
let write: RefMut<int32> = @borrow_mut(cell)
write.set(43)
write.drop()
```

These types currently require `T is Copy`. A cell allows multiple shared readers
or one exclusive writer; conflicting acquisitions trap. `can_borrow()` and
`can_borrow_mut()` inspect availability. The direct methods `borrow()` and
`borrow_mut()` expose the same operations as the hooks.

| Type | Operations |
|---|---|
| `BorrowCell<T>` | `new`, `borrow`, `borrow_mut`, availability checks, `drop` |
| `Ref<T>` | `get`, `clone`, `drop` |
| `RefMut<T>` | `get`, `set`, `drop` |

Views retain the allocation even after the cell handle is dropped. Drop hooks
release reservations at scope exit; `--no-auto-drop` requires explicit cleanup.
Raw `from_storage` constructors require unsafe permission. Counts are non-atomic.

The compiler does not perform static lifetime checking. Ownership transfer through
aggregate fields, closures, and by-value arguments remains incomplete; keep views
in named local bindings or return them directly, and do not duplicate their raw
representation. These runtime checks are not a claim that arbitrary code is
memory-safe.
