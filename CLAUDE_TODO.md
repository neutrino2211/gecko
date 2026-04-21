# Module System & Stdlib Implementation TODO

Tracking implementation of the new module system per specs in `spec/modules.md`, `spec/traits.md`, `spec/attributes.md`, and `spec/stdlib.md`.

## Phase 1: Parser Changes ✅ COMPLETE

- [x] **Dot notation imports** - `import std.collections.vec`
  - Updated `Import` struct to use `Path []string` instead of `Package string`
  - Added `Package()` and `ModuleName()` helper methods
  - Updated `compiler/compiler.go` to resolve dot-separated paths

- [x] **`public` visibility modifier** (already existed, just documented)
  - Visibility options: `private`, `public`, `protected`, `external`
  - Private by default, `public` for exported symbols

- [x] **Hook attributes** - `@drop_hook(.method)`
  - Updated `Attribute` struct to support `Args []*AttributeArg`
  - `AttributeArg` can be either `String` or `Method` (for `.methodname`)
  - Added `GetStringValue()` and `GetHookMethods()` helper methods
  - Reordered Entry parser to parse declarations before intrinsics

## Phase 2: Compiler Changes ✅ COMPLETE

- [x] **Module resolution order** ✅
  - Relative: `./path/module.gecko`
  - Stdlib: `$GECKO_HOME/stdlib/path/module.gecko`
  - Support `mod.gecko` for directory modules
  - `std.*` imports strip the `std` prefix and search directly in `$GECKO_HOME/stdlib/`
  - Vendor path: `./vendor/path/module.gecko`
  - `getGeckoHome()` checks `$GECKO_HOME` env, then OS-specific defaults

- [x] **Directory imports with lazy resolution** ✅
  - [x] Infrastructure: `DirectoryImport` struct, lazy resolver callback
  - [x] Type lookup triggers lazy parsing of directory files
  - [x] **Inherent `impl` blocks** - `impl ClassName { ... }` now dispatches to `CInherentImplementation`
  - [x] **Symbol resolution via `package`** - Lazy-resolved modules use their `package` declaration as scope prefix (e.g., `point__Point__new`)
  - [x] **Qualified type syntax** (`shapes.Circle`) - Implemented via `LazyModuleTypeResolver`

- [x] **Type suggestion on unresolved types** ✅
  - `TypeRegistry` scans stdlib and project directories for types
  - `FormatSuggestions()` generates helpful import suggestions
  - Test: `test_sources/compile_tests/type_suggestions/missing_type.gecko`

- [x] **Hook registry** ✅
  - 19 hook types defined in `hooks/hook_registry.go`
  - `ProcessTraitHooks()` validates signatures and registers traits
  - Duplicate hook detection with errors
  - Tests: `hook_invalid_method`, `hook_duplicate`, `hook_wrong_signature`

- [x] **Visibility enforcement** ✅
  - Private by default for methods in impl blocks and class bodies
  - Fixed `ToMethodToken()` to preserve visibility (was hardcoding "public")
  - Fixed inline class method registration to include visibility
  - Added visibility checking in `FuncCallToCString` and `processChain`
  - Tests: `visibility_private_access`, `visibility_private_method`

- [x] **Code generation for hooks** ✅
  - `@drop_hook`: Cleanup calls at scope exit via `generateDropCalls()` (LIFO order)
  - `@add_hook` etc.: Operator desugaring via `GetOperatorTraitMethodCall()`
  - `@index_hook`: Array indexing desugars to `get()` method calls
  - `@iterator_hook`: For-in loops use `next()` and `has_next()` methods
  - Tests: `index_hook/`, `for_in_loop/`, `hooks/`

- [x] **Generic trait implementations** ✅
  - `impl<T> Trait for Class<T>` now stores with generic class for later instantiation
  - Fixed: `use { ... }` clause symbols now properly imported for nested imports
  - Test: `generic_trait_impl/main.gecko`

- [x] **Stdlib trait implementations** ✅
  - Added trait impls to stdlib types:
    - `Vec<T>`: Drop, Clone, Index, IndexMut
    - `String`: Drop, Clone, Index
    - `StringBuilder`: Drop, Index
    - `Box<T>`: Drop
    - `Rc<T>`: Drop, Clone
    - `Slice<T>`: Clone, Index

- [x] **Copy/clone hooks** - REMOVED BY DESIGN
  - Clone works as a regular trait - call `.clone()` explicitly when needed
  - Copy is a marker trait only (no hook) - Gecko uses C-style implicit bitwise copy
  - Move semantics not planned - explicit memory management is the intended model

