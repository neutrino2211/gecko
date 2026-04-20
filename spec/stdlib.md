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
│   ├── hash.gecko        # HashMap<K,V>, HashSet<T>
│   └── string.gecko      # String type
├── memory/
│   ├── box.gecko         # Box<T> - unique ownership
│   ├── rc.gecko          # Rc<T> - reference counting
│   ├── weak.gecko        # Weak<T> - weak references
│   └── raw.gecko         # Raw pointer utilities
├── option.gecko          # Option<T>
├── result.gecko          # Result<T, E>
├── io.gecko              # I/O primitives (hosted only)
└── fmt.gecko             # Formatting (hosted only)
```

## Module Descriptions

### std.core.traits

Lifecycle traits with compiler hooks:

```gecko
@drop_hook(.drop)
public trait Drop {
    func drop(self): void
}

@clone_hook(.clone)
public trait Clone {
    func clone(self): Self
}

@copy_hook(.copy)
public trait Copy {
    func copy(self): Self
}

public trait Default {
    func default_val(): Self
}
```

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
    public func map<U>(self, f: func(T): U): Option<U>
}
```

**Dependencies:** None (freestanding-compatible)

### std.result

Error handling type (requires effect typing):

```gecko
public class Result<T, E> {
    let is_ok: bool
    let ok_value: T
    let err_value: E
}

impl Result<T, E> {
    public func ok(val: T): Result<T, E>
    public func err(e: E): Result<T, E>
    public func is_ok(self): bool
    public func is_err(self): bool
    public func unwrap(self): T throws E
    public func unwrap_or(self, default: T): T
}
```

**Dependencies:** None (freestanding-compatible)

### std.collections.vec

Dynamic array:

```gecko
public class Vec<T> {
    let data: T*
    let len: uint64
    let cap: uint64
}

impl Vec<T> {
    public func new(): Vec<T>
    public func with_capacity(cap: uint64): Vec<T>
    public func push(self, value: T): void
    public func pop(self): Option<T>
    public func get(self, index: uint64): T
    public func set(self, index: uint64, value: T): void
    public func len(self): uint64
    public func is_empty(self): bool
    public func clear(self): void
}

impl Drop for Vec<T> {
    func drop(self): void { free(self.data) }
}

impl Index<uint64, T> for Vec<T> {
    func get(self, i: uint64): T { return self.get(i) }
}
```

**Dependencies:** malloc/free (hosted)

### std.collections.string

Owned string type:

```gecko
public class String {
    let data: uint8*
    let len: uint64
    let cap: uint64
}

impl String {
    public func new(): String
    public func from_cstr(s: string): String
    public func as_cstr(self): string
    public func push(self, c: uint8): void
    public func push_str(self, s: string): void
    public func len(self): uint64
}

impl Drop for String {
    func drop(self): void { free(self.data) }
}
```

**Dependencies:** malloc/free (hosted)

### std.memory.box

Unique heap ownership:

```gecko
public class Box<T> {
    let ptr: T*
}

impl Box<T> {
    public func new(value: T): Box<T>
    public func get(self): T*
    public func into_inner(self): T
}

impl Drop for Box<T> {
    func drop(self): void {
        // Drop inner value if it implements Drop
        free(self.ptr)
    }
}
```

**Dependencies:** malloc/free (hosted)

### std.memory.rc

Reference-counted pointer:

```gecko
public class Rc<T> {
    let ptr: T*
    let count: uint64*
}

impl Rc<T> {
    public func new(value: T): Rc<T>
    public func clone(self): Rc<T>
    public func get(self): T*
    public func strong_count(self): uint64
}

impl Drop for Rc<T> {
    func drop(self): void {
        @deref(self.count) = @deref(self.count) - 1
        if @deref(self.count) == 0 {
            free(self.ptr)
            free(self.count)
        }
    }
}
```

**Dependencies:** malloc/free (hosted)

### std.io

I/O primitives (hosted environments only):

```gecko
declare external func puts(s: string): int32
declare external func printf(fmt: string, ...): int32
declare external func getchar(): int32

public func print(s: string): void {
    puts(s)
}

public func println(s: string): void {
    puts(s)
    puts("\n")
}
```

**Dependencies:** libc (hosted only)

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

## Consolidation Plan

Current state has two directories (`stdlib/` and `std/`). Consolidate into single `stdlib/` with structure above.

### Migration

1. Move `std/` contents into appropriate `stdlib/` subdirectories
2. Update import paths in examples and tests
3. Delete `std/` directory
4. Update compiler default stdlib path

### Files to Consolidate

| Old Location | New Location |
|--------------|--------------|
| `std/mem.gecko` | `stdlib/memory/raw.gecko` |
| `std/io.gecko` | `stdlib/io.gecko` |
| `std/str.gecko` | `stdlib/collections/string.gecko` |
| `std/math.gecko` | `stdlib/math.gecko` |
| `std/types.gecko` | Remove (use stdint types directly) |
| `stdlib/core.gecko` | Split into `stdlib/core/traits.gecko` + `stdlib/core/ops.gecko` |
| `stdlib/vec.gecko` | `stdlib/collections/vec.gecko` |
| `stdlib/string.gecko` | Merge with `stdlib/collections/string.gecko` |
| `stdlib/box.gecko` | `stdlib/memory/box.gecko` |
| `stdlib/rc.gecko` | `stdlib/memory/rc.gecko` |
| `stdlib/weak.gecko` | `stdlib/memory/weak.gecko` |
| `stdlib/option.gecko` | `stdlib/option.gecko` |
| `stdlib/ops.gecko` | `stdlib/core/ops.gecko` |
| `stdlib/raw.gecko` | `stdlib/memory/raw.gecko` |
