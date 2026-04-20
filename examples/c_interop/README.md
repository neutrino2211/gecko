# C Interop Example

This example demonstrates Gecko's C interoperability features using the `cimport` directive.

## Features Demonstrated

1. **cimport** - Import C header files directly
2. **declare external** - Declare C functions for use in Gecko
3. **variardic functions** - Support for C variadic functions like printf
4. **Pointer operations** - Passing pointers to C functions
5. **Struct interop** - Gecko structs are compatible with C

## Running

```bash
gecko run examples/c_interop/main.gecko
```

## How cimport Works

The `cimport` directive generates `#include` statements in the output C code:

```gecko
cimport "<stdio.h>"     // System header: #include <stdio.h>
cimport "myheader.h"    // Local header:  #include "myheader.h"
```

## Notes

- When using `cimport`, avoid re-declaring functions that are already in the header (e.g., malloc, strlen) to prevent conflicting type errors
- The `variardic` keyword enables C variadic function calls (like printf with format strings)
- Gecko structs compile directly to C structs with compatible memory layout
