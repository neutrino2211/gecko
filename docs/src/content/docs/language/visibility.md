---
title: Visibility
description: Access control and encapsulation in Gecko
sidebar:
  order: 9
---

Gecko uses visibility modifiers to control access to symbols across file and package boundaries.

## Visibility Levels

| Modifier | Scope |
|----------|-------|
| `private` | Same file only (default) |
| `protected` | Same package (any file in package) |
| `public` | Accessible from anywhere |
| `external` | C ABI linkage (implies public) |

## Default Visibility

By default, all symbols are **private** - only accessible within the same file:

```gecko
// utils.gecko
package utils

// Private by default - only accessible in this file
func helper(): void {
    // ...
}

class InternalState {
    let value: int
}
```

## Private Visibility

Use `private` to make intent explicit (same as default):

```gecko
private func internal_calculation(): int {
    return 42
}

private class FileLocalCache {
    let data: string
}
```

## Protected Visibility

Use `protected` to share symbols within a package (across files):

```gecko
// shapes/circle.gecko
package shapes

protected func calculate_area(radius: float64): float64 {
    return 3.14159 * radius * radius
}

protected class ShapeMetrics {
    let area: float64
    let perimeter: float64
}

// shapes/rectangle.gecko
package shapes

func rectangle_area(w: float64, h: float64): float64 {
    // Can access protected symbols from same package
    let metrics = ShapeMetrics { area: w * h, perimeter: 2 * (w + h) }
    return metrics.area
}
```

Protected symbols are **not** visible to subpackages:

```gecko
// shapes/3d/cube.gecko
package shapes.3d

import shapes

func cube_area(): float64 {
    // Error: calculate_area is protected, not accessible from subpackage
    shapes.calculate_area(5.0)
}
```

## Public Visibility

Use `public` to make symbols accessible from anywhere:

```gecko
// utils.gecko
package utils

// Accessible from any module
public func format_string(s: string): string {
    return s
}

public class Config {
    public let debug: bool
    public let log_level: int
}
```

## Class Member Visibility

Class fields and methods can have their own visibility:

```gecko
public class User {
    // Public fields - accessible from anywhere
    public let name: string
    public let email: string
    
    // Protected fields - accessible within package
    protected let account_type: string
    
    // Private fields - only this file
    let password_hash: string
    let internal_id: int
}

impl User {
    // Public constructor
    public func new(name: string, email: string): User {
        return User {
            name: name,
            email: email,
            account_type: "standard",
            password_hash: "",
            internal_id: 0
        }
    }

    // Public method - callable from anywhere
    public func display_name(self): string {
        return self.name
    }
    
    // Protected method - callable within package
    protected func upgrade_account(self, account_type: string): void {
        self.account_type = account_type
    }

    // Private method - only callable within this file
    func hash_password(self, password: string): void {
        self.password_hash = compute_hash(password)
    }
}
```

## Trait Visibility

Traits and their implementations follow the same rules:

```gecko
// Public trait - can be implemented by other modules
public trait Displayable {
    func display(self): string
}

// Private trait - internal to this module
trait InternalFormatter {
    func format_internal(self): string
}
```

## Import Visibility

Only public symbols can be imported:

```gecko
// math.gecko
package math

public func add(a: int, b: int): int {
    return a + b
}

func internal_calc(x: int): int {
    return x * 2
}

// main.gecko
import math

func main(): void {
    math.add(1, 2)           // OK - add is public
    math.internal_calc(5)    // Error - not public
}
```

## Selective Exports

When using selective imports, only public symbols are available:

```gecko
// shapes.gecko
package shapes

public class Circle {
    public let radius: int
}

public class Rectangle {
    public let width: int
    public let height: int
}

class InternalHelper {
    // ...
}

// main.gecko
import shapes use { Circle, Rectangle }  // OK
import shapes use { InternalHelper }     // Error - not public
```

## External Visibility

The `external` modifier is used for C interoperability:

```gecko
// Declare a C function
declare external func printf(format: string): int

// Declare a C struct
external "FILE" class File {
    // fields match C struct layout
}
```

External declarations have C linkage and are accessible across the C ABI boundary.

## Visibility Summary

```
┌─────────────────────────────────────────────────────────┐
│ File A (package foo)    │ File B (package foo)          │
├─────────────────────────┼───────────────────────────────┤
│ private   ✓             │ private   ✗                   │
│ protected ✓             │ protected ✓                   │
│ public    ✓             │ public    ✓                   │
└─────────────────────────┴───────────────────────────────┘

┌─────────────────────────────────────────────────────────┐
│ File C (package bar)    │ File D (package foo.sub)      │
├─────────────────────────┼───────────────────────────────┤
│ private   ✗             │ private   ✗                   │
│ protected ✗             │ protected ✗ (not inherited)   │
│ public    ✓             │ public    ✓                   │
└─────────────────────────┴───────────────────────────────┘
```

## Best Practices

1. **Default to private** - Only expose what's necessary
2. **Use protected for package internals** - Share within package, hide from outside
3. **Use public sparingly** - Only for true public APIs
4. **Document public symbols** - Use doc comments for public APIs
5. **Use explicit private when intentional** - Makes intent clear to readers

```gecko
public class Database {
    // Public API
    public func query(sql: string): Result<Rows, DbError> {
        return self.execute_internal(sql)
    }
    
    public func connect(url: string): Result<void, DbError> {
        return self.init_connection(url)
    }
    
    // Protected - shared within package for testing/extensions
    protected let config: DbConfig
    
    protected func raw_execute(sql: string): Result<Rows, DbError> {
        // ...
    }
    
    // Private implementation details
    private let connection: Connection
    private let cache: Cache
    
    private func execute_internal(sql: string): Result<Rows, DbError> {
        // ...
    }
    
    private func init_connection(url: string): Result<void, DbError> {
        // ...
    }
}
```
