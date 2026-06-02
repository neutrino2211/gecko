# Scoping and Symbol Resolution

This document defines Gecko's scope model, symbol lookup order, and C name mangling rules.

## Overview

Gecko uses a hierarchical scope tree.
Each scope may define symbols and child module scopes.
Symbol resolution is explicit and deterministic.

## Scope Hierarchy

```
Root Scope (package main)
├── Variables: { x, y }
├── Classes: { Point, Circle }
├── Traits: { Drawable }
├── Methods: { main, helper }
├── Children (imported modules):
│   ├── math
│   │   ├── Classes: { Vector }
│   │   └── Methods: { sqrt, sin, cos }
│   └── io
│       └── Methods: { print, read }
└── Class Scopes:
    └── Point
        ├── Variables: { x, y }  (fields)
        ├── Methods: { new, distance }
        └── Traits: { Drawable: [...] }  (implemented traits)
```

## Symbol Storage

| Symbol Type | Storage Location | Key Format |
|-------------|------------------|------------|
| Variables | `scope.Variables[name]` | simple name |
| Classes | `scope.Classes[name]` | simple name |
| Traits (declarations) | `scope.Traits[name]` | simple name |
| Methods | `scope.Methods[name]` | simple name |
| Modules | `scope.Children[name]` | import module name |
| Trait impl methods | `class.Traits[name]` | mangled trait key |

## Module Identity

Module identity uses:

1. canonical file path (for de-duplication/caching), and
2. import module name (for namespace lookup in `scope.Children`).

Import scope keys must not rely on package name alone.

## Name Mangling

Gecko mangles non-external symbols to C identifiers using `__` separators.

### Rules

1. `external` symbols keep their exact ABI name (no mangling).
2. Module/class/method paths are flattened by replacing `.` with `__`.
3. Trait implementation methods include type + trait + method components.
4. Generic trait impl names append concrete type arguments in mangled form.

### Examples

| Symbol | Gecko Name | Mangled C Name |
|--------|-----------|----------------|
| Module function | `math.sqrt` | `math__sqrt` |
| Class | `shapes.Point` | `shapes__Point` |
| Class method | `shapes.Point.new` | `shapes__Point__new` |
| Trait method | `Point` impl `Add<int>` `.add` | `Point__Add__int__add` |
| External | `printf` | `printf` |

## Resolution Order

For unqualified symbols:

1. current scope
2. parent scopes (walk `Parent` chain)
3. error

For module-qualified symbols (`module.Symbol`):

1. resolve `module` in root `Children`
2. resolve `Symbol` in that module scope
3. if missing, try lazy module resolution (directory imports)
4. error with suggestions

For type and method fallback from directory imports:

1. normal scope lookup
2. lazy resolver (`LazyResolver` / `LazyMethodResolver`)
3. cache resolved file scope
4. error if still missing

## Lazy Resolution

Directory imports are resolved on-demand:

1. symbol lookup misses in existing scopes
2. compiler searches imported directories for matching file/type/method
3. resolved file is parsed once and cached by canonical path
4. resolved scope is attached under root module children

## Visibility Checks

Visibility uses file/package boundaries:

| Modifier | Access |
|----------|--------|
| (none) / `private` | current file |
| `protected` | current package |
| `public` | any importing package |
| `external` | public + C ABI name |

Visibility is enforced at:

1. type references
2. method calls
3. field access

## Trait Method Conflicts

If a type implements multiple traits that expose the same unqualified method name, plain method calls are ambiguous unless disambiguated.
