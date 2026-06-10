# Types

## Primitive Types

### Integers

| Type | Size | Range |
|------|------|-------|
| `int8` | 8-bit signed | -128 to 127 |
| `int16` | 16-bit signed | -32,768 to 32,767 |
| `int32` | 32-bit signed | -2^31 to 2^31-1 |
| `int64` | 64-bit signed | -2^63 to 2^63-1 |
| `int` | alias for `int64` | |
| `uint8` | 8-bit unsigned | 0 to 255 |
| `uint16` | 16-bit unsigned | 0 to 65,535 |
| `uint32` | 32-bit unsigned | 0 to 2^32-1 |
| `uint64` | 64-bit unsigned | 0 to 2^64-1 |
| `uint` | alias for `uint64` | |

### Other Primitives

| Type | Description |
|------|-------------|
| `bool` | Boolean, `true` or `false` |
| `string` | C string pointer (`const char*`) |
| `void` | No value (for return types) |

## Type References

A type reference specifies a type with optional modifiers:

```
TypeRef = ValueType [ 'volatile' ] [ '*' ]
ValueType = Type [ '<' TypeArgs '>' ] [ '?' ]
```

### Nullability

Values are non-null by default.

```gecko
let x: int32      // non-null int32
let y: int32?     // nullable int32
```

For pointers:

- `Type*` means a nullable pointer to a non-null `Type` value.
- `Type?*` means a nullable pointer to a nullable `Type` value.
- `Type*?` is invalid syntax (pointer nullability is implicit and cannot be annotated).

```gecko
let p: int32*      // pointer may be null, pointee is non-null int32
let q: int32?*     // pointer may be null, pointee is nullable int32
let r: int32*?     // invalid
```

### Pointers

```gecko
let p: int32*           // pointer to int32
let q: int32 volatile*  // pointer to volatile int32
let r: int32?*          // pointer to nullable int32
```

### Mutability

Mutability is controlled by declaration keywords, not type suffixes:

```gecko
let x: int32 = 42
const y: int32 = 42
```

`const` makes the binding immutable.

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
let x = 42              // infers int (int64)
let y = true            // infers bool
let s = "hello"         // infers string
let r = Rect::new(1,2)  // infers Rect from static method return
let a = obj.area()      // infers from method return type
let p = &variable       // infers pointer type
let x: int32 = make_zero() // expected type context participates in inference
```

Inference is bidirectional for generics:
- from call arguments
- from expected type context (assignment targets and return context)

If inference is ambiguous, compilation fails and requires explicit `<...>` type arguments.

## Scope Narrowing

Gecko performs path-sensitive narrowing for core nullability flows.

```gecko
func use(ptr: int32*): int32 {
    if (ptr == nil) {
        return 0
    }
    // ptr is narrowed to non-null on this path
    return require_nonnull(ptr)
}
```

Narrowing rules cover:
- `if/else if/else` branches
- short-circuit boolean guards (`&&` and `||`) conservatively
- early exits (`return`, `break`, `continue`) at control-flow join points

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
