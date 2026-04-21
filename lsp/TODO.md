# LSP Completion Improvements

Ordered by complexity (simplest first):

## 1. Enum Variant Completions
**Complexity:** Low
**Status:** DONE

Add completions for enum variants when typing after an enum type name.
Example: `Color::` should suggest `Red`, `Green`, `Blue`

Implemented:
- `symbols.go`: `getStaticMethodCompletions()` - added enum case lookup
- `symbols.go`: `getImportedModuleCompletions()` - added enum type completions
- `symbols.go`: `getEntryCompletions()` - added local enum completions
- Added tests in `completions_test.go`

---

## 2. Nested Scope Resolution
**Complexity:** Low-Medium
**Status:** DONE

Variables declared in nested blocks (if/while/for) now appear correctly in completions.
Properly respects block scope boundaries - variables only visible when cursor is inside the block.

Implemented:
- `symbols.go`: Added `getVarsFromIfBlock()`, `getVarsFromElseIfBlock()`, `getVarsFromLoopBlock()`
- `symbols.go`: Added `lookupVarInIfBlock()`, `lookupVarInElseIfBlock()`, `lookupVarInLoopBlock()`
- Handles if/else-if/else chains
- Handles while loops
- Added tests in `completions_test.go`

Note: For-in loop variables need proper iterator expression to parse

---

## 3. Generic Type Completions
**Complexity:** Medium
**Status:** DONE

Completions for generic types now substitute type arguments into method signatures.
Example: `Container<int>` shows methods with `int` instead of `T`.

Implemented:
- `symbols.go`: `ParsedGenericType` struct and `parseGenericType()` to extract base type and type args
- `symbols.go`: `substituteTypeParams()` and `replaceTypeParam()` for type parameter substitution
- `symbols.go`: `getClassMemberCompletionsGeneric()` - substitutes type params in class field/method types
- `symbols.go`: `getTraitMethodCompletions()` - now accepts type args and substitutes in impl methods
- Added test `TestGenericTypeCompletions` in `completions_test.go`

---

## 4. Method Signature Parameter Hints
**Complexity:** Medium
**Status:** DONE

Shows parameter information when typing inside function call parentheses.
Highlights the active parameter based on cursor position (comma count).

Implemented:
- `server.go`: Added `SignatureHelpProvider` capability with `(` and `,` triggers
- `server.go`: Added `handleSignatureHelp` handler
- `symbols.go`: `GetSignatureHelp()` - parses line to find function call and active param
- `symbols.go`: `findFunctionSignature()`, `findStaticMethodSignature()`, `findMethodSignature()`
- `symbols.go`: `buildSignatureInfo()`, `buildImplSignatureInfo()` - build LSP response
- `symbols.go`: Updated `sanitizeForParsing()` to handle incomplete function calls
- Added tests: `TestSignatureHelpFreeFunction`, `TestSignatureHelpSecondParameter`, 
  `TestSignatureHelpStaticMethod`, `TestSignatureHelpInstanceMethod`

---

## 5. Import Suggestions
**Complexity:** Medium-High
**Status:** DONE

When a type is used but not imported, provides code actions to add the import.
Works with diagnostics that indicate unresolved/unknown types.

Implemented:
- `server.go`: Added `CodeActionProvider` capability and `handleCodeAction` handler
- `symbols.go`: `GetCodeActions()` - analyzes diagnostics and suggests imports
- `symbols.go`: `findImportInsertionLine()` - finds where to insert imports
- `symbols.go`: `isUnresolvedTypeDiagnostic()`, `extractTypeNameFromDiagnostic()`
- `symbols.go`: `createImportAction()` - creates LSP code action with edit
- `diagnostics.go`: `pathToURI()` helper function
- Added test `TestCodeActionsUnresolvedType`

---

## 6. Stdlib Discovery
**Complexity:** High
**Status:** DONE

Suggests stdlib types that aren't yet imported when typing type names.
Types are shown in completions with import path information.

Implemented:
- `stdlib_index.go`: New file with `StdlibIndex` struct
- `stdlib_index.go`: `StdlibExport` - represents a single exported symbol
- `stdlib_index.go`: `GetStdlibIndex()` - lazily initializes global index
- `stdlib_index.go`: `FindByName()`, `FindByPrefix()`, `AllExports()`
- `stdlib_index.go`: `buildIndex()` - scans stdlib and indexes public exports
- `stdlib_index.go`: `getStdlibPath()` - finds stdlib via GECKO_HOME or relative paths
- `symbols.go`: `getStdlibCompletions()` - suggests stdlib types in completions
- Added tests `TestStdlibIndex`, `TestStdlibCompletions`
