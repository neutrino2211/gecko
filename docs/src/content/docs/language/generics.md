---
title: Generics
description: Type parameters and constraints in Gecko
sidebar:
  order: 5
---

Generics allow you to write code that works with multiple types while maintaining type safety. Gecko uses angle brackets `<T>` for type parameters.

## Generic Functions

Define functions that work with any type:

```gecko
func identity<T>(value: T): T {
    return value
}

// Usage - type is inferred
let x = identity(42)        // x: int
let s = identity("hello")   // s: string
```

### Multiple Type Parameters

Functions can have multiple type parameters:

```gecko
func swap<A, B>(pair: Pair<A, B>): Pair<B, A> {
    return Pair { first: pair.second, second: pair.first }
}
```

## Generic Classes

Classes can have type parameters:

```gecko
class Box<T> {
    let value: T
}

impl Box {
    func new(value: T): Box<T> {
        return Box { value: value }
    }

    func get(self): T {
        return self.value
    }

    func set(self, value: T): void {
        self.value = value
    }
}

// Usage
let intBox = Box::new(42)
let strBox = Box::new("hello")
```

### Generic Class with Multiple Parameters

```gecko
class Pair<A, B> {
    let first: A
    let second: B
}

impl Pair {
    func new(first: A, second: B): Pair<A, B> {
        return Pair { first: first, second: second }
    }
}

let pair = Pair::new(1, "one")  // Pair<int, string>
```

## Trait Constraints

Constrain type parameters to types implementing specific traits:

```gecko
trait Printable {
    func print(self): void
}

// T must implement Printable
func print_twice<T is Printable>(item: T): void {
    item.print()
    item.print()
}
```

### Multiple Constraints

Use `&` to require multiple traits:

```gecko
trait Clone {
    func clone(self): Self
}

trait Debug {
    func debug(self): void
}

// T must implement both Clone and Debug
func clone_and_debug<T is Clone & Debug>(item: T): T {
    item.debug()
    return item.clone()
}
```

## Generic Traits

Traits themselves can have type parameters:

```gecko
trait Container<T> {
    func get(self): T
    func set(self, value: T): void
}

class Wrapper<T> {
    let inner: T
}

impl<T> Container<T> for Wrapper<T> {
    func get(self): T {
        return self.inner
    }

    func set(self, value: T): void {
        self.inner = value
    }
}
```

## Generic Implementations

Implement traits for generic types:

```gecko
// Implement Clone for any Box<T> where T is Clone
impl<T is Clone> Clone for Box<T> {
    func clone(self): Box<T> {
        return Box::new(self.value.clone())
    }
}
```

## Type Inference

Gecko can often infer generic type arguments:

```gecko
let v = Vec::new()      // Vec<???> - type unknown until used
v.push(42)              // Now Vec<int>

let s = String::from("hello")  // Type inferred from argument
```

### Explicit Type Arguments

When inference isn't possible, specify types explicitly:

```gecko
let empty: Vec<int> = Vec::new()

// Or using turbofish syntax in some contexts
let result = parse::<int>("42")
```

## Common Generic Patterns

### Option Type

```gecko
class Option<T> {
    let has_value: bool
    let value: T
}

impl Option {
    func some(value: T): Option<T> {
        return Option { has_value: true, value: value }
    }

    func none(): Option<T> {
        let opt: Option<T>
        opt.has_value = false
        return opt
    }

    func is_some(self): bool {
        return self.has_value
    }

    func unwrap(self): T {
        return self.value
    }

    func unwrap_or(self, default: T): T {
        if self.has_value {
            return self.value
        }
        return default
    }
}
```

### Generic Collections

```gecko
import std.collections.vec use { Vec }

func sum<T is Add<T>>(items: Vec<T>, zero: T): T {
    let total = zero
    for let item in items {
        total = total + item
    }
    return total
}
```

## Limitations

- Gecko uses monomorphization: each generic instantiation creates specialized code
- Recursive generic types must use pointers to avoid infinite size
- Generic type parameters are erased at runtime (no reflection)
