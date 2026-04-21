---
title: Tryable (trait)
description: Tryable<T> - Trait for types that can be "tried" (unwrap or propagate).
---

```gecko
trait Tryable<T>
```

Tryable<T> - Trait for types that can be "tried" (unwrap or propagate).
Used with the `try` keyword: `let val = try expr`
If the expression has no value, propagates the error by early return.
The enclosing function must also return a Tryable type.

## Required Methods

### has_value

```gecko
func has_value(self: void): bool
```

### try_unwrap

```gecko
func try_unwrap(self: void): T
```

