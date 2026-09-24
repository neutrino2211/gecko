---
title: Box
description: Box<T> - Unique ownership smart pointer.
---

```gecko
class Box<T>
```

Box<T> - Unique ownership smart pointer.

Provides single-owner heap allocation with automatic cleanup.
When a `Box<T>` goes out of scope, the memory is freed.

Unlike `Rc<T>`, `Box<T>` cannot be cloned - ownership must be
transferred (moved) rather than shared.

Example:
```
let b: Box<int32> = Box<int32>::new(42)
let val: int32 = b.into_inner()
```

## Type Parameters

- **T**

## Fields

### ptr

```gecko
let ptr: uint64
```

Internal pointer to heap-allocated memory.

## Methods

### new

```gecko
func new(value: T): Box<T>
```

Allocates memory and stores a value, returning a new `Box<T>`.

The value is moved into heap-allocated memory. The `Box<T>` becomes its owner.

**Arguments:**

| Name | Type |
|------|------|
| `value` | `T` |

**Returns:** `Box<T>`

### from_raw

```gecko
@unsafe
func from_raw(raw_ptr: uint64): Box<T>
```

Takes ownership of memory from a raw pointer.

The caller must ensure the pointer owns initialized, correctly aligned storage
compatible with `free`, and that no other owner will release it. After this call, the `Box<T>` owns
the memory and will free it when dropped.

**Arguments:**

| Name | Type |
|------|------|
| `raw_ptr` | `uint64` |

**Returns:** `Box<T>`

### from_raw_checked

```gecko
@unsafe
func from_raw_checked(raw_ptr: uint64): Option<Box<T>>
```

Creates a Box from a raw pointer with runtime validation.

FFI-facing invariant checks:
- pointer must be non-null

**Arguments:**

| Name | Type |
|------|------|
| `raw_ptr` | `uint64` |

**Returns:** `Option<Box<T>>`

### is_valid

```gecko
func is_valid(self: void): bool
```

Returns true if this Box contains valid (non-null) memory.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `bool`

### get

```gecko
@unsafe
func get(self: void): T
```

Copies the contained value without transferring ownership. Calling it requires
an unsafe context; copying a payload with Drop can duplicate ownership.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `T`

### into_inner

```gecko
func into_inner(self: void): T
```

Moves out the value, frees its storage, and invalidates the Box. The caller owns
the returned value.

### set

```gecko
func set(self: void, value: T)
```

Drops the previous contained value, then stores the replacement.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |
| `value` | `T` |

### as_raw

```gecko
func as_raw(self: void): uint64
```

Returns the raw pointer without giving up ownership.

The Box still owns the memory after this call.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `uint64`

### into_raw

```gecko
func into_raw(self: void): uint64
```

Gives up ownership and returns the raw pointer.

After this call, the Box is invalidated and the caller
is responsible for freeing the memory.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `uint64`

### drop

```gecko
func drop(self: void)
```

Frees the memory owned by this Box.

After calling drop, the Box is invalidated.
The Drop hook also runs automatically at scope exit unless `--no-auto-drop` is used.
It drops the contained value before freeing storage.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

---

*Defined in `stdlib/memory/box.gecko:11`*
