# Types

## Primitive Types

### Integers

| Type | Size | Range |
|------|------|-------|
| `int8` | 8-bit signed | -128 to 127 |
| `int16` | 16-bit signed | -32,768 to 32,767 |
| `int32` | 32-bit signed | -2^31 to 2^31-1 |
| `int64` | 64-bit signed | -2^63 to 2^63-1 |
| `int` | alias for `int32` | |
| `uint8` | 8-bit unsigned | 0 to 255 |
| `uint16` | 16-bit unsigned | 0 to 65,535 |
| `uint32` | 32-bit unsigned | 0 to 2^32-1 |
| `uint64` | 64-bit unsigned | 0 to 2^64-1 |
| `uint` | alias for `uint32` | |

### Other Primitives

| Type | Description |
|------|-------------|
| `bool` | Boolean, `true` or `false` |
| `string` | C string pointer (`const char*`) |
| `void` | No value (for return types) |

## Type References

A type reference specifies a type with optional modifiers:

```
TypeRef = Type [ '<' TypeArgs '>' ] [ 'volatile' ] [ '*' ] [ '!' ]
```

### Pointers

```gecko
let p: int32*           // pointer to int32
let q: int32 volatile*  // pointer to volatile int32
let r: int32*!          // non-null pointer to int32
```

### Const

The `!` suffix after a type (not pointer) marks it as const:

```gecko
let x: int32! = 42  // const int32
```

**Gap**: The const system is inconsistent. `!` is overloaded for both const and non-null.

## Arrays

### Fixed-Size Arrays

```gecko
let buffer: [4096]uint8    // array of 4096 uint8s
let matrix: [16]int32      // array of 16 int32s
```

### Dynamic Arrays

No built-in dynamic array. Use `Vec<T>` from stdlib.

**Gap**: No slice type for referencing portions of arrays.

## Type Inference

The compiler infers types in these contexts:

```gecko
let x = 42              // infers int32
let y = true            // infers bool
let s = "hello"         // infers string
let r = Rect::new(1,2)  // infers Rect from static method return
let a = obj.area()      // infers from method return type
let p = &variable       // infers pointer type
```

**Gap**: No inference for generic type arguments in all contexts.

## Type Casts

Use `as` for explicit casts:

```gecko
let addr = 0xB8000 as uint16*   // integer to pointer
let num = ptr as uint64         // pointer to integer
```

**Gap**: No checked casts. All casts are unchecked like C.

## External Types

Declare opaque types from C:

```gecko
declare external type FILE
```

This creates an incomplete type usable only through pointers.

## Function Types

Function pointer types:

```gecko
let callback: func(int32, int32): int32
let handler: func(uint8*): void
```
