---
title: Option
description: Option<T> - Represents an optional value.
---

```gecko
class Option<T>
```

Option<T> - Represents an optional value.

An Option is either `Some` (containing a value) or `None` (empty).
Use this instead of null pointers for safer code.

Example:
```
func find(arr: Vec<int32>, target: int32): Option<uint64> {
    // ... search logic ...
    if (found) {
        return Option<uint64>::some(index)
    }
    return Option<uint64>::none()
}

let result = find(arr, 42)
if (result.is_some()) {
    let idx = result.unwrap()
}
```

## Type Parameters

- **T**

## Fields

### value

```gecko
let value: T
```

The contained value (only valid if has_value is true).

### has_value

```gecko
let has_value: bool
```

Whether this Option contains a value.

## Methods

### some

```gecko
func some(val: T): Option<T>
```

Creates an Option containing a value.

**Arguments:**

| Name | Type |
|------|------|
| `val` | `T` |

**Returns:** `Option<T>`

### none

```gecko
func none(): Option<T>
```

Creates an empty Option.

**Returns:** `Option<T>`

### is_some

```gecko
func is_some(self: void): bool
```

Returns true if this Option contains a value.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `bool`

### is_none

```gecko
func is_none(self: void): bool
```

Returns true if this Option is empty.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `bool`

### unwrap

```gecko
func unwrap(self: void): T
```

Returns the contained value.
Behavior is undefined if the Option is None.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `T`

### unwrap_or

```gecko
func unwrap_or(self: void, default_val: T): T
```

Returns the contained value, or a default if None.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |
| `default_val` | `T` |

**Returns:** `T`

### get_or

```gecko
func get_or(self: void, default_val: T): T
```

Returns the contained value, or computes it from a default.
Note: In Gecko, this takes the computed default directly.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |
| `default_val` | `T` |

**Returns:** `T`

---

*Defined in `stdlib/option.gecko:3`*
