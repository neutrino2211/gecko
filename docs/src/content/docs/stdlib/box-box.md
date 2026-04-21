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
let val: int32 = b.get()
b.drop()  // Manual cleanup (automatic in future)
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

The value is copied to heap-allocated memory. The `Box<T>`
becomes the sole owner of this memory.

**Arguments:**

| Name | Type |
|------|------|
| `value` | `T` |

**Returns:** `Box<T>`

### from_raw

```gecko
func from_raw(raw_ptr: uint64): Box<T>
```

Takes ownership of memory from a raw pointer.

The caller must ensure the pointer was allocated with `malloc`
and is of the correct type. After this call, the `Box<T>` owns
the memory and will free it when dropped.

**Arguments:**

| Name | Type |
|------|------|
| `raw_ptr` | `uint64` |

**Returns:** `Box<T>`

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
func get(self: void): T
```

Returns the value stored in the Box.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `T`

### set

```gecko
func set(self: void, value: T)
```

Overwrites the value stored in the Box.

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
In the future, this will be called automatically when
the Box goes out of scope.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

---

*Defined in `stdlib/memory/box.gecko:8`*
