---
title: Modules & Imports
description: Organizing code with packages and imports in Gecko
sidebar:
  order: 6
---

Gecko uses a module system to organize code into reusable packages.

## Package Declaration

Every Gecko file starts with a package declaration:

```gecko
package mypackage

// ... rest of file
```

The package name is used for symbol namespacing and import paths.

## Import Syntax

### Basic Imports

Import a module using dot notation:

```gecko
import std.collections.vec
import std.collections.string
```

After importing, use the module name to access its contents:

```gecko
let v = vec.Vec::new()
let s = string.String::from("hello")
```

### Selective Imports

Import specific symbols with `use`:

```gecko
import std.collections.vec use { Vec }
import std.collections.string use { String, StringBuilder }
```

Now use them directly without qualification:

```gecko
let v = Vec::new()
let s = String::from("hello")
```

### Multiple Symbols

Import multiple symbols in one statement:

```gecko
import std.core.traits use { Drop, Clone, Iterator }
import std.core.ops use { Add, Eq, Lt }
```

## Standard Library Imports

The standard library uses the `std` prefix:

| Import Path | Contents |
|-------------|----------|
| `std.core.traits` | Drop, Clone, Copy, Iterator, Index, etc. |
| `std.core.ops` | Add, Sub, Mul, Div, Eq, Lt, etc. |
| `std.collections.vec` | Vec<T> |
| `std.collections.string` | String, StringBuilder |
| `std.collections.slice` | Slice<T> |
| `std.memory.box` | Box<T> |
| `std.memory.rc` | Rc<T> |
| `std.memory.weak` | Weak<T> |
| `std.option` | Option<T> |

### Example: Using Collections

```gecko
package main

import std.collections.vec use { Vec }
import std.collections.string use { String }

func main(): void {
    let names: Vec<String> = Vec::new()
    names.push(String::from("Alice"))
    names.push(String::from("Bob"))
    
    for let name in names {
        // process name
    }
}
```

## Local Imports

### Relative Imports

Import from files relative to the current file:

```gecko
// Import ./utils.gecko
import utils

// Import ./helpers/math.gecko
import helpers.math
```

### Directory Imports

Import all public symbols from a directory:

```gecko
// If ./shapes/ contains circle.gecko, rectangle.gecko
import shapes

// Access types with module prefix
let c = shapes.Circle::new(5)
let r = shapes.Rectangle::new(10, 20)
```

## Visibility

By default, symbols are private to their module. Use `public` to export:

```gecko
// utils.gecko
package utils

// Private - only visible in this file
func internal_helper(): void {
    // ...
}

// Public - accessible from other modules
public func format_name(name: string): string {
    return name
}

public class Config {
    public let debug: bool
    let internal_state: int  // private field
}
```

### Importing Public Symbols

Only `public` symbols can be imported:

```gecko
import utils

utils.format_name("test")  // OK
utils.internal_helper()    // Error: not public
```

## C Interoperability

### C Header Import

Import C headers with `cimport`:

```gecko
cimport "stdio.h"

func main(): void {
    printf("Hello from C!\n")
}
```

### With Object Files

Link with compiled C code:

```gecko
cimport "mylib.h" withobject "mylib.o"
```

### With Libraries

Link with system libraries:

```gecko
cimport "math.h" withlibrary "m"

func main(): void {
    let x = sin(3.14159)
}
```

### External Declarations

Declare external C functions explicitly:

```gecko
declare external func malloc(size: uint64): void*
declare external func free(ptr: void*): void
declare external variardic func printf(format: string): int
```

## Module Resolution

Gecko searches for modules in this order:

1. Relative to the current source file
2. Project root (where `gecko.toml` is located)
3. `.gecko/deps/` (installed dependencies)
4. `$GECKO_HOME/stdlib/` (standard library)

## Project Structure

A typical Gecko project:

```
myproject/
├── gecko.toml          # Project configuration
├── src/
│   ├── main.gecko      # Entry point
│   ├── utils.gecko     # Utility module
│   └── models/
│       ├── user.gecko
│       └── post.gecko
└── tests/
    └── test_utils.gecko
```

### gecko.toml

```toml
[package]
name = "myproject"
version = "0.1.0"

[build.entries]
main = "src/main.gecko"

[dependencies]
# somelib = { git = "https://github.com/user/somelib", tag = "v1.0" }
```

## Best Practices

1. **Use selective imports** for clarity about what symbols you're using
2. **Keep modules focused** - one responsibility per module
3. **Export only what's needed** - default to private
4. **Use consistent naming** - module names should be lowercase
5. **Group related imports** - stdlib, then external, then local
