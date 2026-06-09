# Operators

## Arithmetic Operators

| Operator | Description | Example |
|----------|-------------|---------|
| `+` | Addition | `a + b` |
| `-` | Subtraction | `a - b` |
| `*` | Multiplication | `a * b` |
| `/` | Division | `a / b` |
| `%` | Modulo | `a % b` |
| `-` (unary) | Negation | `-x` |

## Comparison Operators

| Operator | Description | Example |
|----------|-------------|---------|
| `==` | Equal | `a == b` |
| `!=` | Not equal | `a != b` |
| `<` | Less than | `a < b` |
| `>` | Greater than | `a > b` |
| `<=` | Less or equal | `a <= b` |
| `>=` | Greater or equal | `a >= b` |

All comparisons return `bool`.

## Logical Operators

| Operator | Description | Example |
|----------|-------------|---------|
| `&&` | Logical AND | `a && b` |
| `\|\|` | Logical OR | `a \|\| b` |
| `!` | Logical NOT | `!a` |

Short-circuit evaluation: `&&` and `||` don't evaluate the right operand if the result is determined by the left.

## Bitwise Operators

| Operator | Description | Example |
|----------|-------------|---------|
| `&` | Bitwise AND | `a & b` |
| `\|` | Bitwise OR | `a \| b` |
| `^` | Bitwise XOR | `a ^ b` |
| `<<` | Left shift | `a << 2` |
| `>>` | Right shift (arithmetic) | `a >> 2` |
| `<<<` | Left rotate | `a <<< 4` |
| `>>>` | Right rotate | `a >>> 4` |

## Assignment

Simple assignment:

```gecko
x = 42
obj.field = value
arr[i] = value
```

Explicit global assignment (bypasses local shadowing):

```gecko
global counter = 42
global state.buffer[i] = byte
```

**Gap**: No compound assignment (`+=`, `-=`, etc.).

## Address-Of

Get pointer to variable:

```gecko
let x: int32 = 42
let p: int32* = &x
```

## Dereference

Access value through pointer:

```gecko
let value: int32 = @deref(p)
```

`@deref(...)` is the canonical dereference form.

Field access on pointers uses `.` (no `->` operator).

## Type Cast

```gecko
let addr: uint64 = ptr as uint64
let ptr: uint8* = 0x1000 as uint8*
```

## Index

Array and pointer indexing:

```gecko
let value: int32 = arr[i]
arr[i] = 42
```

## Member Access

```gecko
let w: int32 = rect.width      // field access
let a: int32 = shape.area()    // method call
```

Same syntax for values and pointers (no `->` operator).

## Static Method Call

```gecko
let opt: Option<int32> = Option<int32>::some(42)
let rect: Rect = Rect::new(10, 20)
```

## Operator Precedence

From highest to lowest:

1. Unary: `!`, `-`, `+`, `&` (address-of)
2. Cast: `as`
3. Multiplicative: `*`, `/`, `%`
4. Additive: `+`, `-`
5. Shift/Rotate: `<<`, `>>`, `<<<`, `>>>`
6. Bitwise AND: `&`
7. Bitwise XOR: `^`
8. Bitwise OR: `|`
9. Comparison: `<`, `>`, `<=`, `>=`
10. Equality: `==`, `!=`
11. Logical AND: `&&`
12. Logical OR: `||`

Use parentheses for clarity:

```gecko
let result: int32 = (a + b) * c
let check: bool = (x > 0) && (y < 10)
```

## Gaps and Limitations

- No compound assignment (`+=`, `-=`, `*=`, `/=`)
- No increment/decrement (`++`, `--`)
- No ternary operator (`? :`)
- No user-defined operators beyond hook-mapped built-ins
- No null coalescing (`??`)
- No range operator (`..`)