## Phase 3: Stdlib Consolidation ✅ COMPLETE

- [x] **Created new directory structure**
  ```
  stdlib/
  ├── mod.gecko           # Package: std
  ├── core/
  │   ├── mod.gecko       # Package: core
  │   ├── traits.gecko    # Drop, Clone, Copy, Iterator, Index traits
  │   └── ops.gecko       # Add, Sub, Mul, Div, Eq, etc. operator traits
  ├── collections/
  │   ├── mod.gecko       # Package: collections
  │   ├── vec.gecko       # Package: vec
  │   ├── slice.gecko     # Package: slice
  │   └── string.gecko    # Package: string (String, StringBuilder, str_len, streq)
  ├── memory/
  │   ├── mod.gecko       # Package: memory
  │   ├── box.gecko       # Package: box
  │   ├── rc.gecko        # Package: rc
  │   ├── weak.gecko      # Package: weak
  │   └── raw.gecko       # Package: raw
  └── option.gecko        # Package: option
  ```

- [x] **Added hook attributes to core traits**
  - `@drop_hook(.drop)` on Drop trait
  - `@clone_hook(.clone)` on Clone trait
  - `@copy_hook(.copy)` on Copy trait
  - `@iterator_hook(.next, .has_next)` on Iterator trait
  - `@into_iterator_hook(.iter)` on IntoIterator trait
  - `@index_hook(.index)` on Index trait
  - `@index_mut_hook(.index_mut)` on IndexMut trait
  - All operator hooks (@add_hook, @sub_hook, @eq_hook, etc.) on operator traits

- [x] **Updated imports in examples and tests**
  - `import std.collections.string` for StringBuilder
  - `import std.collections.slice` for Slice
  - Test expectation updated for new module paths

- [x] **Deleted old `std/` directory**

## Phase 4: LSP Updates ✅ COMPLETE

- [x] **Import suggestions for unknown types** ✅
  - Already implemented via `TypeRegistry` and `SuggestionProvider`
  - Scans stdlib and project directories for type definitions
  - Shows suggestions in diagnostic error messages
  - Test: `test_sources/compile_tests/type_suggestions/missing_type.gecko`

- [x] **Respect visibility in completions** ✅
  - Module-qualified type completions filter by visibility
  - Imported class member completions filter by visibility
  - Inherent impl methods now correctly detected
  - Fixed `FormatTypeRef` to include module prefix
  - Added test fixtures: `lsp/testdata/`

## Phase 5: Documentation ✅ COMPLETE

- [x] Update CLAUDE.md with new import syntax
- [x] Update examples to use new patterns (string_builder, traits)
- [x] Add stdlib usage examples in CLAUDE.md

---

## Stability Audit (2026-04-20)

Comprehensive audit of the codebase for inconsistencies and gaps before adding new features.

### P0 - Critical (Type Safety) - COMPLETED 2026-04-20

These issues allowed incorrect code to compile silently:

- [x] **Trait constraint validation** - `T is Area` constraints now validated
  - Location: `backends/c_backend/c_typecheck.go:350-375`
  - Fix: Added `TypeImplementsTrait()` check during generic function calls
  - MethodSignature now stores full `[]*tokens.TypeParam` with constraints

- [x] **Struct literal field type checking** - Fields now validated
  - Location: `backends/c_backend/c_expressions.go:295`, `c_typecheck.go:608-663`
  - Fix: Added `CheckStructLiteralTypes()` called during struct literal code gen

- [x] **Generic class type arg validation** - Type args now checked against constraints
  - Location: `backends/c_backend/c_expressions.go:630-633, 657`
  - Fix: Added `ValidateClassTypeArgs()` for static method calls on generic classes

- [x] **Silent type checking failures** - Now emit ERRORS instead of silent skips ✅ FIXED
  - Added `TypeCheckError()` function - type safety gaps are compile errors
  - Fixed bug in `errors.go` where warnings were appended to errors list
  - Created global ErrorScope registry for centralized error collection
  - Fixed generic field access: register fields AND methods for generic classes
  - Fixed pointer builtin methods: `ptr.offset()`, `ptr.read()`, `ptr.write()` in `GetTypeOfFuncCall`
  - Fixed generic method return types: substitute `T` with actual type arg (e.g., `Vec<int32>.get()` returns `int32`)
  - All 36 compile tests + type checking error tests pass

### P1 - High (Grammar & Testing)

- [x] **Implement `Enum`** - Basic C-style enums for FFI ✅
  - Location: `backends/c_backend/c_backend.go:NewEnum`
  - Syntax: `enum Color { Red Green Blue }`
  - Value access: `Color.Red` -> `enums__Color_Red`
  - Generates C typedef enum for direct C interop

