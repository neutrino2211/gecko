---
title: Result
description: Result<T, E> - Represents either success or failure
---

# Result

`Result<T, E>` represents an operation that can succeed with value `T` or fail with error `E`.

## Import

```gecko
import std.result use { Result }
```

## Type Definition

```gecko
public class Result<T, E> {
    let value: T
    let error: E
    let is_ok: bool
}
```

## Static Methods

### ok

```gecko
func ok(val: T): Result<T, E>
```

Creates a success Result containing a value.

### err

```gecko
func err(e: E): Result<T, E>
```

Creates an error Result containing an error.

## Instance Methods

### is_success

```gecko
func is_success(self): bool
```

Returns true if this Result is Ok.

### is_error

```gecko
func is_error(self): bool
```

Returns true if this Result is Err.

### unwrap

```gecko
func unwrap(self): T
```

Returns the contained value. Behavior is undefined if the Result is Err.

### unwrap_err

```gecko
func unwrap_err(self): E
```

Returns the contained error. Behavior is undefined if the Result is Ok.

### unwrap_or

```gecko
func unwrap_or(self, default_val: T): T
```

Returns the contained value, or a default if Err.

### map

```gecko
func map<U>(self, mapper: func(T): U): Result<U, E>
```

Maps the success value with a function.

### map_err

```gecko
func map_err<F>(self, mapper: func(E): F): Result<T, F>
```

Maps the error value with a function.

## Trait Implementations

### Tryable<T>

Enables `try` keyword for early return on error:

```gecko
func process(): Result<int, string> {
    let val = try some_operation()  // Early returns if error
    return Result<int, string>::ok(val * 2)
}
```

### Orable<T>

Enables `or` keyword for default values:

```gecko
let val = some_operation() or 0  // Returns 0 on error
```

## Example

```gecko
import std.result use { Result }

func divide(a: int32, b: int32): Result<int32, string> {
    if (b == 0) {
        return Result<int32, string>::err("division by zero")
    }
    return Result<int32, string>::ok(a / b)
}

func main(): int {
    // Explicit handling
    let result = divide(10, 2)
    if (result.is_success()) {
        let val = result.unwrap()
        // use val
    }
    
    // Using or
    let safe_val = divide(10, 0) or 0
    
    return 0
}
```
