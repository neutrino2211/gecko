---
title: C Interoperability
description: Calling C functions and using C libraries
sidebar:
  order: 6
---

## Declaring External Functions

Use `declare external` to declare C functions:

```gecko
declare external func malloc(size: uint64): void*
declare external func free(ptr: void*): void
declare external func strlen(s: string): uint64
```

### Variadic Functions

```gecko
declare external variardic func printf(format: string): int
```

## External Variables

```gecko
declare external let errno: int
```

## Type Mapping

| Gecko Type | C Type |
|------------|--------|
| `int8` | `int8_t` |
| `int16` | `int16_t` |
| `int32` | `int32_t` |
| `int64` | `int64_t` |
| `uint8` | `uint8_t` |
| `uint16` | `uint16_t` |
| `uint32` | `uint32_t` |
| `uint64` | `uint64_t` |
| `float32` | `float` |
| `float64` | `double` |
| `bool` | `int` (0 or 1) |
| `string` | `const char*` |
| `void*` | `void*` |
| `T*` | `T*` |

## Opaque Types

Declare types that exist in C but whose internals are hidden:

```gecko
declare external type FILE

declare external func fopen(path: string, mode: string): FILE*
declare external func fclose(file: FILE*): int
declare external func fread(buf: void*, size: uint64, count: uint64, file: FILE*): uint64
```

## Struct Compatibility

Gecko classes map directly to C structs:

```gecko
// Gecko
class Point {
    let x: int32
    let y: int32
}

// Equivalent C
// typedef struct { int32_t x; int32_t y; } Point;
```

### Packed Structs

For exact memory layout control:

```gecko
@packed
class NetworkHeader {
    let version: uint8
    let flags: uint8
    let length: uint16
}
```

## Inline Assembly

For direct hardware access:

```gecko
@naked
@noreturn
func halt() {
    asm { "cli; hlt" }
}
```

## C Import (Planned)

```gecko
cimport "stdio.h"
cimport "mylib.h" withlibrary "mylib"
```

## Example: File I/O

```gecko
package main

declare external type FILE
declare external func fopen(path: string, mode: string): FILE*
declare external func fclose(file: FILE*): int
declare external func fgets(buf: string, size: int, file: FILE*): string
declare external variardic func printf(format: string): int

func main(): int {
    let f: FILE* = fopen("test.txt", "r")
    if @is_null(f as void*) {
        printf("Failed to open file\n")
        return 1
    }

    let buf: [256]uint8
    fgets(buf as string, 256, f)
    printf("Read: %s", buf as string)

    fclose(f)
    return 0
}
```

## Function Attributes

| Attribute | Description |
|-----------|-------------|
| `@naked` | No prologue/epilogue |
| `@noreturn` | Function never returns |
| `@section(".text")` | Place in specific section |
