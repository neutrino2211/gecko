# Functions

## Declaration Syntax

```gecko
[visibility] [variardic] func name[<TypeParams>](params): ReturnType {
    body
}
```

## Basic Functions

```gecko
func add(a: int32, b: int32): int32 {
    return a + b
}

func greet(): void {
    puts("Hello")
}
```

## Visibility

| Modifier | Meaning |
|----------|---------|
| (none) | Package-private |
| `public` | Exported from module |
| `private` | Only within current scope |
| `external` | Exported with C linkage (no name mangling) |

```gecko
external func main(): int32 {
    return 0
}

public func helper(): void {
    // accessible from other modules
}
```

## Parameters

### Basic Parameters

```gecko
func process(data: uint8*, len: uint64): bool {
    // ...
}
```

### Self Parameter

Methods use `self` as the receiver:

```gecko
func area(self): int32 {
    return self.width * self.height
}
```

**Note**: `self` is always a pointer to the class instance.

### Variadic Functions

```gecko
variardic func printf(fmt: string): int32 {
    // C-style varargs
}
```

**Note**: The keyword is `variardic` (with typo preserved for compatibility).

## Return Types

Explicit return type after colon:

```gecko
func compute(): int64 {
    return 42
}
```

Void functions:

```gecko
func log(msg: string): void {
    puts(msg)
}

// or omit return type (defaults to void)
func log2(msg: string) {
    puts(msg)
}
```

## Early Return

```gecko
func find(arr: int32*, len: int32, target: int32): int32 {
    let i: int32 = 0
    while i < len {
        if arr[i] == target {
            return i
        }
        i = i + 1
    }
    return -1
}
```

## Generic Functions

See [generics.md](generics.md).

```gecko
func identity<T>(x: T): T {
    return x
}
```

## Forward Declarations

```gecko
declare func process(data: uint8*): int32
```

## External Declarations

See [c-interop.md](c-interop.md).

```gecko
declare external func malloc(size: uint64): void*
declare external variardic func printf(fmt: string): int32
```

## Gaps and Limitations

- No default parameter values
- No named arguments at call site
- No function overloading
- No closures or lambdas
- Variadic functions only for C interop (no type-safe variadics)
