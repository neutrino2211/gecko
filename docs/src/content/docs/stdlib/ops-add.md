---
title: Add (trait)
description: Add<T> - Trait for the `+` operator.
---

```ts
trait Add<T>
```

Add<T> - Trait for the `+` operator.

Types implementing this trait can be added together using `+`.

Example:
```
impl Add<int32> for MyType {
    func add(self, other: int32): int32 {
        return self.value + other
    }
}
```

## Required Methods

### add

```ts
func add(self: void, other: T): T
```

