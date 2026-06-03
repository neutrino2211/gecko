---
title: Buffer
description: Buffer<T> is a typed view over a raw pointer + length.
---

```gecko
class Buffer<T>
```

Buffer<T> is a typed view over a raw pointer + length.

This is the safe-by-default alternative to direct pointer arithmetic.
Pointer movement is explicit via ptr intrinsics inside this abstraction.

## Type Parameters

- **T**

## Fields

### ptr

```gecko
let ptr: T*
```

### length

```gecko
let length: uint64
```

---

*Defined in `stdlib/memory/buffer.gecko:7`*
