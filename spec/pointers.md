# Pointer Safety

Pointers in Gecko are explicitly managed through intrinsics. No implicit operations are allowed on pointer types.

## Design Principles

1. **Explicit over implicit** - All pointer operations require intrinsics
2. **Auditable** - Pointer operations are grep-able (`@ptr_`)
3. **C interop unchanged** - Pointers pass to/from C naturally
4. **Safe by default** - Accidental misuse is impossible

## Pointer Types

```gecko
T*      // Pointer to T (nullable)
T*!     // Non-null pointer to T
void*   // Opaque pointer (from C)
```

## Restricted Operations

The following operations are **compile errors** on pointer types:

```gecko
let p: int32* = get_pointer()

// ALL ERRORS:
let q = p           // Assignment
let q: int32* = p   // Even with type annotation
let v = p[0]        // Indexing/dereference
let v = *p          // Dereference operator
p = p + 1           // Arithmetic
p += 1              // Compound assignment
if p { }            // Implicit bool conversion (use @ptr_is_null)
```

## Pointer Intrinsics

### Movement and Copying

```gecko
// Move pointer ownership (source becomes invalid)
@ptr_move(source, dest)

// Copy pointer value (both remain valid)
@ptr_copy(source, dest)
```

**Example:**
```gecko
let src: int32* = malloc(4) as int32*
let dest: int32*

@ptr_move(src, dest)    // src is now invalid
// src cannot be used after this point

let other: int32*
@ptr_copy(dest, other)  // Both dest and other are valid
```

### Reading and Writing

```gecko
// Read value at pointer + offset
@ptr_read(ptr, offset) -> T

// Write value at pointer + offset
@ptr_write(ptr, offset, value)
```

**Example:**
```gecko
let data: int32* = malloc(12) as int32*

@ptr_write(data, 0, 10)     // data[0] = 10
@ptr_write(data, 1, 20)     // data[1] = 20
@ptr_write(data, 2, 30)     // data[2] = 30

let first = @ptr_read(data, 0)   // first = 10
let second = @ptr_read(data, 1)  // second = 20
```

### Pointer Arithmetic

```gecko
// Get pointer offset by N elements
@ptr_offset(ptr, n) -> T*

// Get distance between two pointers (in elements)
@ptr_diff(p1, p2) -> int64
```

**Example:**
```gecko
let arr: int32* = malloc(40) as int32*
let third = @ptr_offset(arr, 2)      // Points to arr[2]
let tenth = @ptr_offset(arr, 9)      // Points to arr[9]

let distance = @ptr_diff(tenth, third)  // distance = 7
```

### Null Checking

```gecko
// Check if pointer is null
@ptr_is_null(ptr) -> bool

// Assert pointer is non-null (returns T*!)
@ptr_unwrap(ptr) -> T*!
```

**Example:**
```gecko
let p: int32* = maybe_null_pointer()

if @ptr_is_null(p) {
    return -1
}

let valid: int32*! = @ptr_unwrap(p)  // Safe after null check
```

### Casting

```gecko
// Cast pointer to different type
@ptr_cast<T>(ptr) -> T*

// Cast pointer to integer
@ptr_to_int(ptr) -> uint64

// Cast integer to pointer
@int_to_ptr<T>(addr) -> T*
```

**Example:**
```gecko
let void_ptr: void* = malloc(100)
let byte_ptr: uint8* = @ptr_cast<uint8>(void_ptr)

let addr: uint64 = @ptr_to_int(byte_ptr)
let restored: uint8* = @int_to_ptr<uint8>(addr)
```

## Allowed Operations

The following operations **are allowed** without intrinsics:

```gecko
// Passing to functions (including C)
free(ptr)
memcpy(dest, src, len)

// Returning from functions
func get_buffer(): uint8* { ... }

// Comparison (equality only)
if ptr == other_ptr { }
if ptr != other_ptr { }

// Null literal assignment (initialization only)
let p: int32* = null
```

## Smart Pointer Types

The stdlib provides safe wrappers that use intrinsics internally:

```gecko
Box<T>      // Owned heap allocation, auto-drop
Rc<T>       // Reference counted
Weak<T>     // Weak reference
Buffer<T>   // Sized buffer with bounds checking
Slice<T>    // Unsized view with length
```

**Example:**
```gecko
import std.box use { Box }

// Safe - Box handles the pointer internally
let data: Box<int32> = Box::new(42)
let value = data.get()    // No intrinsics needed
// Automatically freed when data goes out of scope
```

## C Interop

External declarations work unchanged - the restriction is on Gecko code:

```gecko
declare external func malloc(size: uint64): void*
declare external func free(ptr: void*): void
declare external func memcpy(dest: void*, src: void*, n: uint64): void*

func example(): void {
    let raw: void* = malloc(100)      // OK: return from C
    
    let data: uint8*
    @ptr_cast<uint8>(raw)             // Cast void* to uint8*
    @ptr_move(raw, data)              // Move to typed pointer (raw invalid after)
    
    @ptr_write(data, 0, 65)           // Write 'A'
    @ptr_write(data, 1, 0)            // Null terminator
    
    free(data as void*)               // OK: pass to C
}
```

## Error Handler

For runtime pointer errors (null dereference via `@ptr_unwrap`), provide a handler:

```gecko
@error_handler
func __gecko_error(code: int32, msg: string): void {
    // Handle error - log, panic, halt, etc.
}
```

Error codes:
- `1` - Null pointer unwrap
- `2` - Use after move (if runtime checking enabled)

## Migration Guide

**Before (unsafe implicit):**
```gecko
let p: int32* = malloc(4) as int32*
*p = 42
let v = *p
let q = p
free(p)
```

**After (explicit intrinsics):**
```gecko
let p: int32* = malloc(4) as int32*
@ptr_write(p, 0, 42)
let v = @ptr_read(p, 0)
let q: int32*
@ptr_copy(p, q)
free(p)
```

## Compiler Implementation

1. **Type checker**: Reject implicit pointer operations
2. **Intrinsic codegen**: Generate appropriate C/LLVM for each intrinsic
3. **Move tracking**: Optional runtime checks for use-after-move
4. **Error handler**: Link to user-provided handler or default panic
