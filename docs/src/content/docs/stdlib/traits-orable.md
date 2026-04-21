---
title: Orable
description: Orable<T> - Trait for types supporting the or keyword
---

# Orable

`Orable<T>` enables the `or` keyword for providing default values.

## Import

```gecko
import std.core.traits use { Orable }
```

## Trait Definition

```gecko
@or_hook(.unwrap_or)
public trait Orable<T> {
    func unwrap_or(self, default_val: T): T
}
```

## Methods

### unwrap_or

```gecko
func unwrap_or(self, default_val: T): T
```

Returns the contained value, or the default if none.

## Hook Behavior

When a type implements `Orable<T>`, you can use the `or` keyword:

```gecko
let val = expr or default
```

The compiler generates a call to `unwrap_or(expr, default)`.

## Standard Implementations

- `Option<T>` implements `Orable<T>`
- `Result<T, E>` implements `Orable<T>`

## Example

```gecko
import std.option use { Option }

func find_user(id: int32): Option<string> {
    if (id == 1) {
        return Option<string>::some("Alice")
    }
    return Option<string>::none()
}

func main(): int {
    // Using or for default value
    let name = find_user(99) or "Unknown"
    
    // Chaining
    let result = find_user(1) or find_user(2) or "Default"
    
    return 0
}
```
