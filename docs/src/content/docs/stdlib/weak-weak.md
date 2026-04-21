---
title: Weak
description: Weak<T> - Non-owning reference to `Rc<T>` data.
---

```gecko
class Weak<T>
```

Weak<T> - Non-owning reference to `Rc<T>` data.

A weak reference does not prevent the data from being freed.
Before accessing the data, you must check if the `Rc` is still
alive using `is_alive()`.

Use weak references to break reference cycles in data structures
like graphs or trees with parent pointers.

Example:
```
let rc: Rc<int32> = Rc<int32>::new(42)
let weak: Weak<int32> = Weak<int32>::from_rc_ptr(rc.inner_ptr())

if (weak.is_alive()) {
    let val: int32 = weak.try_get()
}
```

## Type Parameters

- **T**

## Fields

### ptr

```gecko
let ptr: uint64
```

Internal pointer to the same allocation as the `Rc`.

## Methods

### from_rc_ptr

```gecko
func from_rc_ptr(rc_ptr: uint64): Weak<T>
```

Creates a weak reference from an `Rc`'s internal pointer.

Increments the weak reference count.

**Arguments:**

| Name | Type |
|------|------|
| `rc_ptr` | `uint64` |

**Returns:** `Weak<T>`

### null

```gecko
func null(): Weak<T>
```

Creates a null weak reference.

**Returns:** `Weak<T>`

### is_valid

```gecko
func is_valid(self: void): bool
```

Returns true if this weak reference is not null.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `bool`

### strong_count

```gecko
func strong_count(self: void): uint64
```

Returns the strong reference count of the underlying allocation.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `uint64`

### weak_count

```gecko
func weak_count(self: void): uint64
```

Returns the weak reference count of the underlying allocation.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `uint64`

### is_alive

```gecko
func is_alive(self: void): bool
```

Returns true if the underlying `Rc` still exists.

If this returns false, the data has been freed and `try_get()`
will return a default value.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `bool`

### try_get

```gecko
func try_get(self: void): T
```

Attempts to read the value, returning 0 if the Rc was dropped.

Check `is_alive()` first to know if the value is valid.
In the future, this will return `Option<T>`.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `T`

### clone

```gecko
func clone(self: void): Weak<T>
```

Creates another weak reference to the same data.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `Weak<T>`

### drop

```gecko
func drop(self: void)
```

Releases this weak reference.

Decrements the weak count. If both strong and weak counts reach zero,
the underlying memory allocation is freed.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

---

*Defined in `stdlib/memory/weak.gecko:6`*
