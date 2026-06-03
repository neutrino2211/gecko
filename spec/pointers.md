# Pointers

Pointers in Gecko are explicit, C-compatible references.

## Design Principles

1. **Simple model** - Pointer semantics stay close to C
2. **Explicit dereference** - `@deref(...)` is required for pointer loads/stores
3. **No hidden ownership** - Lifetime and allocation are explicit
4. **Interop first** - Pointer types map directly to C ABI

## Pointer Types

```gecko
T*      // nullable pointer to non-null T
T?*     // nullable pointer to nullable T
void*   // opaque pointer
```

Invalid:

```gecko
T*?     // invalid syntax (pointer nullability is implicit)
```

## Core Operations

### Address-Of

```gecko
let x: int = 42
let p: int* = &x
```

### Dereference

```gecko
let value: int = @deref(p)
@deref(p) = 100
```

### Null Checks

```gecko
if p == null {
    return
}

let value = @deref(p)
```

### Pointer Casts

```gecko
let raw: void* = malloc(64)
let bytes: uint8* = raw as uint8*

let addr: uint64 = bytes as uint64
let restored: uint8* = addr as uint8*
```

### Pointer Arithmetic Policy

Raw pointer arithmetic in expression operators is disallowed:

```gecko
let p: uint8* = ...
let q = p + 1   // error
```

Use explicit intrinsics for low-level offset operations:

```gecko
let q: uint8* = @ptr_add(p, 1) as uint8*
let r: uint8* = @ptr_sub(q, 1) as uint8*
```

For typed, bounds-aware access patterns, prefer `std.memory.buffer.Buffer<T>`.

### Equality

```gecko
if p == q {
    // same address
}
```

## Member and Index Access

Pointer member access uses `.` (no `->`):

```gecko
let point_ptr: Point*
let x = point_ptr.x
```

Indexing follows normal `[]` syntax when applicable:

```gecko
let data: uint8*
let first = data[0]
data[0] = 65
```

## Volatile Pointers

```gecko
let reg: uint32 volatile* = 0x40000000 as uint32 volatile*
@deref(reg) = 1
```

## C Interop

Pointers pass to/from external C declarations directly:

```gecko
declare external func malloc(size: uint64): void*
declare external func free(ptr: void*): void

let p: uint8* = malloc(128) as uint8*
free(p as void*)
```

## Gaps and Limitations

- No ownership or borrow checking
- No pointer provenance checking
- No built-in bounds checks for pointer indexing
- Intrinsics remain low-level escape hatches; `Buffer<T>` is the preferred typed abstraction
