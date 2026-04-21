---
title: Visibility
description: Access control and encapsulation in Gecko
sidebar:
  order: 9
---

Gecko uses visibility modifiers to control access to symbols across module boundaries.

## Default Visibility

By default, all symbols are **private** to their module:

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

## Public Visibility

Use `public` to make symbols accessible from other modules:

```gecko
// utils.gecko
package utils

// Accessible from other modules
public func format_string(s: string): string {
    return s
}

public class Config {
    public let debug: bool
    public let log_level: int
}
```

## Visibility Modifiers

| Modifier | Scope |
|----------|-------|
| (none) | Private - same file only |
| `public` | Accessible from any module |
| `private` | Explicit private (same as default) |
| `protected` | Reserved for future use |
| `external` | C ABI linkage |

## Class Member Visibility

Class fields and methods can have their own visibility:

```gecko
public class User {
    // Public fields - accessible from outside
    public let name: string
    public let email: string
    
    // Private fields - internal use only
    let password_hash: string
    let internal_id: int
}

impl User {
    // Public constructor
    public func new(name: string, email: string): User {
        return User {
            name: name,
            email: email,
            password_hash: "",
            internal_id: 0
        }
    }

    // Public method
    public func display_name(self): string {
        return self.name
    }

    // Private method - only callable within this module
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

## Best Practices

1. **Default to private** - Only expose what's necessary
2. **Public API, private implementation** - Keep internal details hidden
3. **Document public symbols** - Use doc comments for public APIs
4. **Group related public symbols** - Make the public API cohesive
5. **Use explicit private when intentional** - Makes intent clear to readers

```gecko
public class Database {
    // Public API
    public func query(sql: string): Result {
        return self.execute_internal(sql)
    }
    
    public func connect(url: string): bool {
        return self.init_connection(url)
    }
    
    // Private implementation details
    private let connection: Connection
    private let cache: Cache
    
    private func execute_internal(sql: string): Result {
        // ...
    }
    
    private func init_connection(url: string): bool {
        // ...
    }
}
```
