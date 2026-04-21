---
title: Raw
description: Raw<T> - Unsafe pointer wrapper for low-level memory operations.
---

```gecko
class Raw<T>
```

Raw<T> - Unsafe pointer wrapper for low-level memory operations.

This is the escape hatch when you need direct memory access.
Functions using `Raw<T>` in their signature signal potential unsafety.

Example:
```
let ptr: Raw<uint32> = Raw<uint32>::new(addr)
ptr.write(42)
let val: uint32 = ptr.read()
```

## Type Parameters

- **T**

## Fields

### addr

```gecko
let addr: uint64
```

The raw memory address stored as a 64-bit unsigned integer.

## Methods

### new

```gecko
func new(address: uint64): Raw<T>
```

Creates a new `Raw<T>` from a memory address.

The caller is responsible for ensuring the address is valid
and points to memory of the correct type.

**Arguments:**

| Name | Type |
|------|------|
| `address` | `uint64` |

**Returns:** `Raw<T>`

### null

```gecko
func null(): Raw<T>
```

Creates a null `Raw<T>` pointer.

Useful for representing optional pointers or uninitialized state.

**Returns:** `Raw<T>`

### init

```gecko
func init(self: void, address: uint64)
```

Initializes this pointer from a memory address.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |
| `address` | `uint64` |

### init_null

```gecko
func init_null(self: void)
```

Initializes this pointer as null (address 0).

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

### to_address

```gecko
func to_address(self: void): uint64
```

Returns the raw memory address as a uint64.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `uint64`

### is_null

```gecko
func is_null(self: void): bool
```

Returns true if this pointer is null (address 0).

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `bool`

### is_valid

```gecko
func is_valid(self: void): bool
```

Returns true if this pointer is not null.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `bool`

### read

```gecko
func read(self: void): T
```

Reads and returns the value at the pointer address.

Does not perform null checking - caller must ensure pointer is valid.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `T`

### write

```gecko
func write(self: void, value: T)
```

Writes a value to the pointer address.

Does not perform null checking - caller must ensure pointer is valid.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |
| `value` | `T` |

### add

```gecko
func add(self: void, n: uint64): Raw<T>
```

Returns a new pointer offset by `n` elements forward.

The offset is calculated as `n * sizeof(T)` bytes.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |
| `n` | `uint64` |

**Returns:** `Raw<T>`

### sub

```gecko
func sub(self: void, n: uint64): Raw<T>
```

Returns a new pointer offset by `n` elements backward.

The offset is calculated as `n * sizeof(T)` bytes.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |
| `n` | `uint64` |

**Returns:** `Raw<T>`

### copy_from

```gecko
func copy_from(self: void, src: Raw<T>, count: uint64)
```

Copies `count` elements from source pointer to this pointer.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |
| `src` | `Raw<T>` |
| `count` | `uint64` |

### zero

```gecko
func zero(self: void, count: uint64)
```

Sets `count` elements at this pointer to zero.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |
| `count` | `uint64` |

---

*Defined in `stdlib/memory/raw.gecko:3`*
