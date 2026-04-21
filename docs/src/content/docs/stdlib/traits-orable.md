---
title: Orable (trait)
description: Orable<T> - Trait for types that can use `or` for default values.
---

```gecko
trait Orable<T>
```

Orable<T> - Trait for types that can use `or` for default values.
Used with the `or` keyword: `let val = expr or default`
If the expression has no value, uses the default.

## Required Methods

### unwrap_or

```gecko
func unwrap_or(self: void, default_val: T): T
```

