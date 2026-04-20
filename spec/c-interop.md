# C Interoperability

Gecko is designed for seamless C ABI interop.

## External Functions

Declare C functions for use in Gecko:

```gecko
declare external func malloc(size: uint64): void*
declare external func free(ptr: void*): void
declare external func puts(s: string): int32
declare external variardic func printf(fmt: string): int32
```

### Syntax

```gecko
declare external [variardic] func name(params): ReturnType
```

- `external` - Uses C calling convention, no name mangling
- `variardic` - C-style variadic function (note the typo is intentional)

## Exporting Functions

Make Gecko functions callable from C:

```gecko
external func main(): int32 {
    return 0
}

external func gecko_init(config: Config*): bool {
    // ...
}
```

The `external` visibility modifier:
- Prevents name mangling
- Uses C calling convention
- Makes symbol visible to linker

## External Types

Declare opaque C types:

```gecko
declare external type FILE
declare external type pthread_t
```

Use only through pointers:

```gecko
declare external func fopen(path: string, mode: string): FILE*
declare external func fclose(f: FILE*): int32

let f: FILE* = fopen("test.txt", "r")
fclose(f)
```

## External Classes

Map Gecko classes to C structs:

```gecko
external "struct stat" class Stat {
    let st_mode: uint32
    let st_size: int64
    // ... other fields
}

declare external func stat(path: string, buf: Stat*): int32
```

The `external "name"` provides the C type name.

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
| `bool` | `_Bool` |
| `string` | `const char*` |
| `void` | `void` |
| `T*` | `T*` |
| `void*` | `void*` |

## Calling Conventions

All external functions use the C calling convention (cdecl on x86, standard ABI on ARM64).

## Name Mangling

- `external` functions: No mangling, use exact name
- Regular functions: Mangled as `package__functionname`
- Methods: Mangled as `package__Class__methodname`
- Generic instantiations: Include type parameters in name

Example:
```gecko
package mylib

func helper(): void { }           // mylib__helper
external func exported(): void { } // exported

class Point {
    func new(): Point { }          // mylib__Point__new
}
```

## Inline Assembly

For low-level control:

```gecko
func halt(): void {
    asm { "hlt" }
}

func outb(port: uint16, value: uint8): void {
    asm { "outb %1, %0" : : "dN"(port), "a"(value) }
}
```

**Note**: Assembly syntax follows GCC inline assembly format.

## C Header Import

```gecko
cimport "stdio.h"
cimport "mylib.h" withobject "mylib.o"
cimport "mylib.h" withlibrary "mylib"
```

**Gap**: `cimport` is parsed but not functional. C header parsing not implemented. Use manual `declare external` instead.

## Linking

The compiler produces object files. Link with gcc/clang:

```bash
gecko build main.gecko -o main.o
gcc main.o -lc -o main
```

For freestanding:

```bash
gecko build kernel.gecko -o kernel.o
ld -T linker.ld kernel.o -o kernel.elf
```

## Gaps and Limitations

- No C header parsing (`cimport` not functional)
- No automatic binding generation
- No `#define` constant import
- No C macro support
- No union types
- No bitfield support
- No anonymous structs/unions
- No flexible array members
- Limited enum interop (basic only)
- No function pointer typedefs from C
