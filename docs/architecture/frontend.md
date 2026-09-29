# Shared frontend

The compiler and language server use the same pipeline:

Source snapshot → lexer tokens → parsed syntax → resolved imports → semantic program.

## Ownership

| Owner | Responsibility |
| --- | --- |
| `parser.ParseSource` | Lex once, retain tokens for highlighting, parse syntax, normalize constraints, preserve exact source ranges. |
| `frontend.Session` | Cache parsing by path and contents; load imports through one source reader; reuse a valid semantic result. |
| `semantic.Program` | Inferred expression types, symbol identities, occurrences, lexical bindings, flow facts, generic member queries, and semantic diagnostics. |
| `analysis.AnalysisContext` | Adapt frontend queries for editor callers. It does not independently load an import graph or infer local variable types. |
| `compiler.CompilePreparedWithDiagnostics` | Reuse frontend work while isolating mutable backend syntax and collecting diagnostics for one compilation. |
| LSP document store | Own one frontend session for open documents and workspace checks. |

`frontend.Result` is the cached snapshot. Its AST, semantic program, source records, and import state are private. `Result.View()` returns a caller-owned syntax and semantic copy for editor work. `Result.CloneForBackend(cfg)` returns a separate copy for compilation. AST-keyed semantic facts are remapped to each copy; changes to a view cannot change the cached result. Views contain mutable Go pointers and must not be shared across independent writers.

## Reuse and invalidation

A session retains the latest parsed contents for each path and the latest analysis for each root. Different roots reuse unchanged imported syntax; they receive independent syntax copies for analysis. `Session.Syntax` supplies a detached syntax view for workspace indexes that do not need semantic analysis.

A semantic result is reused only when its root contents, project configuration, recorded source reads, and relevant directory listings still match. Failed reads are recorded too, so creating a previously missing import invalidates its consumers. Imported source changes rebuild the root's semantic graph but only changed files are reparsed. A cache hit can create another view without lexing or semantic analysis.

Reads use the supplied source reader, including unsaved editor overlays. Directory discovery uses the filesystem. Configuration values are copied for comparison, so editing an existing configuration object cannot make an old result appear current.

Recovery parses retain source offsets by masking invalid text. Recovery uses the document session to share unchanged imports, but a recovered result cannot satisfy a compiler check of the original invalid source.

## Semantic queries

Use `LookupSymbol` and `VisibleSymbols` for names at a cursor position. They use recorded lexical scopes, including shadowing and branch boundaries. Use `SymbolAt` for a resolved occurrence. Do not search every method for a matching local name.

Use `TypeOfExpression` for analyzed expressions. Use `FieldsForType` and `MethodsForType` for generic substitution, and the `From` variants when visibility depends on the requesting file. Symbols retain parameter status, mutability, and local documentation for editor presentation.

`tokens.FormatTypeRef` is the shared textual type formatter. Semantic and analysis wrappers delegate to it.

Self context and unsafe-block metadata belong to an individual semantic analysis. Reference queries have per-file and per-symbol indexes instead of scanning occurrences across the entire import closure.

The LSP uses its document frontend session for workspace symbols, references, and dependency discovery. Open documents reuse their current analysis; closed sources reuse cached syntax or semantic results. An analysis view indexes all files loaded for each module, so hover, definition, and completion can find declarations in different files of one directory import.

## Backend boundary

Backends still mutate syntax during lowering. `CloneForBackend` copies the syntax graph, semantic records, diagnostics, and import state. Directory files referenced by parsed syntax are already in the shared import graph before semantic analysis. A later lazy lookup appends imports only to its caller's view. Syntax identity and semantic identity change together.

The compiler adapter installs unsafe-block metadata into the legacy backend maps. Frontend and backend scopes are collected by each compilation and returned with its artifact; no process-wide diagnostic registry is used. Semantic analysis checks constant initialization and reassignment, return shape, and known struct-literal fields before lowering. Frontend-only LSP queries do not take the compiler lock, and diagnostic requests prepare their frontend snapshots before entering it. Backend checks remain serialized because C lowering still uses global state.

Directory imports selected with `use` and directory symbols referenced by parsed types or calls are loaded before semantic analysis. Unreferenced directory files remain outside the import graph. Legacy demand-driven directory lookups remain available through the shared loader at the backend boundary. This pass does not turn every backend check into a frontend check or provide incremental parsing within a changed file.

Type-suggestion scans run only if a missing-type diagnostic requests suggestions.

## Regression coverage

Frontend tests cover shared imports, changed and newly created dependencies, configuration mutation, directory changes, invalid sources, scope boundaries, concurrent Self inference, and isolation of editor and backend mutations. LSP tests compare prepared and fresh compiler diagnostics, verify reuse across repeated workspace queries, and exercise symbols spread across directory files.
