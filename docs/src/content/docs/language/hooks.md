---
title: Hooks & Operators
description: Customizing operators and language features with hook attributes
sidebar:
  order: 7
---

Gecko uses hook attributes to enable operator overloading and other syntactic sugar. When you define a trait with a hook, implementing that trait enables special syntax for your types.

## Operator Hooks

### Arithmetic Operators

```gecko
@add_hook(.add)
trait Add<T> {
    func add(self, other: T): T
}

@sub_hook(.sub)
trait Sub<T> {
    func sub(self, other: T): T
}

@mul_hook(.mul)
trait Mul<T> {
    func mul(self, other: T): T
}

@div_hook(.div)
trait Div<T> {
    func div(self, other: T): T
}
```

**Example: Vector arithmetic**

```gecko
import std.core.ops use { Add, Sub }

class Vec2 {
    let x: float64
    let y: float64
}

impl Vec2 {
    func new(x: float64, y: float64): Vec2 {
        return Vec2 { x: x, y: y }
    }
}

impl Add<Vec2> for Vec2 {
    func add(self, other: Vec2): Vec2 {
        return Vec2::new(self.x + other.x, self.y + other.y)
    }
}

impl Sub<Vec2> for Vec2 {
    func sub(self, other: Vec2): Vec2 {
        return Vec2::new(self.x - other.x, self.y - other.y)
    }
}

// Usage
let a = Vec2::new(1.0, 2.0)
let b = Vec2::new(3.0, 4.0)
let c = a + b  // Vec2 { x: 4.0, y: 6.0 }
let d = a - b  // Vec2 { x: -2.0, y: -2.0 }
```

### Comparison Operators

```gecko
@eq_hook(.eq)
trait Eq<T> {
    func eq(self, other: T): bool
}

@ne_hook(.ne)
trait Ne<T> {
    func ne(self, other: T): bool
}

@lt_hook(.lt)
trait Lt<T> {
    func lt(self, other: T): bool
}

@gt_hook(.gt)
trait Gt<T> {
    func gt(self, other: T): bool
}

@le_hook(.le)
trait Le<T> {
    func le(self, other: T): bool
}

@ge_hook(.ge)
trait Ge<T> {
    func ge(self, other: T): bool
}
```

**Example: Comparable type**

```gecko
import std.core.ops use { Eq, Lt }

class Version {
    let major: int
    let minor: int
}

impl Eq<Version> for Version {
    func eq(self, other: Version): bool {
        return self.major == other.major && self.minor == other.minor
    }
}

impl Lt<Version> for Version {
    func lt(self, other: Version): bool {
        if self.major < other.major {
            return true
        }
        if self.major == other.major {
            return self.minor < other.minor
        }
        return false
    }
}

// Usage
let v1 = Version { major: 1, minor: 0 }
let v2 = Version { major: 2, minor: 0 }
if v1 < v2 {
    // v1 is older
}
```

### Bitwise Operators

```gecko
@bitand_hook(.bitand)
trait BitAnd<T> {
    func bitand(self, other: T): T
}

@bitor_hook(.bitor)
trait BitOr<T> {
    func bitor(self, other: T): T
}

@bitxor_hook(.bitxor)
trait BitXor<T> {
    func bitxor(self, other: T): T
}

@shl_hook(.shl)
trait Shl<T> {
    func shl(self, other: T): T
}

@shr_hook(.shr)
trait Shr<T> {
    func shr(self, other: T): T
}
```

### Unary Operators

```gecko
@neg_hook(.neg)
trait Neg {
    func neg(self): Self
}

@not_hook(.not)
trait Not {
    func not(self): Self
}
```

**Example: Negatable vector**

```gecko
import std.core.ops use { Neg }

impl Neg for Vec2 {
    func neg(self): Vec2 {
        return Vec2::new(-self.x, -self.y)
    }
}

// Usage
let v = Vec2::new(1.0, 2.0)
let negated = -v  // Vec2 { x: -1.0, y: -2.0 }
```

## Index Hooks

Enable array-like access with `[]`:

```gecko
@index_hook(.index)
trait Index<I, T> {
    func index(self, idx: I): T
}

@index_mut_hook(.index_mut)
trait IndexMut<I, T> {
    func index_mut(self, idx: I, value: T): void
}
```

**Example: Custom array type**

```gecko
import std.core.traits use { Index, IndexMut }

class MyArray {
    let data: int*
    let len: uint64
}

impl Index<uint64, int> for MyArray {
    func index(self, idx: uint64): int {
        return @deref(self.data + idx)
    }
}

impl IndexMut<uint64, int> for MyArray {
    func index_mut(self, idx: uint64, value: int): void {
        @deref(self.data + idx) = value
    }
}

// Usage
let arr: MyArray = // ...
let x = arr[0]     // calls index(arr, 0)
arr[1] = 42        // calls index_mut(arr, 1, 42)
```

