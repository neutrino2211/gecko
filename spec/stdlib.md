# Standard Library

The Gecko standard library is not privileged - it's regular Gecko code that happens to ship with the compiler.

## Design Principles

1. **No prelude** - Nothing is auto-imported
2. **Explicit hooks** - Compiler features wired via trait hook attributes
3. **Layered** - Core traits have no dependencies, collections build on core
4. **Opt-out friendly** - Kernel/embedded code can ignore stdlib entirely

## Directory Structure

```
stdlib/
├── core/
│   ├── traits.gecko      # Lifecycle traits (Drop, Clone, Copy)
│   └── ops.gecko         # Operator traits (Add, Sub, Eq, etc.)
├── collections/
│   ├── vec.gecko         # Vec<T>
│   ├── slice.gecko       # Slice<T> borrowed views
│   └── string.gecko      # String type
├── memory/
│   ├── box.gecko         # Box<T> - unique ownership
│   ├── rc.gecko          # Rc<T> - reference counting
│   ├── weak.gecko        # Weak<T> - weak references
│   └── raw.gecko         # Raw pointer utilities
├── option.gecko          # Option<T>
└── result.gecko          # Result<T, E>
```

## Module Descriptions

### std.core.traits

Lifecycle traits with compiler hooks:

```gecko
@drop_hook(.drop)
public trait Drop {
    func drop(self): void
}

public trait Clone {
    func clone(self): Self
}

public trait Copy {
}

public trait Tryable<T> {
    func has_value(self): bool
    func try_unwrap(self): T
}

public trait Orable<T> {
    func unwrap_or(self, default_val: T): T
}

public trait Default {
    func default_val(): Self
}
```

`Drop`, `Borrow`, and `BorrowMut` hook integration is implemented.
`Clone` is called explicitly. `Copy` marks values eligible for implicit bitwise duplication.

**Dependencies:** None (freestanding-compatible)

### std.core.ops

Operator traits with compiler hooks:

```gecko
@add_hook(.add)
public trait Add<T> { func add(self, other: T): T }

@sub_hook(.sub)
public trait Sub<T> { func sub(self, other: T): T }

@eq_hook(.eq)
public trait Eq<T> { func eq(self, other: T): bool }

// ... etc
```

**Dependencies:** None (freestanding-compatible)

### std.option

Optional value type:

```gecko
public class Option<T> {
    let has_value: bool
    let value: T
}

impl Option<T> {
    public func some(val: T): Option<T>
    public func none(): Option<T>
    public func is_some(self): bool
    public func is_none(self): bool
    public func unwrap(self): T
    public func unwrap_or(self, default: T): T
    public func get_or(self, default: T): T
}
```

`Option<T>` implements Drop for its active Some payload. `unwrap()` transfers
that payload and marks the Option empty.

**Dependencies:** None (freestanding-compatible)

### std.result

Error handling type:

```gecko
public class Result<T, E> {
    let value: T
    let error: E
    let is_ok: bool
    let active: bool
}

impl Result<T, E> {
    public func ok(val: T): Result<T, E>
    public func err(e: E): Result<T, E>
    public func is_success(self): bool
    public func is_error(self): bool
    public func unwrap(self): T
    public func unwrap_err(self): E
    public func unwrap_or(self, default: T): T
}
```

`Result<T, E>` implements Drop for its active Ok or Err payload. `unwrap()` and
`unwrap_err()` transfer that payload and clear `active`.

**Dependencies:** None (freestanding-compatible)

### std.collections.vec

Dynamic array:

```gecko
public class Vec<T> {
    let data: uint64
    let len: uint64
    let cap: uint64
}

impl Vec<T> {
    public func new(): Vec<T>
    public func with_capacity(cap: uint64): Vec<T>
    public func push(self, value: T): void
    public func pop(self): T
    public func get(self, index: uint64): T
    public func set(self, index: uint64, value: T)
    public func length(self): uint64
    public func is_empty(self): bool
    public func capacity(self): uint64
    public func reserve(self, additional: uint64)
    public func clear(self): void
}

impl Drop for Vec<T> {
    func drop(self): void { free(self.data) }
}

impl Index<uint64, T> for Vec<T> {
    func index(self, i: uint64): T { return self.get(i) }
}
```

**Dependencies:** malloc/free (hosted)

### std.collections.string

Owned string type:

```gecko
public class String {
    let data: uint64
    let len: uint64
    let cap: uint64
}

impl String {
    public func new(): String
    public func with_capacity(capacity: uint64): String
    public func from(literal: string): String
    public func as_ptr(self): string
    public func push(self, c: uint8)
    public func push_str(self, s: string)
    public func length(self): uint64
}

impl Drop for String {
    func drop(self): void { free(self.data) }
}
```

**Dependencies:** malloc/free (hosted)

### std.memory.box / rc / weak

`Box<T>` uniquely owns allocated storage. `Rc<T>` and `Weak<T>` share a control
block with strong and weak counts. Drop hooks release handles automatically unless
`--no-auto-drop` is supplied. Box replacement and final strong Rc release invoke
the contained type's Drop hook before storage is freed.

Raw adoption methods require an unsafe region. Weak handles keep control metadata
alive after the contained value is dropped. All counts are non-atomic.

### std.memory.borrow_cell / ref / ref_mut

`BorrowCell<T is Copy>` supplies multiple immutable `Ref<T>` views or one
exclusive `RefMut<T>` view. Borrow hooks dispatch `@borrow(cell)` and
`@borrow_mut(cell)`; direct methods expose the same operations. Acquisitions check
exclusivity at runtime, and view Drop hooks release access and retained storage.

See [Memory Model](memory.md) for the API, examples, and ownership-analysis limits.

### Hosted I/O

The current stdlib tree does not ship a dedicated `std.io` module.
Hosted I/O is currently done via C interop declarations (`declare external` + libc functions).

## Freestanding Support

For kernel/embedded development, only import what you need:

```gecko
// Freestanding kernel code
import std.core.traits use { Drop }
import std.core.ops use { Add, Eq }
import std.option use { Option }

// No heap, no I/O - just traits and value types
```

The core modules have zero external dependencies.
