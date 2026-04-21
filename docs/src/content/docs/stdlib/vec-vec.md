---
title: Vec
description: Vec<T> - A growable, heap-allocated array.
---

```gecko
class Vec<T>
```

Vec<T> - A growable, heap-allocated array.

Provides a dynamic array that can grow as elements are added.
Elements are stored contiguously in memory.

Example:
```
let v = Vec<int32>::new()
v.push(10)
v.push(20)
v.push(30)
let sum = v.get(0) + v.get(1) + v.get(2)  // 60
v.drop()
```

## Type Parameters

- **T**

## Fields

### data

```gecko
let data: uint64
```

Pointer to the element data.

### len

```gecko
let len: uint64
```

Number of elements in the vector.

### cap

```gecko
let cap: uint64
```

Allocated capacity (number of elements).

## Methods

### new

```gecko
func new(): Vec<T>
```

Creates a new empty Vec with default capacity.

**Returns:** `Vec<T>`

### with_capacity

```gecko
func with_capacity(capacity: uint64): Vec<T>
```

Creates a new Vec with the specified initial capacity.

**Arguments:**

| Name | Type |
|------|------|
| `capacity` | `uint64` |

**Returns:** `Vec<T>`

### is_empty

```gecko
func is_empty(self: void): bool
```

Returns true if the vector is empty.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `bool`

### length

```gecko
func length(self: void): uint64
```

Returns the number of elements.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `uint64`

### capacity

```gecko
func capacity(self: void): uint64
```

Returns the allocated capacity.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `uint64`

### get

```gecko
func get(self: void, index: uint64): T
```

Returns the element at the given index.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |
| `index` | `uint64` |

**Returns:** `T`

### set

```gecko
func set(self: void, index: uint64, value: T)
```

Sets the element at the given index.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |
| `index` | `uint64` |
| `value` | `T` |

### reserve

```gecko
func reserve(self: void, additional: uint64)
```

Ensures the vector has at least the specified additional capacity.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |
| `additional` | `uint64` |

### push

```gecko
func push(self: void, value: T)
```

Appends an element to the end of the vector.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |
| `value` | `T` |

### pop

```gecko
func pop(self: void): T
```

Removes and returns the last element.
Returns a default value if empty.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `T`

### first

```gecko
func first(self: void): T
```

Returns the first element, or default if empty.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `T`

### last

```gecko
func last(self: void): T
```

Returns the last element, or default if empty.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `T`

### clear

```gecko
func clear(self: void)
```

Clears the vector, setting length to 0 but keeping capacity.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

### clone

```gecko
func clone(self: void): Vec<T>
```

Creates a copy of this vector.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `Vec<T>`

### as_ptr

```gecko
func as_ptr(self: void): T*
```

Returns a raw pointer to the data.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `T*`

### drop

```gecko
func drop(self: void)
```

Frees the memory owned by this vector.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

---

*Defined in `stdlib/collections/vec.gecko:10`*
