# Scoping and Symbol Resolution

This document describes how the Gecko compiler manages scopes, resolves symbols, and mangles names for C code generation.

## Overview

Gecko uses a hierarchical scope tree where each AST node can contain symbols (variables, classes, traits, methods) and child scopes (imported modules). Symbol resolution walks up this tree until a match is found.

## Scope Hierarchy

```
Root Scope (package main)
├── Variables: { x, y }
├── Classes: { Point, Circle }
├── Traits: { Drawable }
├── Methods: { main, helper }
├── Children (imported modules):
│   ├── math (package math)
│   │   ├── Classes: { Vector }
│   │   └── Methods: { sqrt, sin, cos }
│   └── io (package io)
│       └── Methods: { print, read }
└── Class Scopes:
    └── Point
        ├── Variables: { x, y }  (fields)
        ├── Methods: { new, distance }
        └── Traits: { Drawable: [...] }  (implementations)
```

## Symbol Storage

### Current Implementation

| Symbol Type | Storage Location | Key Format | Notes |
|-------------|-----------------|------------|-------|
| Variables | `scope.Variables[name]` | Simple name | `"x"`, `"counter"` |
| Classes | `scope.Classes[name]` | Simple name | `"Point"`, `"Vec"` |
| Traits (decl) | `scope.Traits[name]` | Simple name | `"Iterator"`, `"Add"` |
| Traits (impl) | `class.Traits[name]` | Mangled | `"Iterator__int32"` |
| Methods | `scope.Methods[name]` | Simple name | `"new"`, `"calculate"` |
| Modules | `scope.Children[name]` | Package name | `"math"`, `"io"` |

### Issues with Current Implementation

1. **Trait implementation keys inconsistent** - C backend uses mangled names (`Iterator__int32`), LLVM uses plain names (`Iterator`). This causes collisions when implementing multiple instantiations.

2. **No locality flags** - Variables don't have explicit `IsLocal`/`IsGlobal`. Locality is inferred from `variable.Parent == currentScope`.

3. **OriginModule not set** - Classes/methods don't consistently have `OriginModule` set, breaking cross-module visibility checks.

4. **Import scopes orphaned** - Import scopes have `Parent: nil` to avoid deep name mangling, but this breaks upward resolution.

## Name Mangling Rules

### Current Rules

Mangling replaces `.` with `__` to produce valid C identifiers:

| Symbol | Gecko Name | Mangled C Name |
|--------|-----------|----------------|
| Global variable | `mymodule.counter` | `mymodule__counter` |
| Class | `shapes.Point` | `shapes__Point` |
| Method | `shapes.Point.new` | `shapes__Point__new` |
| Trait method | `Point` impl `Add<Point>` `.add` | `Point__Add__Point__add` |
| External | `printf` | `printf` (no mangling) |

### Mangling Logic

```
GetFullName():
    if IsExternal or Parent == nil:
        return Name  # No mangling
    else:
        return Parent.FullScopeName().replace(".", "__") + "__" + Name
```

### Proposed Clarifications

1. **Package name = root scope name** - The `package` declaration sets the root `Scope` field.
2. **Imports don't add prefix** - Imported symbols keep their original module prefix, not `main__module__symbol`.
3. **External bypasses all mangling** - `external` visibility means C ABI name.

## Symbol Resolution

### Variable Resolution (`ResolveSymbolAsVariable`)

```
fn resolve(scope, name):
    while scope != nil:
        if name in scope.Variables:
            return scope.Variables[name]
        scope = scope.Parent
    return NotFound
```

### Class Resolution (`ResolveClass`)

```
fn resolve(scope, name):
    while scope != nil:
        if name in scope.Classes:
            return scope.Classes[name]
        scope = scope.Parent
    
    # Fallback: lazy resolution for directory imports
    if scope.LazyResolver != nil:
        return scope.LazyResolver(name)
    
    return NotFound
```

### Cross-Module Resolution

For `module.Symbol`:
1. Look up `module` in `scope.Children`
2. Look up `Symbol` in `module.Classes`, `module.Methods`, etc.

```gecko
import math

let v: math.Vector = math.Vector::new(1.0, 2.0)
//     ^-- Resolve "math" in Children, then "Vector" in math.Classes
```

## Visibility Rules

### Modifiers

