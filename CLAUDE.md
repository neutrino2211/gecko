# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is Gecko

Gecko is a compiled general-purpose systems programming language written in Go. It has both LLVM IR and C backends, providing TypeScript-like ergonomics for low-level programming with C ABI interoperability.

## Build and Run Commands

```bash
# Install dependencies
go get

# Run the compiler (shows help)
go run .

# Compile to C (default backend)
gecko compile examples/traits/shapes.gecko

# Build executable
gecko build examples/traits/shapes.gecko -o shapes

# Compile and run
gecko run examples/traits/shapes.gecko

# View generated C code
gecko compile --print-ir --ir-only examples/traits/shapes.gecko

# Cross-compile for specific target
gecko compile --target-arch=amd64 --target-platform=linux <file>

# Enable debug logging
gecko compile --log-level debug <file>
# Or via environment variable
GECKO_DEBUG=1 gecko compile <file>
```

## Examples

The `examples/` directory contains complete, buildable examples:

- **`examples/hello_kernel/`** - A minimal x86 kernel (requires QEMU)
- **`examples/traits/`** - Demonstrates traits, trait constraints, and generics
- **`examples/string_builder/`** - StringBuilder from stdlib with project-based build

## Testing

```bash
# Run all tests
go test ./tests/... -v

# Run specific test
go test ./tests/... -v -run TestCompileAndRun/traits_basic
```

Test files:
- `test_sources/compile_tests/` - Integration tests that should compile and run
- `test_sources/kitchen_sink/` - Conceptual syntax examples (may not compile)
- `tests/` - Go test files for the compiler

## Architecture

### Compilation Pipeline

```
Source (.gecko) -> Lexer/Parser (Participle) -> tokens.File -> Backend -> C/LLVM IR -> gcc/llc -> .o
```

### Key Packages

- **`tokens/`** - Grammar definitions using Participle parser combinator. `tokens.go` defines all AST node types.
- **`parser/`** - EBNF lexer definition and Participle parser configuration.
- **`ast/`** - Semantic AST with scope hierarchy for symbol resolution.
- **`backends/`** - Backend abstraction. `BackendProcessEntries()` dispatches token types to implementations.
- **`backends/c_backend/`** - C code generation (recommended for kernel development).
- **`backends/llvm_backend/`** - LLVM IR generation using `github.com/llir/llvm`.
- **`compiler/`** - Orchestrates parsing and backend invocation.
- **`interfaces/`** - `BackendInterface` and `BackendCodegenImplementations` define the backend contract.

### Adding New Syntax

1. Define the token struct in `tokens/tokens.go` with Participle struct tags
2. Add it to the `Entry` struct (top-level dispatch point)
3. Handle it in `backends/backends.go` `BackendProcessEntries()`
4. Implement codegen in both `c_backend/` and `llvm_backend/`

## Language Features

### Working Features

- Functions with parameters and return types
- Variables (`let`/`const`) with optional type annotations
- Type inference for literals, expressions, and variable references: `let x = 42`
- Primitive types: `int`, `int8-64`, `uint`, `uint8-64`, `bool`, `string`, `void`
- Control flow: `if`/`else if`/`else`, `while`, `for`
- All operators: arithmetic, comparison, bitwise (`&`, `|`, `^`, `<<`, `>>`)
- C interop via `declare external`
- Classes/structs with `@packed` attribute
- Volatile pointers: `uint16 volatile*`
- Pointer/integer casts: `0xB8000 as uint16*`
- Array indexing: `arr[i]` for read and write
- Fixed-size arrays: `[4096]uint8`
- Inline assembly: `asm { "hlt" }`
- Function attributes: `@naked`, `@noreturn`, `@section(".text")`
- Global variables with `@section` placement
- Struct literals: `Type { field: value }`
- Module imports with dot notation: `import std.collections.string`
- Selective imports: `import std.collections.vec use { Vec }`
- Directory imports with qualified types: `import ./shapes` then `shapes.Circle`
- Generics with monomorphization: `func identity<T>(x: T): T`
- Traits: `trait Name { func method(self): Type }`
- Trait implementations: `impl Trait for Class { ... }`
- Trait constraints: `func process<T is Trait>(x: T)`
- Struct field access and assignment: `obj.field`, `obj.field = value`
- Address-of operator: `&variable`
- Function pointers: `func(T, T): T`
- Break/continue in loops
- For-in loops with `@iterator_hook` trait: `for let x in collection { }`

