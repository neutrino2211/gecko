---
title: Pointers and Memory
description: Low-level memory operations in Gecko
sidebar:
  order: 5
---

## Pointer Types

```gecko
let p: int*         // Nullable pointer to non-null int
let q: int?*        // Nullable pointer to nullable int
let raw: void*      // Opaque C pointer
```

`Type*?` is invalid syntax.

## Creating Pointers

### Address-Of

```gecko
let x: int = 42
let ptr: int* = &x
```

### Casting

```gecko
let addr: uint64 = 0xB8000
let ptr: uint16* = addr as uint16*
```

## Dereferencing

Use `@deref` for explicit loads and stores:

```gecko
let x: int = 42
let ptr: int* = &x
let value: int = @deref(ptr)  // 42
@deref(ptr) = 100
```

## Null Checks

```gecko
let ptr: int* = null

if ptr == null {
    return
}

if @is_not_null(ptr) {
    let value: int = @deref(ptr)
}
```

## Volatile Pointers

Mark pointers as `volatile` for memory-mapped I/O:

```gecko
let vga: uint16 volatile* = 0xB8000 as uint16 volatile*
@write_volatile(vga, 0x0F41)  // Write 'A' to VGA buffer
```

## Pointer Arithmetic

Raw pointer arithmetic with `+` and `-` is disallowed:

```gecko
let p: uint8* = 0 as uint8*
let q: uint8* = p + 1   // compile error
```

Use explicit pointer intrinsics when needed:

```gecko
let base: uint8* = 0 as uint8*
let next: uint8* = @ptr_add(base, 1) as uint8*
let prev: uint8* = @ptr_sub(next, 1) as uint8*
```

For typed access patterns, prefer `std.memory.buffer.Buffer<T>`:

```gecko
import std.memory.buffer use { Buffer }

let ptr: uint32* = 0 as uint32*
let buf: Buffer<uint32> = Buffer<uint32>::from_raw(ptr, 16)
let first: uint32 = buf[0]
```

## Stdlib Memory Types

The stdlib provides smart pointer types:

### Box<T> - Unique Ownership

```gecko
import std.memory.box use { Box }

let b: Box<int32> = Box<int32>::new(42)
let val: int32 = b.get()
b.set(100)
b.drop()  // Free memory
```

### Rc<T> - Reference Counted

```gecko
import std.memory.rc use { Rc }

let r1: Rc<int32> = Rc<int32>::new(42)
let r2: Rc<int32> = r1.clone()  // Increment ref count
r1.drop()  // Decrement ref count
r2.drop()  // Memory freed when count reaches 0
```

### Weak<T> - Non-owning Reference

```gecko
import std.memory.rc use { Rc }
import std.memory.weak use { Weak }

let strong: Rc<int32> = Rc<int32>::new(42)
let w: Weak<int32> = Weak<int32>::from_rc(strong)

if w.is_valid() {
    let val: int32 = w.get()
}
```

## Intrinsics

| Intrinsic | Description |
|-----------|-------------|
| `@deref(ptr)` | Read value at pointer |
| `@write_volatile(ptr, val)` | Write value to pointer |
| `@is_null(ptr)` | Check if pointer is null |
| `@is_not_null(ptr)` | Check if pointer is not null |
| `@ptr_add(ptr, n)` | Pointer + n elements |
| `@ptr_sub(ptr, n)` | Pointer - n elements |
| `@size_of<T>()` | Size of type in bytes |
| `@align_of<T>()` | Alignment of type |
