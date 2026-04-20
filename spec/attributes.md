# Attributes

Attributes provide compile-time metadata for declarations.

## Syntax

```gecko
@name
@name("value")
@name(.method)
@name(.method1, .method2)
```

Attributes precede the declaration they modify.

## Function Attributes

### @naked

Omit function prologue/epilogue:

```gecko
@naked
func isr_handler(): void {
    asm { "pusha" }
    // handle interrupt
    asm { "popa" }
    asm { "iret" }
}
```

Use for: interrupt handlers, assembly trampolines.

### @noreturn

Mark function as never returning:

```gecko
@noreturn
func panic(msg: string): void {
    puts(msg)
    while true {
        asm { "hlt" }
    }
}
```

Allows compiler to optimize callers.

### @section

Place function in specific ELF section:

```gecko
@section(".text.boot")
func _start(): void {
    // bootloader entry point
}
```

## Class Attributes

### @packed

Remove struct padding:

```gecko
@packed
class GDTEntry {
    let limit_low: uint16
    let base_low: uint16
    let base_middle: uint8
    let access: uint8
    let granularity: uint8
    let base_high: uint8
}
```

Generated C:
```c
struct __attribute__((packed)) GDTEntry { ... };
```

## Field Attributes

### @section

Place global variable in specific section:

```gecko
@section(".data.boot")
let boot_stack: [4096]uint8
```

## File Attributes

### @backend

Specify compilation backend:

```gecko
@backend("llvm")
package mymodule
```

Values: `"c"` (default), `"llvm"`

## Trait Hook Attributes

Hook attributes connect compiler features to user-defined traits. See [Traits](traits.md) for full documentation.

### Lifecycle Hooks

| Attribute | Purpose | Expected Signature |
|-----------|---------|-------------------|
| `@drop_hook(.method)` | Cleanup on scope exit | `func method(self): void` |
| `@copy_hook(.method)` | Implicit bitwise copy | `func method(self): Self` |
| `@clone_hook(.method)` | Explicit deep clone | `func method(self): Self` |

```gecko
@drop_hook(.drop)
trait Drop {
    func drop(self): void
}
```

### Operator Hooks

| Attribute | Operator | Expected Signature |
|-----------|----------|-------------------|
| `@add_hook(.method)` | `+` | `func method(self, other: T): T` |
| `@sub_hook(.method)` | `-` | `func method(self, other: T): T` |
| `@mul_hook(.method)` | `*` | `func method(self, other: T): T` |
| `@div_hook(.method)` | `/` | `func method(self, other: T): T` |
| `@neg_hook(.method)` | unary `-` | `func method(self): Self` |
| `@eq_hook(.method)` | `==` | `func method(self, other: T): bool` |
| `@ne_hook(.method)` | `!=` | `func method(self, other: T): bool` |
| `@lt_hook(.method)` | `<` | `func method(self, other: T): bool` |
| `@gt_hook(.method)` | `>` | `func method(self, other: T): bool` |
| `@le_hook(.method)` | `<=` | `func method(self, other: T): bool` |
| `@ge_hook(.method)` | `>=` | `func method(self, other: T): bool` |
| `@bitand_hook(.method)` | `&` | `func method(self, other: T): T` |
| `@bitor_hook(.method)` | `\|` | `func method(self, other: T): T` |
| `@bitxor_hook(.method)` | `^` | `func method(self, other: T): T` |
| `@shl_hook(.method)` | `<<` | `func method(self, other: T): T` |
| `@shr_hook(.method)` | `>>` | `func method(self, other: T): T` |

### Indexing Hooks

| Attribute | Sugar | Expected Signature |
|-----------|-------|-------------------|
| `@index_hook(.method)` | `a[i]` read | `func method(self, index: I): T` |
| `@index_mut_hook(.method)` | `a[i] = v` write | `func method(self, index: I, value: T): void` |

### Iterator Hooks

| Attribute | Sugar | Expected Signature |
|-----------|-------|-------------------|
| `@iterator_hook(.next, .has_next)` | iteration | `func next(self): T`, `func has_next(self): bool` |
| `@into_iterator_hook(.method)` | `for x in y` | `func method(self): Iterator<T>` |

### Hook Rules

1. **One per capability** - Defining a second `@drop_hook` in the same scope is a compile error
2. **Signature verification** - Compiler checks the trait signature matches expectations
3. **Module-scoped** - Hooks apply to the defining module and its importers

### Example

```gecko
@add_hook(.add)
public trait Add<T> {
    func add(self, other: T): T
}

class Vector2 {
    let x: int32
    let y: int32
}

impl Add<Vector2> for Vector2 {
    func add(self, other: Vector2): Vector2 {
        return Vector2 { 
            x: self.x + other.x, 
            y: self.y + other.y 
        }
    }
}

// Now + works for Vector2:
let a: Vector2 = Vector2 { x: 1, y: 2 }
let b: Vector2 = Vector2 { x: 3, y: 4 }
let c: Vector2 = a + b  // calls a.add(b)
```

## Multiple Attributes

Stack multiple attributes:

```gecko
@naked
@noreturn
@section(".text.init")
func kernel_entry(): void {
    // ...
}
```

## Attribute Processing

Attributes map to:
- GCC/Clang `__attribute__` extensions
- LLVM IR attributes
- Linker section directives
- Compiler code generation hooks