### Standard Library

The stdlib lives in `$GECKO_HOME/stdlib/` and uses dot notation imports:

```gecko
// Import specific types
import std.collections.string use { StringBuilder, String }
import std.collections.vec use { Vec }
import std.collections.slice use { Slice }
import std.memory.box use { Box }

// Use qualified access
import std.collections.string
let sb: string.StringBuilder
```

**Stdlib structure:**
```
stdlib/
├── core/
│   ├── traits.gecko    # Drop, Clone, Copy, Iterator, Index
│   └── ops.gecko       # Add, Sub, Mul, Div, Eq, Lt, Gt, etc.
├── collections/
│   ├── vec.gecko       # Vec<T> dynamic array
│   ├── slice.gecko     # Slice<T> borrowed view
│   └── string.gecko    # String, StringBuilder
├── memory/
│   ├── box.gecko       # Box<T> heap allocation
│   ├── rc.gecko        # Rc<T> reference counting
│   └── weak.gecko      # Weak<T> weak references
└── option.gecko        # Option<T>
```

### Hook Attributes

Traits can define hooks that enable syntactic sugar:

```gecko
@drop_hook(.drop)           // Called on scope exit
@clone_hook(.clone)         // Explicit cloning
@iterator_hook(.next, .has_next)  // for-in loops
@index_hook(.index)         // arr[i] read access
@index_mut_hook(.index_mut) // arr[i] = v write access
@add_hook(.add)             // a + b operator
@eq_hook(.eq)               // a == b operator
```

### Visibility

Symbols are private by default. Use `public` for exports:

```gecko
public class MyClass { ... }
public func helper(): void { ... }
public trait MyTrait { ... }

impl MyClass {
    public func visible(self): void { }  // Accessible from other modules
    func internal(self): void { }        // Private to this module
}
```

### Type Checking

The compiler performs type checking at compile time:

- **Variable assignments**: `let x: int = "hello"` errors with type mismatch
- **Field assignments**: `circle.radius = "hello"` errors when assigning wrong type to field
- **Function arguments**: Type mismatches in function calls are caught
- **Return types**: Return statements are validated against function return type

Type inference works for:
- Literals: `let x = 42` infers `int32`
- Static method calls: `let r = Rectangle::new(10, 5)` infers `Rectangle`
- Method calls: `let a = shape.area()` infers return type from trait/class method
- Address-of: `let p = &variable` infers pointer type

### LSP Support

The `lsp/` package provides Language Server Protocol support:

- **Hover**: Shows type information for variables, expressions, and function calls
- **Completions**: 
  - Class instance members: `rect.` shows fields and methods
  - Trait methods: Shows methods from `impl Trait for Class` blocks
  - Keywords and local variables
- **Go to Definition**: Navigate to symbol definitions
- **Diagnostics**: Compile errors shown in editor

Build and run LSP:
```bash
go build -o gecko-lsp ./lsp
```

### Not Yet Implemented

- Copy/clone hooks (automatic trigger on assignments)
- String iteration (strings need Iterator impl)
- Default trait implementations
- Trait inheritance
- LSP import suggestions for unknown types

## Ground Rules for Development

These rules govern how Claude should approach problems in this codebase:

1. **HALT on complex problems with easy workarounds**: When encountering a problem that would be complex to solve properly but has an easy shortcut (symlinks, hardcoding, copy-paste), STOP and communicate the issue. Never take shortcuts that mask underlying design problems.

2. **Spec requirements over convenience**: When encountering a loophole or gap that prevents safe implementation, mark it as a **spec requirement** and HALT. The language spec should be extended rather than working around gaps with clever code.

3. **Types over algorithms**: Prefer solving safety issues through the type system rather than runtime checks or algorithms. If you find yourself writing algorithmic safety checks, flag it so we can ideate a type system feature instead.

4. **Verify with LSP and tests**: Never assume code works. Always:
   - Run relevant tests (`go test ./tests/... -v`)
   - Check LSP behavior for editor features
   - Test examples end-to-end (`go run . run <file>`)

5. **Be a language user**: Approach Gecko as a user would. If something feels wrong or unsafe from a user perspective, that's a language design issue to discuss, not a compiler bug to hack around.

6. **Check existing features before adding new ones**: Before implementing a new feature, verify how it interacts with existing features. Ask questions like: "Does this conflict with X?", "What happens if both Y and Z are used together?". Add tests for edge cases and interactions, not just the happy path.
