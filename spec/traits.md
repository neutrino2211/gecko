# Traits

Traits define interfaces that types can implement.

## Design Principles

1. **No privileged traits** - The compiler doesn't hardcode behavior for any trait name
2. **Hooks via attributes** - Developers wire up compiler features to their own traits
3. **Explicit opt-in** - Syntactic sugar only works when hooks are defined

## Declaration Syntax

```gecko
trait Name[<TypeParams>] {
    func method(self, params...): ReturnType
    func static_method(params...): ReturnType
}
```

## Basic Trait

```gecko
trait Shape {
    func area(self): int32
    func perimeter(self): int32
}
```

## Implementation

```gecko
class Rectangle {
    let width: int32
    let height: int32
}

impl Shape for Rectangle {
    func area(self): int32 {
        return self.width * self.height
    }
    
    func perimeter(self): int32 {
        return 2 * (self.width + self.height)
    }
}
```

## Static Trait Methods

Methods without `self`:

```gecko
trait Default {
    func default_val(): Self
}

impl Default for Point {
    func default_val(): Point {
        return Point { x: 0, y: 0 }
    }
}

// Called as:
let p: Point = Point::default_val()
```

## Generic Traits

```gecko
trait Add<T> {
    func add(self, other: T): T
}

impl Add<int32> for int32 {
    func add(self, other: int32): int32 {
        return self + other
    }
}
```

## Trait Constraints

Constrain generic type parameters:

```gecko
func process<T is Shape>(shape: T*): int32 {
    return shape.area()
}
```

## Trait Hooks

Trait hooks connect compiler features to user-defined traits. The compiler provides *capabilities*, developers wire them up.

### Hook Attributes

```gecko
// Cleanup hook - compiler calls .drop() when value goes out of scope
@drop_hook(.drop)
trait Drop {
    func drop(self): void
}

// Copy hook - compiler uses .copy() for implicit copies
@copy_hook(.copy)
trait Copy {
    func copy(self): Self
}

// Clone hook - compiler uses .clone() for explicit cloning
@clone_hook(.clone)  
trait Clone {
    func clone(self): Self
}
```

### Operator Hooks

```gecko
// Arithmetic
@add_hook(.add)      trait Add<T> { func add(self, other: T): T }
@sub_hook(.sub)      trait Sub<T> { func sub(self, other: T): T }
@mul_hook(.mul)      trait Mul<T> { func mul(self, other: T): T }
@div_hook(.div)      trait Div<T> { func div(self, other: T): T }
@neg_hook(.neg)      trait Neg { func neg(self): Self }

// Comparison
@eq_hook(.eq)        trait Eq<T> { func eq(self, other: T): bool }
@ne_hook(.ne)        trait Ne<T> { func ne(self, other: T): bool }
@lt_hook(.lt)        trait Lt<T> { func lt(self, other: T): bool }
@gt_hook(.gt)        trait Gt<T> { func gt(self, other: T): bool }
@le_hook(.le)        trait Le<T> { func le(self, other: T): bool }
@ge_hook(.ge)        trait Ge<T> { func ge(self, other: T): bool }

// Bitwise
@bitand_hook(.bitand)  trait BitAnd<T> { func bitand(self, other: T): T }
@bitor_hook(.bitor)    trait BitOr<T> { func bitor(self, other: T): T }
@bitxor_hook(.bitxor)  trait BitXor<T> { func bitxor(self, other: T): T }
@shl_hook(.shl)        trait Shl<T> { func shl(self, other: T): T }
@shr_hook(.shr)        trait Shr<T> { func shr(self, other: T): T }
```

### Indexing Hooks

```gecko
// arr[i] read access
@index_hook(.get)
trait Index<I, T> {
    func get(self, index: I): T
}

// arr[i] = val write access
@index_mut_hook(.set)
trait IndexMut<I, T> {
    func set(self, index: I, value: T): void
}
```

### Iterator Hooks

```gecko
// for-loop desugaring
@iterator_hook(.next, .has_next)
trait Iterator<T> {
    func next(self): T
    func has_next(self): bool
}

// for x in collection { } desugaring
@into_iterator_hook(.iter)
trait IntoIterator<T> {
    func iter(self): Iterator<T>
}
```

### Hook Rules

1. **One hook per capability** - Only one trait can be registered per hook type
2. **Signature verification** - Compiler verifies trait matches expected signature
3. **No hook = no sugar** - If `@add_hook` isn't defined, `+` only works for primitives
4. **Scope-local** - Hooks are active within the module that defines them and importers

### Example: Complete Drop Implementation

```gecko
package mymodule

// Define the trait with the hook
@drop_hook(.drop)
public trait Drop {
    func drop(self): void
}

// Implement for a type
public class FileHandle {
    let fd: int32
}

impl Drop for FileHandle {
    func drop(self): void {
        // Close the file descriptor
        close(self.fd)
    }
}

// Usage - compiler inserts .drop() call at scope exit
func process_file(): void {
    let f: FileHandle = open_file("data.txt")
    // ... use f ...
}   // <- compiler calls f.drop() here
```

### Opting Out

Projects that don't want automatic behavior simply don't define hooks:

```gecko
// Kernel code - no automatic cleanup
package kernel

trait Cleanup {
    func cleanup(self): void
}

// Must call .cleanup() manually - no compiler magic
impl Cleanup for Buffer {
    func cleanup(self): void {
        kfree(self.ptr)
    }
}
```

## Self Type

Within trait methods, `Self` refers to the implementing type:

```gecko
trait Clone {
    func clone(self): Self
}

impl Clone for Point {
    func clone(self): Point {  // Self = Point
        return Point { x: self.x, y: self.y }
    }
}
```

## Inherent Implementations (Extensions)

Add methods to a class without a trait, similar to Swift extensions:

```gecko
impl Rectangle {
    func scale(self, factor: int32): void {
        self.width = self.width * factor
        self.height = self.height * factor
    }
    
    func new(w: int32, h: int32): Rectangle {
        return Rectangle { width: w, height: h }
    }
}
```

### Extension Rules

1. **Methods only** - Extensions cannot add fields, only methods
2. **Add only** - Extensions cannot override existing methods (compile error if duplicate)
3. **Any file** - Extensions can be in a different file than the class
4. **Same module** - Extensions must be in the same module (or importing module)

### Classes vs Extensions

Classes can define methods inline or via extensions:

```gecko
// Option 1: Methods inside class
class Point {
    let x: int32
    let y: int32
    
    func sum(self): int32 {
        return self.x + self.y
    }
}

// Option 2: Methods via extension
class Point {
    let x: int32
    let y: int32
}

impl Point {
    func sum(self): int32 {
        return self.x + self.y
    }
}
```

Both are equivalent. Use extensions to:
- Organize code by functionality
- Add methods in separate files
- Extend types from other modules

## Visibility

```gecko
public trait Shape {           // exported
    func area(self): int32
}

trait InternalHelper {      // private to module
    func helper(self): void
}
```

## Standard Library Traits

The stdlib provides common traits in `std.core.traits`:

```gecko
import std.core.traits use { Drop, Clone, Copy }
```

These are just regular traits with hooks - nothing special about them.
