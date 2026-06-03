---
title: Traits
description: Interfaces and polymorphism in Gecko
sidebar:
  order: 4
---

## Defining Traits

Traits define interfaces that types can implement:

```gecko
trait Printable {
    func print(self): void
}
```

## Trait Inheritance

Traits can inherit required methods and defaults from parent traits:

```gecko
trait Shape {
    func area(self): int32
}

trait RectangleLike: Shape {
    func perimeter(self): int32
}
```

Implementing `RectangleLike` also satisfies `Shape` constraints. Inheritance is transitive (`Top: Middle` and `Middle: Core` means `Top` also includes `Core`).

## Implementing Traits

Use `impl` to implement a trait for a class:

```gecko
class Point {
    let x: int
    let y: int
}

impl Printable for Point {
    func print(self): void {
        printf("Point(%d, %d)\n", self.x, self.y)
    }
}
```

If an inherited method name is redeclared with a different signature, compilation fails with a trait inheritance error.

## Generic Traits

Traits can have type parameters:

```gecko
trait Add<T> {
    func add(self, other: T): T
}

impl Add<Point> for Point {
    func add(self, other: Point): Point {
        let result: Point
        result.x = self.x + other.x
        result.y = self.y + other.y
        return result
    }
}
```

## Default Methods

Trait methods with bodies are inherited automatically:

```gecko
trait Counter {
    func value(self): int32

    func twice(self): int32 {
        return self.value() * 2
    }
}
```

Implementations only need to provide required methods (the ones without bodies).

## Trait Constraints

Constrain generic types to require trait implementations:

```gecko
func print_twice<T is Printable>(item: T) {
    item.print()
    item.print()
}
```

Constraints against parent traits also work when a type implements a child trait.

## Operator Overloading

Implement operator traits for custom operators:

| Trait | Operator |
|-------|----------|
| `Add<T>` | `+` |
| `Sub<T>` | `-` |
| `Mul<T>` | `*` |
| `Div<T>` | `/` |
| `Eq<T>` | `==` |
| `Ne<T>` | `!=` |
| `Lt<T>` | `<` |
| `Gt<T>` | `>` |
| `Le<T>` | `<=` |
| `Ge<T>` | `>=` |
| `BitAnd<T>` | `&` |
| `BitOr<T>` | `|` |
| `BitXor<T>` | `^` |
| `Shl<T>` | `<<` |
| `Shr<T>` | `>>` |
| `Neg` | unary `-` |
| `Not` | unary `!` |

### Example: Vector Addition

```gecko
import std.core.ops use { Add }

class Vec2 {
    let x: int32
    let y: int32

    func new(x: int32, y: int32): Vec2 {
        let v: Vec2
        v.x = x
        v.y = y
        return v
    }
}

impl Add<Vec2> for Vec2 {
    func add(self, other: Vec2): Vec2 {
        return Vec2::new(self.x + other.x, self.y + other.y)
    }
}

// Usage
let a: Vec2 = Vec2::new(1, 2)
let b: Vec2 = Vec2::new(3, 4)
let c: Vec2 = a + b  // Vec2 { x: 4, y: 6 }
```

## Core Traits

The `core` package provides fundamental traits:

| Trait | Purpose |
|-------|---------|
| `Clone` | Create a copy of a value |
| `Copy` | Bitwise copyable types |
| `Drop` | Cleanup when value goes out of scope |
| `Default` | Default value for a type |
| `From<T>` | Convert from type T |
| `Into<T>` | Convert into type T |
| `Hash` | Compute hash value |
| `Debug` | Debug formatting |
| `Display` | User-facing formatting |
| `Iterator<T>` | Produce a sequence of values |
| `Index<I, T>` | Indexing with `[]` |
