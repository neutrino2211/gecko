# Gecko Language Specification

This directory contains the specification for the Gecko programming language as currently implemented.

## Status

This spec documents **what exists**, not aspirational features. Each document notes known gaps and unimplemented areas.

## Contents

| Document | Description |
|----------|-------------|
| [types.md](types.md) | Primitive types, type references, arrays |
| [functions.md](functions.md) | Function declarations, parameters, return types |
| [classes.md](classes.md) | Class/struct definitions, fields, methods |
| [traits.md](traits.md) | Trait definitions, implementations, compiler hooks |
| [generics.md](generics.md) | Generic type parameters, monomorphization |
| [modules.md](modules.md) | Module system, imports, visibility |
| [control-flow.md](control-flow.md) | Conditionals, loops, early returns |
| [operators.md](operators.md) | Arithmetic, comparison, logical, bitwise |
| [memory.md](memory.md) | Pointers, references, address-of |
| [c-interop.md](c-interop.md) | External declarations, C ABI |
| [attributes.md](attributes.md) | Compile-time attributes, trait hooks |
| [stdlib.md](stdlib.md) | Standard library structure and modules |

## Design Principles

1. **Types solve problems** - Use the type system over runtime constructs
2. **Explicit over implicit** - No prelude, no hidden allocations, no magic
3. **Hooks over hardcoding** - Compiler provides capabilities, developers wire them up
4. **Private by default** - Explicit `pub` required for exports
5. **C ABI compatibility** - Seamless interop with C libraries
6. **Freestanding capable** - Core language and stdlib work without libc