- [x] **Implement `CImport`** - Now generates `#include` directives in C backend ✅
  - Location: `backends/c_backend/c_backend.go:NewCImport`
  - Syntax: `cimport "<stdio.h>"` for system headers, `cimport "local.h"` for local
  - Supports `withobject` and `withlibrary` for linking (stored in token for build stage)

- [x] **Enable unrun test directories** - Partially complete ✅
  - Added: `packed/`, `struct_literal/`, `struct_inline/`, `fixed_arrays/`, `typecheck_valid/`, `enums/`
  - Remaining (need work): `array_index/` (no main), `asm/` (x86 only), `attributes/` (section attr),
    `casts/` (old syntax), `comprehensive/` (old syntax), `globals/` (section attr),
    `imports/` (no main), `loops/` (no main), `strings/` (conflict), `volatile/` (C error),
    `effects_basic/` (throws feature incomplete)

- [x] **Add examples to test suite** ✅
  - `examples/hello_kernel/` - requires QEMU, not suitable for automated tests
  - `examples/string_builder/` - added (fixed `pub` → `public` keyword)
  - Deleted `examples/stdlib_showcase/` (broken Option type usage)

### P2 - Medium (Backend & UX) - COMPLETED 2026-04-20

- [x] **LLVM backend marked experimental** ✅
  - Added runtime warning when `--backend llvm` is used
  - Warning: "LLVM backend is experimental, many features are unsupported"
  - C backend remains recommended for production use

- [x] **Numeric type compatibility** ✅
  - Added lossy conversion warnings for: arguments, assignments, returns, struct literals
  - Functions: `getNumericBitSize()`, `isSignedType()`, `IsLossyConversion()`
  - Warns on: larger-to-smaller conversions, signed-to-unsigned of same size

- [x] **Inconsistent error messages** ✅
  - Standardized all error titles to Title Case category names
  - Fixed: "Cannot infer type" → "Type Inference Error"
  - Fixed: "Uninitialized constant" → "Uninitialized Constant"
  - Fixed: "Cannot Reassign Constant" → "Constant Reassignment"

- [x] **Default impl validation** ✅ (already implemented)
  - Location: `c_backend.go:1393-1421`
  - Required methods (no body in trait) must exist on class
  - Error: "Missing Required Method: Class 'X' cannot implement trait 'Y'"

- [x] **Generic type checking gaps** ✅
  - Generic class fields/methods now registered for type checking
  - Pointer builtin methods (offset, read, write) handled in `GetTypeOfFuncCall`
  - Generic return types substituted: `T` → actual type arg

### P3 - Low (Edge Cases) - MOSTLY COMPLETED 2026-04-20

- [x] **Nested generics (3+ levels)** ✅
  - Fixed struct literal type inference for generic types
  - Added `StructTypeArgs` field to Literal for `Box<T> { ... }` syntax
  - Test: `test_sources/compile_tests/nested_generics/main.gecko`

- [x] **Multiple trait constraints** ✅
  - Grammar: `T is A & B` now supported
  - Updated `TypeParam.Traits []string` and `AllTraits()` helper
  - `MonomorphContext.Constraints` now `map[string][]string`
  - `FindTraitWithMethod()` searches all constraints to find the right trait
  - Test: `test_sources/compile_tests/multiple_constraints/main.gecko`

- [x] **Trait method conflicts** ✅
  - Detection added in `CImplementationForClass`
  - Error: "Trait Method Conflict: Method 'X' is defined in both trait 'A' and trait 'B'"
  - Test: `test_sources/compile_tests/trait_conflicts/conflict.gecko`

- [x] **Qualified type syntax** (`module.Type`) ✅
  - Syntax `shapes.Circle` now works for types from directory imports
  - Type checking uses `LazyModuleTypeResolver` to resolve module-qualified types
  - C codegen: typedef uses simple name (`Circle`), methods use scoped name (`shapes__Circle__new`)
  - Test: `test_sources/compile_tests/qualified_types/main.gecko`

- [ ] **Pointer arithmetic overflow** - Low priority for systems language (intrinsics-only currently)

- [x] **Circular type dependencies** ✅
  - Detects cycles in non-pointer (value) field dependencies
  - Reports "Circular Type Dependency" error with cycle path
  - Pointer cycles are allowed (they have fixed size)
  - Tests: `test_sources/compile_tests/circular_deps/`

---

## Notes

- All changes must pass existing tests
- Parser changes should be backwards compatible where possible
- Hook system is opt-in - no behavior change without hook attributes