## Iterator Hooks

Enable `for-in` loops:

```gecko
@iterator_hook(.next, .has_next)
trait Iterator<T> {
    func next(self): T
    func has_next(self): bool
}

@into_iterator_hook(.iter)
trait IntoIterator<T> {
    func iter(self): Iterator<T>
}
```

**Example: Range iterator**

```gecko
import std.core.traits use { Iterator }

class Range {
    let current: int
    let end: int
}

impl Range {
    func new(start: int, end: int): Range {
        return Range { current: start, end: end }
    }
}

impl Iterator<int> for Range {
    func next(self): int {
        let val = self.current
        self.current = self.current + 1
        return val
    }

    func has_next(self): bool {
        return self.current < self.end
    }
}

// Usage
for let i in Range::new(0, 10) {
    // i goes from 0 to 9
}
```

## Drop Hook

Automatic cleanup when values go out of scope:

```gecko
@drop_hook(.drop)
trait Drop {
    func drop(self): void
}
```

**Example: Resource management**

```gecko
import std.core.traits use { Drop }

class FileHandle {
    let fd: int
}

impl FileHandle {
    func open(path: string): FileHandle {
        let fd = // ... open file
        return FileHandle { fd: fd }
    }
}

impl Drop for FileHandle {
    func drop(self): void {
        close(self.fd)  // Automatically called when out of scope
    }
}

func process_file(): void {
    let file = FileHandle::open("data.txt")
    // ... use file
}  // file.drop() called here automatically
```

## Complete Operator Reference

| Operator | Trait | Hook Attribute | Method |
|----------|-------|----------------|--------|
| `+` | `Add<T>` | `@add_hook(.add)` | `add(self, other: T): T` |
| `-` | `Sub<T>` | `@sub_hook(.sub)` | `sub(self, other: T): T` |
| `*` | `Mul<T>` | `@mul_hook(.mul)` | `mul(self, other: T): T` |
| `/` | `Div<T>` | `@div_hook(.div)` | `div(self, other: T): T` |
| `==` | `Eq<T>` | `@eq_hook(.eq)` | `eq(self, other: T): bool` |
| `!=` | `Ne<T>` | `@ne_hook(.ne)` | `ne(self, other: T): bool` |
| `<` | `Lt<T>` | `@lt_hook(.lt)` | `lt(self, other: T): bool` |
| `>` | `Gt<T>` | `@gt_hook(.gt)` | `gt(self, other: T): bool` |
| `<=` | `Le<T>` | `@le_hook(.le)` | `le(self, other: T): bool` |
| `>=` | `Ge<T>` | `@ge_hook(.ge)` | `ge(self, other: T): bool` |
| `&` | `BitAnd<T>` | `@bitand_hook(.bitand)` | `bitand(self, other: T): T` |
| `\|` | `BitOr<T>` | `@bitor_hook(.bitor)` | `bitor(self, other: T): T` |
| `^` | `BitXor<T>` | `@bitxor_hook(.bitxor)` | `bitxor(self, other: T): T` |
| `<<` | `Shl<T>` | `@shl_hook(.shl)` | `shl(self, other: T): T` |
| `>>` | `Shr<T>` | `@shr_hook(.shr)` | `shr(self, other: T): T` |
| unary `-` | `Neg` | `@neg_hook(.neg)` | `neg(self): Self` |
| unary `!` | `Not` | `@not_hook(.not)` | `not(self): Self` |
| `[]` read | `Index<I, T>` | `@index_hook(.index)` | `index(self, idx: I): T` |
| `[]` write | `IndexMut<I, T>` | `@index_mut_hook(.index_mut)` | `index_mut(self, idx: I, val: T): void` |
| `for-in` | `Iterator<T>` | `@iterator_hook(.next, .has_next)` | `next(self): T`, `has_next(self): bool` |
| scope exit | `Drop` | `@drop_hook(.drop)` | `drop(self): void` |

## Defining Your Own Hooks

Hooks are defined on traits using the `@*_hook` attributes. When you implement a hooked trait, the compiler automatically generates the operator support.

```gecko
// The standard library defines this:
@add_hook(.add)
trait Add<T> {
    func add(self, other: T): T
}

// When you implement it:
impl Add<MyType> for MyType {
    func add(self, other: MyType): MyType {
        // your implementation
    }
}

// The compiler enables:
let result = a + b  // becomes a.add(b)
```
