---
title: Copy (trait)
description: Copy - Marker trait for types that are safe to copy bitwise.
---

```gecko
trait Copy
```

Copy - Marker trait for types that are safe to copy bitwise.
This is a marker trait - types implementing Copy are semantically
safe to duplicate via simple memory copy (like integers, pointers).
No method needed since the copy is implicit.

