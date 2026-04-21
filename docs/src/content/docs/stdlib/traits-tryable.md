---
title: Tryable
description: Tryable<T> - Trait for types supporting the try keyword
---

# Tryable

`Tryable<T>` enables the `try` keyword for unwrapping or early return.

## Import

```gecko
import std.core.traits use { Tryable }
```

## Trait Definition

```gecko
@try_hook(.has_value, .try_unwrap)
public trait Tryable<T> {
    func has_value(self): bool
    func try_unwrap(self): T
}
```

## Methods

### has_value

```gecko
func has_value(self): bool
```

Returns true if this contains a success value.

### try_unwrap

```gecko
func try_unwrap(self): T
```

Returns the contained value. Behavior is undefined if `has_value()` is false.

## Hook Behavior

When a type implements `Tryable<T>`, you can use the `try` keyword:

```gecko
let val = try expr
```

The compiler generates code equivalent to:

```c
({
    Type __tmp = expr;
    if (!has_value(&__tmp)) {
        return __tmp;  // Early return
    }
    try_unwrap(&__tmp);  // Yields the value
})
```

**Important**: The enclosing function must also return a type that implements `Tryable<T>`.

## Standard Implementations

- `Option<T>` implements `Tryable<T>`
- `Result<T, E>` implements `Tryable<T>`

## Example

```gecko
import std.option use { Option }

func get_value(): Option<int32> {
    return Option<int32>::some(42)
}

// Function must return Tryable type to use try
func process(): Option<int32> {
    let val = try get_value()  // Early returns None if get_value() returns None
    return Option<int32>::some(val * 2)
}
```
