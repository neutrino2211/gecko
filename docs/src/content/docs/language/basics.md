---
title: Language Basics
description: Variables, types, and functions in Gecko
sidebar:
  order: 1
---

## Variables

Declare variables with `let` (mutable) or `const` (immutable):

```gecko
let x: int = 42          // Mutable, explicit type
let y = 42               // Mutable, type inferred as int
const PI: float64 = 3.14 // Immutable
```

## Primitive Types

| Type | Description |
|------|-------------|
| `int` | Platform-sized signed integer |
| `int8`, `int16`, `int32`, `int64` | Sized signed integers |
| `uint` | Platform-sized unsigned integer |
| `uint8`, `uint16`, `uint32`, `uint64` | Sized unsigned integers |
| `float32`, `float64` | Floating point numbers |
| `bool` | Boolean (`true` or `false`) |
| `string` | String literal (pointer to null-terminated bytes) |
| `void` | No value |

## Functions

```gecko
func add(a: int, b: int): int {
    return a + b
}

func greet(name: string) {
    printf("Hello, %s!\n", name)
}
```

### Variadic Functions

```gecko
declare external variadic func printf(format: string): int
```

## Operators

### Arithmetic
`+`, `-`, `*`, `/`

### Comparison
`==`, `!=`, `<`, `>`, `<=`, `>=`

### Logical
`&&`, `||`, `!`

### Bitwise
`&`, `|`, `^`, `<<`, `>>`

## Type Inference

Gecko infers types from context:

```gecko
let x = 42              // int
let y = 3.14            // float64
let z = true            // bool
let s = "hello"         // string
let result = add(1, 2)  // int (from function return type)
```

## Comments

```gecko
// Single line comment

/// Doc comment for the following item
func documented(): void {}
```

## Arrays

### Fixed-size Arrays

```gecko
let arr: [4]int = [1, 2, 3, 4]
let first: int = arr[0]
arr[1] = 10
```

### Array Types

```gecko
func sum(data: [4]int): int {
    return data[0] + data[1] + data[2] + data[3]
}
```
