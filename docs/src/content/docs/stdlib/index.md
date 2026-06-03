---
title: Standard Library
description: Module-oriented Gecko standard library reference
---

Gecko stdlib docs are organized by **module path**.
Navigate by package/module first, then by exported symbols, similar to a module-first API browser.

## Module Tree

```text
std
├── core
│   ├── traits
│   └── ops
├── collections
│   ├── vec
│   ├── slice
│   └── string
├── memory
│   ├── box
│   ├── buffer
│   ├── raw
│   ├── rc
│   └── weak
├── option
└── result
```

## Modules

### `std.core`

- [`std.core.traits`](/stdlib/std-core-traits/) - Foundational traits used across the language.
- [`std.core.ops`](/stdlib/std-core-ops/) - Operator traits used by hooks and overloaded operators.

### `std.collections`

- [`std.collections.vec`](/stdlib/std-collections-vec/) - Dynamic array type.
- [`std.collections.slice`](/stdlib/std-collections-slice/) - Borrowed view over contiguous data.
- [`std.collections.string`](/stdlib/std-collections-string/) - String and string-building utilities.

### `std.memory`

- [`std.memory.box`](/stdlib/std-memory-box/) - Unique ownership smart pointer.
- [`std.memory.buffer`](/stdlib/std-memory-buffer/) - Typed pointer+length view.
- [`std.memory.raw`](/stdlib/std-memory-raw/) - Low-level raw pointer wrapper.
- [`std.memory.rc`](/stdlib/std-memory-rc/) - Reference-counted shared ownership.
- [`std.memory.weak`](/stdlib/std-memory-weak/) - Non-owning weak references.

### Top-Level Modules

- [`std.option`](/stdlib/std-option/) - Optional value type.
- [`std.result`](/stdlib/std-result/) - Success/error result type.
