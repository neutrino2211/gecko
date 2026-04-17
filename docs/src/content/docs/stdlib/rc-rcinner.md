---
title: RcInner
description: Internal structure holding reference counts and value.
---

```ts
class RcInner<T>
```

Internal structure holding reference counts and value.

## Type Parameters

- **T**

## Fields

### strong

```ts
let strong: uint64
```

### weak

```ts
let weak: uint64
```

### value

```ts
let value: T
```

---

*Defined in `stdlib/rc.gecko:6`*
