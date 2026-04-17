---
title: TryFrom (trait)
description: TryFrom<T> - Fallible conversion from type T.
---

```ts
trait TryFrom<T>
```

TryFrom<T> - Fallible conversion from type T.
Returns an error indicator (0 = success, non-zero = error code).

## Required Methods

### try_from

```ts
func try_from(value: T, out: Self*): int32
```

