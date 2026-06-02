# Modules

## Design Principles

1. **Explicit over implicit** - No prelude, no auto-imports
2. **No privileged code** - stdlib is just code, not special
3. **Private by default** - Explicit `public` required for exports
4. **Lazy resolution** - Compiler only parses what's actually used
5. **No vendor search path** - Dependencies are resolved via project deps, not `vendor/`

## Package Declaration

Each file declares its package:

```gecko
package mymodule
```

If omitted, defaults to `main`.

## Import Syntax

Dot notation for hierarchical modules:

```gecko
import std.collections.vec
import std.collections.string use { String }
```

### Full Module Import

```gecko
import std.collections.string

// Access via module prefix:
let s = std.collections.string.String::from("hello")
```

### Selective Import

Import specific symbols into current scope:

```gecko
import std.collections.vec use { Vec }
import std.option use { Option }

// Use directly without prefix:
let list: Vec<int32> = Vec<int32>::new()
let maybe: Option<int32> = Option<int32>::some(42)
```

### Directory Import

Import an entire directory as a namespace:

```gecko
import std.collections    // imports stdlib/collections/

// Compiler lazily resolves types from the directory:
let v: Vec<int32> = Vec<int32>::new()      // found in collections/vec.gecko
let s: String = String::from("hello")      // found in collections/string.gecko
```

**Important:** Directory imports are NOT recursive. The compiler only searches immediate children of the imported directory.

## Module Resolution

The compiler searches for modules in this order:

1. **Relative:** `./path/to/module.gecko` or `./path/to/module/mod.gecko`
2. **Project root:** `<project-root>/path/to/module.gecko` or `<project-root>/path/to/module/mod.gecko`
3. **Project deps:** `<project-root>/.gecko/deps/path/to/module.gecko` or `<project-root>/.gecko/deps/path/to/module/mod.gecko`
4. **Stdlib:** `$GECKO_HOME/std/path/to/module.gecko` (falls back to `$GECKO_HOME/stdlib`)

For dot notation `import a.b.c`:
- Translates to path `a/b/c.gecko` or `a/b/c/mod.gecko`

### Environment Variables

| Variable | Purpose | Default |
|----------|---------|---------|
| `GECKO_HOME` | Stdlib location | Compiled-in path |

## Visibility

All declarations are **private by default**.

| Modifier | Accessible From |
|----------|-----------------|
| (none) | Current file only |
| `private` | Current file only |
| `protected` | Current package |
| `public` | Any importing package |
| `external` | Exported with C linkage |

```gecko
package mylib

public class Point {           // exported
    public let x: int32        // field accessible
    public let y: int32
    let internal: int32     // field private to current file
}

public func create_point(x: int32, y: int32): Point {  // exported
    return Point { x: x, y: y, internal: 0 }
}

func helper(): void {       // private to current file
    // ...
}
```

### Visibility Rules

1. A `public` class can have private fields - only `public` fields are accessible externally
2. A private class cannot be returned from a `public` function (type leak error)
3. `external` implies `public` but also exports with C ABI naming

## Error Messages

### Unknown Type with Suggestions

When a type is used but not imported, the compiler:
1. Reports an error
2. Searches all available modules for the type
3. Suggests possible imports

```
error: Unknown type `Vec`
  --> main.gecko:10:12
   |
10 |     let v: Vec<int32>
   |            ^^^

help: `Vec` was found in the following modules:
  - std.collections.vec

Consider adding: import std.collections.vec use { Vec }
```

## Built-in Types

Only C integer types from `<stdint.h>` are available without import:

| Type | C Equivalent |
|------|--------------|
| `int8`, `int16`, `int32`, `int64` | `int8_t`, etc. |
| `uint8`, `uint16`, `uint32`, `uint64` | `uint8_t`, etc. |
| `int`, `uint` | `int64_t`, `uint64_t` |
| `bool` | `_Bool` / `bool` |
| `void` | `void` |
| `string` | `const char*` |

Everything else requires explicit import - including `Vec`, `Option`, `String`, etc.

## C Interop

Import C headers:

```gecko
cimport "stdio.h"
cimport "mylib.h" withobject "mylib.o"
cimport "mylib.h" withlibrary "mylib"
```

Or declare external functions directly:

```gecko
declare external func malloc(size: uint64): void*
declare external func free(ptr: void*): void
```

## Standard Library Structure

```
stdlib/
├── core/
│   ├── traits.gecko      # Trait hooks (Drop, Clone, etc.)
│   └── ops.gecko         # Operator traits (Add, Sub, etc.)
├── collections/
│   ├── vec.gecko         # Vec<T>
│   ├── slice.gecko       # Slice<T>
│   └── string.gecko      # String type
├── memory/
│   ├── box.gecko         # Box<T>
│   ├── rc.gecko          # Rc<T>
│   ├── weak.gecko        # Weak<T>
│   └── raw.gecko         # Raw pointer utilities
├── option.gecko          # Option<T>
└── result.gecko          # Result<T, E>
```

## File Organization

Recommended project structure:

```
project/
├── main.gecko              # package main, entry point
├── utils.gecko             # package utils
├── models/
│   ├── mod.gecko           # package models (re-exports)
│   ├── user.gecko          # internal
│   └── post.gecko          # internal
└── .gecko/
    └── deps/               # resolved project dependencies
```

## No Re-exports

Re-exporting imported symbols is not supported. Instead:

1. Import the directory and let the compiler find types
2. Or import each module explicitly where needed

This avoids "dot notation hell" where types flow through many re-export layers.

## Circular Imports (Planned)

Explicit circular import diagnostics are planned.
Until then, projects should avoid circular import structures and extract shared symbols into a third module.
