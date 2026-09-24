---
title: Result
description: Result<T, E> - Represents either a success (Ok) or failure (Err).
---

```gecko
class Result<T, E>
```

Result<T, E> - Represents either a success (Ok) or failure (Err).

Use Result for operations that can fail with a meaningful error type.

Example:
```
func divide(a: int32, b: int32): Result<int32, string> {
    if (b == 0) {
        return Result<int32, string>::err("division by zero")
    }
    return Result<int32, string>::ok(a / b)
}

// Using try - propagates errors automatically
func calculate(x: int32): Result<int32, string> {
    let val = try divide(x, 2)  // Returns early if error
    return Result<int32, string>::ok(val * 10)
}

// Using or - provides default on error
let val = divide(10, 0) or 0  // Returns 0 on error
```

## Type Parameters

- **T**
- **E**

## Fields

### value

```gecko
let value: T
```

### error

```gecko
let error: E
```

### is_ok

```gecko
let is_ok: bool
```

### active

```gecko
let active: bool
```

Tracks whether the selected payload is still owned by this Result.

## Methods

### ok

```gecko
func ok(val: T): Result<T, E>
```

Creates a success Result containing a value.

**Arguments:**

| Name | Type |
|------|------|
| `val` | `T` |

**Returns:** `Result<T, E>`

### err

```gecko
func err(e: E): Result<T, E>
```

Creates an error Result containing an error.

**Arguments:**

| Name | Type |
|------|------|
| `e` | `E` |

**Returns:** `Result<T, E>`

### is_success

```gecko
func is_success(self: void): bool
```

Returns true if this Result is Ok.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `bool`

### is_error

```gecko
func is_error(self: void): bool
```

Returns true if this Result is Err.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `bool`

### unwrap

```gecko
func unwrap(self: void): T
```

Moves out the Ok payload and clears `active`. Calling it on Err traps.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `T`

### unwrap_err

```gecko
func unwrap_err(self: void): E
```

Moves out the Err payload and clears `active`. Calling it on Ok traps.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |

**Returns:** `E`

### unwrap_or

```gecko
func unwrap_or(self: void, default_val: T): T
```

Returns the contained value, or a default if Err.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |
| `default_val` | `T` |

**Returns:** `T`

### map

```gecko
func map(self: void, mapper: func(T): T): Result<T, E>
```

Maps the success value with a function.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |
| `mapper` | `func(T): T` |

**Returns:** `Result<T, E>`

### map_err

```gecko
func map_err(self: void, mapper: func(E): E): Result<T, E>
```

Maps the error value with a function.

**Arguments:**

| Name | Type |
|------|------|
| `self` | `void` |
| `mapper` | `func(E): E` |

**Returns:** `Result<T, E>`

The Drop hook checks `active` and `is_ok`, then drops exactly one payload using
`@drop_in_place`. The implementation is visible in `stdlib/result.gecko`.

---

*Defined in `stdlib/result.gecko:7`*
