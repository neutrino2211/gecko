---
title: FFI & C Interop
description: Calling C code from Gecko and vice versa
sidebar:
  order: 11
---

Gecko provides seamless interoperability with C code, allowing you to use existing C libraries and integrate with system APIs.

## Importing C Headers

### Basic Import

```gecko
cimport "stdio.h"

func main(): void {
    printf("Hello from Gecko!\n")
}
```

### With Object Files

Link with compiled C code:

```gecko
cimport "mylib.h" withobject "mylib.o"
```

### With Libraries

Link with system or external libraries:

```gecko
cimport "math.h" withlibrary "m"

func main(): void {
    let x = sin(3.14159)
    let y = sqrt(2.0)
}
```

## External Declarations

Explicitly declare C functions without importing a header:

### Functions

```gecko
// Simple function
declare external func puts(s: string): int

// Function returning void
declare external func exit(code: int): void

// Function with pointer parameter
declare external func free(ptr: void*): void
```

### Variadic Functions

```gecko
declare external variardic func printf(format: string): int
declare external variardic func sprintf(buf: string, format: string): int

func main(): void {
    printf("Value: %d\n", 42)
    printf("Name: %s, Age: %d\n", "Alice", 30)
}
```

### Opaque Types

For types whose internals you don't need:

```gecko
declare external type FILE

declare external func fopen(path: string, mode: string): FILE*
declare external func fclose(file: FILE*): int
declare external func fread(buf: void*, size: uint64, count: uint64, file: FILE*): uint64
```

## C Struct Mapping

Map Gecko classes to C structs:

```gecko
// C struct:
// struct Point {
//     int x;
//     int y;
// };

external "Point" class Point {
    let x: int
    let y: int
}

declare external func draw_point(p: Point*): void
```

### Packed Structs

For structs that must match exact C layout:

```gecko
// C struct with no padding
@packed
external "PackedData" class PackedData {
    let flag: uint8
    let value: uint32
    let tag: uint8
}
```

## Type Mapping

| Gecko Type | C Type |
|------------|--------|
| `int8` | `int8_t` / `char` |
| `int16` | `int16_t` / `short` |
| `int32` | `int32_t` / `int` |
| `int64` | `int64_t` / `long long` |
| `int` | `int` (platform-sized) |
| `uint8` | `uint8_t` / `unsigned char` |
| `uint16` | `uint16_t` / `unsigned short` |
| `uint32` | `uint32_t` / `unsigned int` |
| `uint64` | `uint64_t` / `unsigned long long` |
| `uint` | `unsigned int` |
| `float32` | `float` |
| `float64` | `double` |
| `bool` | `bool` / `_Bool` |
| `string` | `const char*` |
| `void*` | `void*` |
| `T*` | `T*` |

## Common Patterns

### Memory Allocation

```gecko
declare external func malloc(size: uint64): void*
declare external func free(ptr: void*): void
declare external func realloc(ptr: void*, size: uint64): void*

func allocate_buffer(size: uint64): uint8* {
    return malloc(size) as uint8*
}

func free_buffer(buf: uint8*): void {
    free(buf as void*)
}
```

### String Handling

```gecko
cimport "string.h"

func copy_string(dest: string, src: string): void {
    strcpy(dest, src)
}

func string_length(s: string): uint64 {
    return strlen(s)
}
```

### Error Handling with errno

```gecko
cimport "errno.h"

declare external func strerror(errnum: int): string

func check_error(): void {
    if errno != 0 {
        let msg = strerror(errno)
        printf("Error: %s\n", msg)
    }
}
```

### File I/O

```gecko
cimport "stdio.h"

func read_file(path: string): string {
    let file = fopen(path, "r")
    if @is_null(file) {
        return ""
    }
    
    // Get file size
    fseek(file, 0, 2)  // SEEK_END
    let size = ftell(file)
    fseek(file, 0, 0)  // SEEK_SET
    
    // Allocate and read
    let buf = malloc(size + 1) as string
    fread(buf as void*, 1, size as uint64, file)
    
    fclose(file)
    return buf
}
```

### Callbacks

Pass Gecko functions as C callbacks:

```gecko
// C: typedef int (*compare_fn)(const void*, const void*);
// C: void qsort(void* base, size_t n, size_t size, compare_fn cmp);

declare external func qsort(
    base: void*,
    n: uint64,
    size: uint64,
    cmp: func(void*, void*): int
): void

func compare_ints(a: void*, b: void*): int {
    let ia = @deref(a as int*)
    let ib = @deref(b as int*)
    return ia - ib
}

func sort_array(arr: int*, len: uint64): void {
    qsort(arr as void*, len, @size_of<int>(), compare_ints)
}
```

## Exposing Gecko to C

Gecko functions can be called from C using the C backend:

```gecko
// gecko_lib.gecko
package geckolib

public func add_numbers(a: int, b: int): int {
    return a + b
}
```

Compile to C:
```bash
gecko compile --type library gecko_lib.gecko
```

Use from C:
```c
// main.c
extern int geckolib_add_numbers(int a, int b);

int main() {
    int result = geckolib_add_numbers(10, 20);
    printf("Result: %d\n", result);
    return 0;
}
```

## Safety Considerations

1. **Null pointers** - C functions may return NULL; always check
2. **Buffer overflows** - C doesn't check bounds; validate sizes
3. **Memory leaks** - Match every malloc with free
4. **Type mismatches** - Ensure Gecko types match C types exactly
5. **Calling conventions** - Default is cdecl; adjust for other ABIs

```gecko
// Safe wrapper pattern
func safe_read_file(path: string): Option<string> {
    let file = fopen(path, "r")
    if @is_null(file) {
        return Option::none()
    }
    
    // ... read file ...
    
    fclose(file)
    return Option::some(content)
}
```

## Tips

1. **Use cimport for system headers** - Handles include paths automatically
2. **Prefer explicit declarations** for better documentation
3. **Wrap unsafe C APIs** in safe Gecko interfaces
4. **Test thoroughly** - FFI bugs can be subtle and dangerous
5. **Match struct layouts exactly** - Use @packed when needed
