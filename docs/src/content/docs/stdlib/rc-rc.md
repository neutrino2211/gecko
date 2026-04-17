---
title: Rc
description: Rc<T> - Reference Counted Smart Pointer.
---

```ts
class Rc<T>
```

Rc<T> - Reference Counted Smart Pointer.

Enables shared ownership of heap-allocated data through
automatic reference counting. Multiple `Rc<T>` instances can
point to the same data, and the memory is freed when the
last reference is dropped.

Use `clone()` to create additional references.
Use `Weak<T>` for non-owning references that don't prevent cleanup.

Example:
```
let rc1: Rc<int32> = Rc<int32>::new(42)
let rc2: Rc<int32> = rc1.clone()  // refcount = 2
rc2.drop()                         // refcount = 1
rc1.drop()                         // frees memory
```

## Type Parameters

- **T**

## Fields

### ptr

```ts
let ptr: uint64
```

Internal pointer to the reference-counted allocation.

## Methods

### new

```ts
func new(value: T): Rc<T>
```

Creates a new `Rc<T>` containing the given value.

Allocates memory for the value plus reference count metadata.
The initial strong count is 1.

**Arguments:**

| Name | Type |
|------|------|
| `value` | `T` |

**Returns:** `Rc<T>`

### is_valid

```ts
func is_valid(self: void): bool
```

Returns true if this Rc points to valid memory.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `bool`

### strong_count

```ts
func strong_count(self: void): uint64
```

Returns the current strong reference count.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `uint64`

### get

```ts
func get(self: void): T
```

Returns the value stored in this Rc.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `T`

### clone

```ts
func clone(self: void): Rc<T>
```

Creates a new reference to the same data.

Increments the strong reference count and returns a new `Rc<T>`
pointing to the same allocation.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `Rc<T>`

### drop

```ts
func drop(self: void)
```

Decrements the reference count and frees memory if this was the last reference.

After calling drop, this Rc is invalidated.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

### as_raw

```ts
func as_raw(self: void): uint64
```

Returns a raw pointer to the value (does not affect refcount).

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `uint64`

### inner_ptr

```ts
func inner_ptr(self: void): uint64
```

Returns the internal pointer for creating `Weak<T>` references.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `uint64`

---

*Defined in `stdlib/rc.gecko:13`*
