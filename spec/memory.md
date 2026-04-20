# Memory Model

Gecko has a manual memory model similar to C, with some additional type-level safety.

## Stack Allocation

Local variables are stack-allocated:

```gecko
func example(): void {
    let x: int32 = 42           // on stack
    let buffer: [1024]uint8     // array on stack
    let point: Point            // struct on stack
}
```

## Pointers

### Basic Pointers

```gecko
let p: int32*        // pointer to int32
let q: uint8*        // pointer to uint8
let r: void*         // void pointer
```

### Address-Of

```gecko
let x: int32 = 42
let p: int32* = &x
```

### Dereference

```gecko
let value: int32 = *p
*p = 100
```

### Null Pointers

```gecko
let p: int32* = nil
if p != nil {
    // safe to dereference
}
```

**Gap**: No null safety at compile time for regular pointers.

## Non-Null Pointers

Declare pointers that cannot be null:

```gecko
let p: int32*!    // non-null pointer
```

The compiler tracks nullability:

```gecko
func process(data: uint8*!): void {
    // data guaranteed non-null
}

let p: int32* = nil
process(p)  // ERROR: cannot pass nullable to non-null
```

**Gap**: Non-null checking is incomplete. Some paths may allow null assignment.

## Volatile Pointers

For memory-mapped I/O:

```gecko
let vga: uint16 volatile* = 0xB8000 as uint16 volatile*
*vga = 0x0F41  // compiler won't optimize away
```

## Pointer Arithmetic

Not directly supported. Use casts:

```gecko
let base: uint8* = buffer
let offset: uint8* = (base as uint64 + 10) as uint8*
```

**Gap**: No safe pointer arithmetic syntax.

## Heap Allocation

Use C functions:

```gecko
declare external func malloc(size: uint64): void*
declare external func free(ptr: void*): void

let p: int32* = malloc(4) as int32*
*p = 42
free(p as void*)
```

Or stdlib smart pointers:

```gecko
import box use { Box }

let b: Box<int32> = Box<int32>::new(42)
let value: int32 = b.get()
b.drop()  // manual cleanup required
```

## Arrays

### Fixed-Size Arrays

```gecko
let buffer: [4096]uint8
buffer[0] = 0xFF
let len: uint64 = 4096
```

### Array Decay

Arrays decay to pointers when passed to functions:

```gecko
func process(data: uint8*, len: uint64): void {
    // ...
}

let buffer: [1024]uint8
process(&buffer[0], 1024)
```

**Gap**: No safe array passing that preserves size information.

## Memory Safety

Gecko provides minimal memory safety:

| Feature | Status |
|---------|--------|
| Null pointer checks | Runtime only |
| Non-null types | Partial |
| Bounds checking | None |
| Use-after-free | None |
| Double-free | None |
| Buffer overflow | None |
| Dangling pointers | None |

**Design Note**: Gecko prioritizes C interop and low-level control over memory safety. Use smart pointers from stdlib for safer patterns.

## Gaps and Limitations

- No ownership system
- No borrow checker
- No automatic memory management
- No RAII (Drop trait exists but not auto-called)
- No bounds-checked arrays
- No slice types
- No lifetime annotations
- No pointer arithmetic syntax
- Incomplete non-null tracking
