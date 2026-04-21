---
title: Intrinsics
description: Built-in compiler operations in Gecko
sidebar:
  order: 10
---

Intrinsics are built-in operations that the compiler handles specially. They provide low-level functionality that can't be expressed in regular Gecko code.

## Memory Intrinsics

### @deref

Dereference a pointer to read or write the value it points to:

```gecko
let ptr: int* = // ...
let value = @deref(ptr)     // Read
@deref(ptr) = 42            // Write
```

### @size_of

Get the size of a type in bytes:

```gecko
let int_size = @size_of<int>()      // 4 or 8 depending on platform
let ptr_size = @size_of<void*>()    // 8 on 64-bit
let struct_size = @size_of<MyStruct>()
```

### @align_of

Get the alignment requirement of a type:

```gecko
let int_align = @align_of<int>()
let struct_align = @align_of<MyStruct>()
```

### @copy

Copy bytes from one memory location to another:

```gecko
let src: uint8* = // ...
let dst: uint8* = // ...
@copy(dst, src, 100)  // Copy 100 bytes
```

### @zero

Zero out a memory region:

```gecko
let ptr: uint8* = // ...
@zero(ptr, 256)  // Zero 256 bytes
```

## Pointer Arithmetic

### @ptr_add

Offset a pointer forward by a number of elements:

```gecko
let arr: int* = // ...
let third = @ptr_add(arr, 2)  // Points to arr[2]
```

### @ptr_sub

Offset a pointer backward:

```gecko
let end: int* = // ...
let start = @ptr_sub(end, 10)  // 10 elements before end
```

## Volatile Operations

For memory-mapped I/O and hardware registers:

### @write_volatile

Write to memory with volatile semantics (prevents optimization):

```gecko
let mmio: uint32* = 0xB8000 as uint32*
@write_volatile(mmio, 0x1234)  // Write won't be optimized away
```

### @read_volatile

Read from memory with volatile semantics:

```gecko
let status: uint32* = 0xB8004 as uint32*
let value = @read_volatile(status)  // Read won't be cached
```

## Null Checking

### @is_null

Check if a pointer is null:

```gecko
let ptr: int* = // ...
if @is_null(ptr) {
    // Handle null case
}
```

### @is_not_null

Check if a pointer is not null (enables type narrowing):

```gecko
let ptr: int* = // ...
if @is_not_null(ptr) {
    // ptr is narrowed to int*! (non-null) in this block
    let value = @deref(ptr)
}
```

## Control Flow

### @unreachable

Mark code as unreachable (compiler optimization hint):

```gecko
func get_sign(x: int): int {
    if x > 0 {
        return 1
    } else if x < 0 {
        return -1
    } else {
        return 0
    }
    @unreachable()  // Should never reach here
}
```

### @trap

Trigger an abnormal program termination:

```gecko
func assert(condition: bool): void {
    if !condition {
        @trap()  // Abort the program
    }
}
```

## Builtin Operators

These implement primitive type operations. They're mainly used internally by the standard library to bootstrap operator traits for primitive types.

```gecko
// Arithmetic
@builtin_add(a, b)   // a + b for primitives
@builtin_sub(a, b)   // a - b
@builtin_mul(a, b)   // a * b
@builtin_div(a, b)   // a / b

// Comparison
@builtin_eq(a, b)    // a == b
@builtin_ne(a, b)    // a != b
@builtin_lt(a, b)    // a < b
@builtin_gt(a, b)    // a > b
@builtin_le(a, b)    // a <= b
@builtin_ge(a, b)    // a >= b

// Bitwise
@builtin_bitand(a, b)  // a & b
@builtin_bitor(a, b)   // a | b
@builtin_bitxor(a, b)  // a ^ b
@builtin_shl(a, b)     // a << b
@builtin_shr(a, b)     // a >> b

// Unary
@builtin_neg(a)        // -a
@builtin_not(a)        // !a
```

## Complete Reference

| Intrinsic | Description | Example |
|-----------|-------------|---------|
| `@deref(ptr)` | Dereference pointer | `@deref(p) = 42` |
| `@size_of<T>()` | Size of type in bytes | `@size_of<int>()` |
| `@align_of<T>()` | Alignment of type | `@align_of<int>()` |
| `@copy(dst, src, n)` | Copy n bytes | `@copy(d, s, 100)` |
| `@zero(ptr, n)` | Zero n bytes | `@zero(p, 256)` |
| `@ptr_add(ptr, n)` | Pointer + n elements | `@ptr_add(arr, 5)` |
| `@ptr_sub(ptr, n)` | Pointer - n elements | `@ptr_sub(end, 5)` |
| `@write_volatile(ptr, val)` | Volatile write | `@write_volatile(mmio, v)` |
| `@read_volatile(ptr)` | Volatile read | `@read_volatile(status)` |
| `@is_null(ptr)` | Check for null | `@is_null(p)` |
| `@is_not_null(ptr)` | Check not null (narrows) | `@is_not_null(p)` |
| `@unreachable()` | Mark unreachable | `@unreachable()` |
| `@trap()` | Abort program | `@trap()` |

## Usage Notes

1. **Intrinsics are unsafe** - They bypass normal safety checks
2. **Use sparingly** - Prefer safe abstractions when possible
3. **Document usage** - Explain why intrinsics are needed
4. **Test thoroughly** - Intrinsic misuse can cause undefined behavior

## Example: Manual Memory Management

```gecko
func allocate_array(count: uint64): int* {
    let size = count * @size_of<int>()
    let ptr = malloc(size) as int*
    
    if @is_null(ptr) {
        @trap()  // Allocation failed
    }
    
    @zero(ptr, size)
    return ptr
}

func copy_array(dst: int*, src: int*, count: uint64): void {
    @copy(dst, src, count * @size_of<int>())
}

func free_array(ptr: int*): void {
    if @is_not_null(ptr) {
        free(ptr as void*)
    }
}
```
