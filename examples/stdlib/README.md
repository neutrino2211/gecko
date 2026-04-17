# Standard Library Example

Demonstrates using Gecko's stdlib modules via the import system.

## Running

```bash
gecko run demo.gecko
echo $?  # Expected: 80
```

## What it does

- `math.abs(-15)` = 15
- `math.min(5, 10)` = 5
- `math.max(5, 10)` = 10
- `math.clamp(100, 0, 50)` = 50
- **Total: 80**

## Available stdlib modules

- `std/types.gecko` - Core traits (Sized, Default, Clone, Eq, Ord, Add, etc.)
- `std/mem.gecko` - Memory operations (malloc, free, memcpy, etc.)
- `std/io.gecko` - Basic I/O (putchar, puts, etc.)
- `std/str.gecko` - String utilities (strlen, strcmp, etc.)
- `std/math.gecko` - Math functions (abs, min, max, clamp)
- `std/sys.gecko` - System operations (exit, abort, sleep)

## Note on imports

Imports are resolved relative to the source file. Copy needed stdlib modules to your project directory or place them adjacent to your source files.