| Modifier | Meaning | Name Mangling |
|----------|---------|---------------|
| (none) | Private to module | Yes |
| `public` | Accessible from other modules | Yes |
| `external` | C ABI export | No |

### Enforcement Points

1. **Type usage** - When referencing a type, check visibility
2. **Method calls** - When calling a method, check visibility
3. **Field access** - When accessing a field, check visibility

### Current Issues

- `OriginModule` not consistently set on symbols
- Visibility checks only partially implemented
- `external` on methods prevents mangling but doesn't check access

## Proposed Unified Model

### 1. Consistent Trait Implementation Keys

**Change:** Both C and LLVM backends use mangled trait names for `class.Traits`:

```go
// Always use mangled name for generic traits
mangledTraitName := traitName
for _, typeArg := range typeArgs {
    mangledTraitName += "__" + TypeRefToCType(typeArg)
}
class.Traits[mangledTraitName] = &methodList
```

### 2. Explicit Variable Locality

**Change:** Add `IsGlobal` flag to `ast.Variable`:

```go
type Variable struct {
    Name       string
    IsPointer  bool
    IsConst    bool
    IsVolatile bool
    IsExternal bool
    IsArgument bool
    IsGlobal   bool  // NEW: explicit global flag
    Parent     *Ast
}
```

### 3. Consistent OriginModule

**Change:** Set `OriginModule` when creating any symbol:

```go
// In NewClass, NewMethod, NewTrait, etc.
symbolAst.OriginModule = scope.GetRoot().Scope
```

### 4. Import Scope Parenting

**Change:** Import scopes have proper parent chain, but mangling uses `OriginModule`:

```go
// Import scope creation
importScope := &ast.Ast{
    Scope:        importedFile.PackageName,
    Parent:       rootScope,  // Proper parent for resolution
    OriginModule: importedFile.PackageName,  // For mangling
}

// Mangling uses OriginModule, not parent chain
func (a *Ast) GetFullName() string {
    if a.OriginModule != "" {
        return strings.ReplaceAll(a.OriginModule + "." + a.Scope, ".", "__")
    }
    // ... existing logic
}
```

### 5. Resolution Order

Unified resolution order for all symbol types:

1. **Local scope** - Check current scope's map
2. **Parent chain** - Walk up `Parent` pointers
3. **Imported modules** - Check `Children` map at root
4. **Lazy resolution** - For directory imports, search files
5. **Error** - Report with suggestions

## State Machine for Type System

Before implementing full type checking, the scoping model must be solid:

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│   Parse     │────▶│ Build Scope  │────▶│  Resolve    │
│   Tokens    │     │    Tree      │     │   Types     │
└─────────────┘     └──────────────┘     └─────────────┘
                           │                    │
                           ▼                    ▼
                    ┌──────────────┐     ┌─────────────┐
                    │  Set Origin  │     │   Check     │
                    │   Module     │     │  Visibility │
                    └──────────────┘     └─────────────┘
                                               │
                                               ▼
                                        ┌─────────────┐
                                        │   Type      │
                                        │   Check     │
                                        └─────────────┘
```

## Lazy Resolution

Directory imports support on-demand resolution of types and methods. When a symbol cannot be found in the scope hierarchy, the compiler searches imported directories for matching definitions.

### Type Resolution

```go
LazyTypeResolver func(typeName string) (*Ast, bool)
```

Called when `ResolveClass()` fails to find a type in the scope hierarchy. Searches directory imports for a matching class or trait definition.

### Method Resolution

```go
LazyMethodResolver func(methodName string) (*Method, bool)
```

Called when `ResolveMethod()` fails to find a method in the scope hierarchy. Searches directory imports for a matching top-level function.

### File Caching

To avoid processing the same file multiple times (when multiple symbols from one file are used), backends maintain a `lazyResolvedFiles` map keyed by file path. When a file is lazily resolved:

1. Check if already in cache → return existing scope
2. Create new scope and process entries
3. Add to `file.Children` for future resolution
4. Cache for future lazy resolutions

## Completed Migration

- **Phase 1:** Fix LLVM trait storage - mangled trait names consistent with C backend
- **Phase 2:** Set OriginModule consistently on all AST nodes
- **Phase 3:** Add explicit IsGlobal flag to Variable struct  
- **Phase 4:** Extend lazy resolution to include methods
- **Phase 5:** Document final model in spec/scoping.md
