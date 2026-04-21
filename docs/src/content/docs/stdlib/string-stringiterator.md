---
title: StringIterator
description: StringIterator - Iterator over bytes in a String.
---

```gecko
class StringIterator
```

StringIterator - Iterator over bytes in a String.

## Fields

### data

```gecko
let data: uint64
```

### len

```gecko
let len: uint64
```

### pos

```gecko
let pos: uint64
```

## Methods

### new

```gecko
func new(data: uint64, len: uint64): StringIterator
```

**Arguments:**

| Name | Type |
|------|------|
| `data` | `uint64` |
| `len` | `uint64` |

**Returns:** `StringIterator`

### next

```gecko
func next(self: void): uint8
```

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `uint8`

### has_next

```gecko
func has_next(self: void): bool
```

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `bool`

---

*Defined in `stdlib/collections/string.gecko:16`*
