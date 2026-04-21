---
title: StringBuilder
description: StringBuilder - A simple growable byte buffer for building strings.
---

```gecko
class StringBuilder
```

StringBuilder - A simple growable byte buffer for building strings.

## Fields

### buffer

```gecko
let buffer: uint8*
```

### length

```gecko
let length: uint64
```

### capacity

```gecko
let capacity: uint64
```

## Methods

### init

```gecko
func init(self: void): void
```

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

### init_with_capacity

```gecko
func init_with_capacity(self: void, cap: uint64): bool
```

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |
| `cap` | `uint64` |

**Returns:** `bool`

### grow

```gecko
func grow(self: void, new_cap: uint64): bool
```

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |
| `new_cap` | `uint64` |

**Returns:** `bool`

### ensure_capacity

```gecko
func ensure_capacity(self: void, additional: uint64): bool
```

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |
| `additional` | `uint64` |

**Returns:** `bool`

### push_byte

```gecko
func push_byte(self: void, byte: uint8): bool
```

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |
| `byte` | `uint8` |

**Returns:** `bool`

### push_bytes

```gecko
func push_bytes(self: void, data: uint8*, len: uint64): bool
```

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |
| `data` | `uint8*` |
| `len` | `uint64` |

**Returns:** `bool`

### clear

```gecko
func clear(self: void): void
```

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

### destroy

```gecko
func destroy(self: void): void
```

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

### len

```gecko
func len(self: void): uint64
```

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `uint64`

### cap

```gecko
func cap(self: void): uint64
```

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `uint64`

### is_empty

```gecko
func is_empty(self: void): bool
```

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `bool`

### get

```gecko
func get(self: void, index: uint64): uint8
```

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |
| `index` | `uint64` |

**Returns:** `uint8`

---

*Defined in `stdlib/collections/string.gecko:313`*
