# Traits

Traits define interfaces that types can implement.

## Design Principles

1. **No privileged traits** - The compiler doesn't hardcode behavior for any trait name
2. **Hooks via attributes** - Developers wire up compiler features to their own traits
3. **Explicit opt-in** - Syntactic sugar only works when hooks are defined
4. **Pointer operations use intrinsics** - Pointer/null operations are explicit intrinsics, not special traits

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

## Trait Inheritance

Traits can inherit methods/requirements from a parent trait:

```gecko
trait Shape {
    func area(self): int32
}

trait Drawable: Shape {
    func draw(self): void
}
```

Implementing `Drawable` also satisfies `Shape` constraints.

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

## Default Methods and Impl Model

Gecko uses a single trait implementation form:

```gecko
impl TraitName for TypeName {
    // required methods and optional overrides
}
```

There is no separate `default impl` form in the language spec.

### Legacy Compatibility Note

Some compiler versions may still accept `default impl Trait for Type` as legacy syntax.
That form is deprecated and should be migrated to the single impl model:

```gecko
impl Trait for Type {
    // required methods, optional overrides
}
```

### Default Methods

Trait methods with bodies are defaults. Methods without bodies are required.

```gecko
trait Counter {
    func get_value(self): int

    func double_value(self): int {
        return self.get_value() * 2
    }
}

class MyCounter {
    let count: int
}

impl Counter for MyCounter {
    func get_value(self): int {
        return self.count
    }
    // double_value inherited from trait default
}
```

### Required Method Rule

If an impl omits a required method, compilation fails.

```gecko
impl Counter for MyCounter {
    // ^? error: missing required method get_value(self): int
}
```

### Override Rule

An impl method with the same signature as a default trait method overrides that default.

```gecko
impl Counter for MyCounter {
    func get_value(self): int { return self.count }

    func double_value(self): int {
        return 999 // overrides default implementation
    }
}
```

### Ambiguous Method Rule

If two traits implemented by a type provide the same method name, unqualified method calls are ambiguous unless disambiguated.

```gecko
trait A { func show(self): void }
trait B { func show(self): void }

impl A for MyType { func show(self): void { } }
impl B for MyType { func show(self): void { } }

let x: MyType
x.show()
// ^? error: ambiguous method 'show' (A, B)
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

See [Trait Hooks](trait-hooks.md) for hook attributes, operators, indexing,
iteration, and explicit cleanup examples.

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
4. **Defining package only** - Inherent extensions (`impl Type { ... }`) are only allowed in the package where `Type` is defined

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
- Keep related methods near feature code within the same package

## Coherence Rules

Gecko uses explicit coherence rules for trait implementations:

1. **Local inherent impl only** - `impl Type { ... }` requires `Type` to be defined in the current package
2. **Trait impl orphan rule** - `impl Trait for Type` is allowed only if the current package defines `Trait` or `Type`
3. **Foreign-foreign impls are rejected** - If both `Trait` and `Type` are imported from other packages, compilation fails

### Coherence Examples

```gecko
// package math
public class Point {
    public let x: int32
    public let y: int32
}

// Allowed: inherent extension in defining package
impl Point {
    func norm_sq(self): int32 {
        return self.x * self.x + self.y * self.y
    }
}
```

```gecko
// package app
import math use { Point }

// Error: Point is foreign to this package
impl Point {
    func manhattan(self): int32 {
        return self.x + self.y
    }
}
```

```gecko
// package app
import math use { Point }

public trait Renderable {
    func render(self): void
}

// Allowed: local trait on foreign type
impl Renderable for Point {
    func render(self): void {
        // ...
    }
}
```

### Coherence Diagnostics

When coherence rules are violated, the compiler should emit explicit diagnostics:

```text
error: cannot add inherent impl for foreign type `math.Point`
  --> app/main.gecko:5:1
   |
5  | impl Point {
   | ^^^^^^^^^^^
   |
help: inherent impls are only allowed in the defining package `math`
```

```text
error: orphan impl is not allowed: both trait `fmt.Display` and type `math.Point` are foreign
  --> app/main.gecko:8:1
   |
8  | impl Display for Point {
   | ^^^^^^^^^^^^^^^^^^^^^^^
   |
help: define a local trait or wrap the foreign type in a local newtype
```

## Visibility

```gecko
public trait Shape {           // exported
    func area(self): int32
}

trait InternalHelper {      // private to current file
    func helper(self): void
}
```

## Standard Library Traits

The stdlib provides common traits in `std.core.traits`:

```gecko
import std.core.traits use { Drop, Clone, Copy }
```

These are just regular traits with hooks - nothing special about them.
