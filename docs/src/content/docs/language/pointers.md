---
title: Pointers and Memory
description: Low-level memory operations in Gecko
sidebar:
  order: 5
---

## Pointer Types

```gecko
let ptr: int*           // Pointer to int
let vptr: void*         // Void pointer
let pptr: int**         // Pointer to pointer
```

## Creating Pointers

### Address-of Operator

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

Use the `@deref` intrinsic:

```gecko
let x: int = 42
let ptr: int* = &x
let value: int = @deref(ptr)  // 42
```

## Writing to Pointers

Use `@write_volatile` for safe writes:

```gecko
let x: int = 0
let ptr: int* = &x
@write_volatile(ptr, 42)  // x is now 42
```

## Volatile Pointers

Mark pointers as volatile for memory-mapped I/O:

```gecko
let vga: uint16 volatile* = 0xB8000 as uint16 volatile*
@write_volatile(vga, 0x0F41)  // Write 'A' to VGA buffer
```

## Non-null Pointers

Mark pointers that should never be null:

```gecko
let ptr: int*!  // Non-nullable pointer

// Built-in methods
if ptr.is_null() {
    // handle null
}

if ptr.is_not_null() {
    let val: int = @deref(ptr)
}
```

## Pointer Arithmetic

```gecko
let base: uint8* = malloc(100) as uint8*
let offset: uint8* = base.offset(10)  // base + 10 bytes
```

## Null Checks

```gecko
let ptr: void* = malloc(100)
if @is_null(ptr) {
    // allocation failed
}
```

## Smart Pointers

The stdlib provides smart pointer types:

### Box<T> - Unique Ownership

```gecko
import box

let b: Box<int32> = Box<int32>::new(42)
let val: int32 = b.get()
b.set(100)
b.drop()  // Free memory
```

### Rc<T> - Reference Counted

```gecko
import rc

let r1: Rc<int32> = Rc<int32>::new(42)
let r2: Rc<int32> = r1.clone()  // Increment ref count
r1.drop()  // Decrement ref count
r2.drop()  // Memory freed when count reaches 0
```

### Weak<T> - Non-owning Reference

```gecko
import rc
import weak

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
| `@size_of<T>()` | Size of type in bytes |
| `@align_of<T>()` | Alignment of type |
