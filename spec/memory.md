# Memory Model

Gecko uses explicit manual memory management with C-compatible semantics.

## Stack Allocation

Local bindings are stack-allocated:

```gecko
func example(): void {
    let x: int = 42
    let buffer: [1024]uint8
    let point: Point
}
```

## Pointer Model

Pointer nullability is always implicit on the pointer itself.

```gecko
let p: int*      // nullable pointer to non-null int
let q: int?*     // nullable pointer to nullable int
let r: void*     // opaque C pointer
```

`Type*?` is invalid syntax.

## Address-Of

```gecko
let x: int = 42
let p: int* = &x
```

## Dereference

Dereference is explicit through `@deref(...)`.

```gecko
let value: int = @deref(p)
@deref(p) = 100
```

## Null Pointers

Use `null` for null pointers.

```gecko
let p: int* = null

if p != null {
    let value = @deref(p)
}
```

## Volatile Pointers

For memory-mapped I/O:

```gecko
let vga: uint16 volatile* = 0xB8000 as uint16 volatile*
@deref(vga) = 0x0F41
```

## Pointer Arithmetic

Raw pointer arithmetic via `+` / `-` is disallowed.

Use explicit intrinsics when low-level address stepping is required:

```gecko
let base: uint8* = buffer as uint8*
let offset: uint8* = @ptr_add(base, 10) as uint8*
```

For regular typed access, prefer `std.memory.buffer.Buffer<T>` over direct pointer math.

## Heap Allocation

Heap memory is typically allocated via C interop or stdlib wrappers:

```gecko
declare external func malloc(size: uint64): void*
declare external func free(ptr: void*): void

let p: int* = malloc(8) as int*
@deref(p) = 42
free(p as void*)
```

Or with stdlib abstractions:

```gecko
import std.memory.box use { Box }

let b: Box<int> = Box<int>::new(42)
let value_ptr: int* = b.get()
let value: int = @deref(value_ptr)
```

## Arrays

### Fixed-Size Arrays

```gecko
let buffer: [4096]uint8
buffer[0] = 0xFF
```

### Array Decay

Arrays are passed as pointers explicitly:

```gecko
func process(data: uint8*, len: uint64): void {
    // ...
}

let buffer: [1024]uint8
process(&buffer[0], 1024)
```

## Memory Safety

Gecko intentionally keeps memory behavior close to C.

| Feature | Status |
|---------|--------|
| Null pointer prevention | Programmer-managed |
| Bounds checking | None by default |
| Use-after-free prevention | None |
| Double-free prevention | None |
| Dangling pointer prevention | None |

Use stdlib abstractions (`Box`, `Rc`, `Weak`, collections) where stronger invariants are needed.

## Gaps and Limitations

- No ownership system
- No borrow checker
- No automatic memory management
- No lifetime annotations
- No built-in bounds-checked arrays
