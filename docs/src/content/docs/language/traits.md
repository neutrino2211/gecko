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

## Trait Constraints

Constrain generic types to require trait implementations:

```gecko
func print_twice<T is Printable>(item: T) {
    item.print()
    item.print()
}
```

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
import core

class Vec2 {
    let x: float64
    let y: float64

    func new(x: float64, y: float64): Vec2 {
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
let a: Vec2 = Vec2::new(1.0, 2.0)
let b: Vec2 = Vec2::new(3.0, 4.0)
let c: Vec2 = a + b  // Vec2 { x: 4.0, y: 6.0 }
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
