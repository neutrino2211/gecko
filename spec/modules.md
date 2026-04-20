# Modules

## Design Principles

1. **Explicit over implicit** - No prelude, no auto-imports
2. **No privileged code** - stdlib is just code, not special
3. **Private by default** - Explicit `public` required for exports
4. **Lazy resolution** - Compiler only parses what's actually used

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
import std.collections.hash use { HashMap }
```

### Full Module Import

```gecko
import std.math

// Access via module prefix:
let result: float64 = std.math.sqrt(16.0)
```

### Selective Import

Import specific symbols into current scope:

```gecko
import std.collections.vec use { Vec }
import std.option use { Option, Some, None }

// Use directly without prefix:
let list: Vec<int32> = Vec<int32>::new()
let maybe: Option<int32> = Some(42)
```

### Directory Import

Import an entire directory as a namespace:

```gecko
import std.collections    // imports stdlib/collections/

// Compiler lazily resolves types from the directory:
let v: Vec<int32> = Vec<int32>::new()      // found in collections/vec.gecko
let s: HashSet<int32> = HashSet<int32>::new()  // found in collections/hash.gecko
```

**Important:** Directory imports are NOT recursive. `import std.collections` does not include `std.collections.hash`. The compiler only searches immediate children of the directory.

## Module Resolution

The compiler searches for modules in this order:

1. **Relative:** `./path/to/module.gecko` or `./path/to/module/mod.gecko`
2. **Stdlib:** `$GECKO_HOME/stdlib/path/to/module.gecko`
3. **Vendor:** `./vendor/path/to/module.gecko` (future)

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
| (none) | Same module only |
| `public` | Any importing module |
| `external` | Exported with C linkage |

```gecko
package mylib

public class Point {           // exported
    public let x: int32        // field accessible
    public let y: int32
    let internal: int32     // field private to module
}

public func create_point(x: int32, y: int32): Point {  // exported
    return Point { x: x, y: y, internal: 0 }
}

func helper(): void {       // private to module
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
error: Unknown type `HashMap`
  --> main.gecko:10:12
   |
10 |     let m: HashMap<string, int32>
   |            ^^^^^^^

help: `HashMap` was found in the following modules:
  - std.collections.hash
  - vendor.custom_maps

Consider adding: import std.collections.hash use { HashMap }
```

## Built-in Types

Only C integer types from `<stdint.h>` are available without import:

| Type | C Equivalent |
|------|--------------|
| `int8`, `int16`, `int32`, `int64` | `int8_t`, etc. |
| `uint8`, `uint16`, `uint32`, `uint64` | `uint8_t`, etc. |
| `int`, `uint` | Platform-native `int`, `unsigned int` |
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
│   ├── hash.gecko        # HashMap, HashSet
│   └── string.gecko      # String type
├── memory/
│   ├── box.gecko         # Box<T>
│   ├── rc.gecko          # Rc<T>
│   └── raw.gecko         # Raw pointer utilities
├── option.gecko          # Option<T>
├── result.gecko          # Result<T, E>
└── io.gecko              # I/O (hosted only)
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
└── vendor/                 # third-party code
    └── json/
        └── mod.gecko
```

## No Re-exports

Re-exporting imported symbols is not supported. Instead:

1. Import the directory and let the compiler find types
2. Or import each module explicitly where needed

This avoids "dot notation hell" where types flow through many re-export layers.

## Circular Imports

Circular imports are a compile error:

```
error: Circular import detected
  --> a.gecko:1:1
   |
   = note: a.gecko -> b.gecko -> a.gecko
```

Refactor by extracting shared types into a third module.
