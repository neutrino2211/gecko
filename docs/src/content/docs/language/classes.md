---
title: Classes and Structs
description: Defining custom types in Gecko
sidebar:
  order: 3
---

## Classes

Classes define custom types with fields and methods:

```gecko
class Point {
    let x: int
    let y: int

    func new(x: int, y: int): Point {
        let p: Point
        p.x = x
        p.y = y
        return p
    }

    func distance(self): float64 {
        return sqrt(self.x * self.x + self.y * self.y)
    }
}
```

## Creating Instances

### Static Methods

Methods without `self` are static:

```gecko
let p: Point = Point::new(3, 4)
```

### Struct Literals

```gecko
let p: Point = Point { x: 3, y: 4 }
```

## Instance Methods

Methods with `self` operate on instances:

```gecko
class Counter {
    let value: int

    func new(): Counter {
        let c: Counter
        c.value = 0
        return c
    }

    func increment(self) {
        self.value = self.value + 1
    }

    func get(self): int {
        return self.value
    }
}

// Usage
let c: Counter = Counter::new()
c.increment()
c.increment()
let val: int = c.get()  // 2
```

## Field Access

```gecko
let p: Point = Point::new(3, 4)
let x: int = p.x        // Read field
p.x = 10                // Write field
```

## Generics

Classes can have type parameters:

```gecko
class Box<T> {
    let value: T

    func new(v: T): Box<T> {
        let b: Box<T>
        b.value = v
        return b
    }

    func get(self): T {
        return self.value
    }
}

// Usage
let b: Box<int> = Box<int>::new(42)
let val: int = b.get()
```

## Attributes

### Packed Structs

Remove padding for memory layout control:

```gecko
@packed
class PackedData {
    let a: uint8
    let b: uint32
    let c: uint8
}
```

### Section Placement

Place data in specific memory sections:

```gecko
@section(".rodata")
const MESSAGE: string = "Hello"
```

## External Types

Declare opaque types from C:

```gecko
declare external type FILE
```
