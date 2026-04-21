---
title: String
description: String - A growable, heap-allocated string.
---

```gecko
class String
```

String - A growable, heap-allocated string.

Unlike string literals (string type), String owns its data
and can be modified. Use this when you need to build strings
dynamically or concatenate multiple strings.

## Fields

### data

```gecko
let data: uint64
```

Pointer to the character data (null-terminated).

### len

```gecko
let len: uint64
```

Number of bytes in the string (excluding null terminator).

### cap

```gecko
let cap: uint64
```

Allocated capacity in bytes (including null terminator).

## Methods

### new

```gecko
func new(): String
```

Creates a new empty String with default capacity.

**Returns:** `String`

### with_capacity

```gecko
func with_capacity(capacity: uint64): String
```

Creates a new String with the specified initial capacity.

**Arguments:**

| Name | Type |
|------|------|
| `capacity` | `uint64` |

**Returns:** `String`

### from

```gecko
func from(literal: string): String
```

Creates a String from a string literal.

**Arguments:**

| Name | Type |
|------|------|
| `literal` | `string` |

**Returns:** `String`

### is_empty

```gecko
func is_empty(self: void): bool
```

Returns true if the string is empty.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `bool`

### length

```gecko
func length(self: void): uint64
```

Returns the length of the string in bytes.

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

### as_ptr

```gecko
func as_ptr(self: void): string
```

Returns a pointer to the underlying C string.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `string`

### char_at

```gecko
func char_at(self: void, index: uint64): uint8
```

Returns the byte at the given index.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |
| `index` | `uint64` |

**Returns:** `uint8`

### reserve

```gecko
func reserve(self: void, additional: uint64)
```

Ensures the string has at least the specified capacity.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |
| `additional` | `uint64` |

### push

```gecko
func push(self: void, c: uint8)
```

Appends a single byte to the string.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |
| `c` | `uint8` |

### push_str

```gecko
func push_str(self: void, s: string)
```

Appends a string literal to the string.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |
| `s` | `string` |

### append

```gecko
func append(self: void, other: String)
```

Appends another String to this string.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |
| `other` | `String` |

### clear

```gecko
func clear(self: void)
```

Clears the string, setting length to 0 but keeping capacity.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

### clone

```gecko
func clone(self: void): String
```

Creates a copy of this string.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `String`

### equals

```gecko
func equals(self: void, other: String): bool
```

Compares two strings for equality.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |
| `other` | `String` |

**Returns:** `bool`

### equals_str

```gecko
func equals_str(self: void, s: string): bool
```

Compares with a string literal for equality.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |
| `s` | `string` |

**Returns:** `bool`

### drop

```gecko
func drop(self: void)
```

Frees the memory owned by this string.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

### iter

```gecko
func iter(self: void): StringIterator
```

Returns an iterator over the bytes of this string.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `StringIterator`

---

*Defined in `stdlib/collections/string.gecko:55`*
