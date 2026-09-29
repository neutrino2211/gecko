# Language server status

The server currently provides diagnostics, hover, completion, go to definition,
references, rename, signature help, import suggestions, document and workspace
symbols, semantic tokens, and inlay hints.

## Current behavior

- Compiler checks use the open document under its real path. Import resolution
  reads other open documents before disk and uses the nearest `gecko.toml`.
- Editor analysis follows transitive imports and reuses parsed modules across
  repeated imports and cycles.
- Checks use the project's backend and default target, including its effective
  architecture, platform, vendor, and target-specific native settings. An
  editor target setting can override the default target.
- Open `gecko.toml` buffers take precedence over disk. Config edits and watched
  file changes refresh open project files; malformed TOML is diagnosed on the
  config file instead of checking sources with fallback settings.
- Changes to imported files refresh open importers through cached transitive
  dependencies, including intermediate modules and directory imports. Open
  buffers and watched source changes invalidate affected dependency closures.
- During an incomplete edit, analysis can mask a malformed line without
  shifting later source positions or close an unfinished function body to keep
  its completed bindings. It can also discard an unfinished multiline
  declaration with an open parameter list, generic header, or body while
  retaining later top-level declarations.
  The longest valid prefix remains a fallback.
- Diagnostics retain their source file, severity, and available error code.
  When the compiler reports a source offset, the range covers the full token.
  Compiler messages can carry an exact end offset, related locations, and
  suggested edits. Trait override conflicts use all three; the suggested
  return-type change appears as an explicit quick fix. Import errors highlight
  the import path, unsafe intrinsic errors highlight the full intrinsic name,
  unresolved imports carry a machine-readable code, and syntax errors retain
  the parser's byte offset.
  The server publishes versioned results, clears them on close, and waits
  briefly after edits before checking again.
- Text positions at the protocol boundary use UTF-16. File URIs are encoded
  and decoded.
- The shared semantic graph records exact declaration and use spans for local
  variables, parameters, resolved calls, and class fields used through access,
  assignment, or struct initializers. It also links class type annotations and
  struct construction to class declarations, plus trait names in parent
  clauses, implementation headers, and type annotations. Explicit
  `import … use { Name }` entries link to their imported declarations. Import
  aliases also link to qualified type and function uses. Trait
  method declarations and resolved calls on trait-typed receivers have semantic
  identities, including inherited and imported methods. Chained method calls
  through fields resolve to their declarations. Qualified child traits can
  resolve methods inherited from imported parent traits; explicit imports select
  the intended trait when other imports have the same name. Trait names in inline
  generic constraints and `where` clauses link to their declarations. Go to
  definition also resolves class names in static calls, explicit type arguments,
  struct type arguments, and casts. Class rename includes indexed static calls.
  Qualified class names retain separate identities when imports share a name.
  Field reads and writes also keep their class identity in that case, and
  destructured field names link to their declarations. Unqualified static calls
  use the explicitly imported class when modules share a name, and ambiguous
  imports do not assign a method identity.
  Imported class names in lambda parameter and return annotations link to
  their declarations. Lambda bodies link parameter uses, captured variables,
  and local bindings, including parameters inferred from a function type.
  Calls on generic receivers link through inline and
  `where` trait bounds on functions and classes when one bound supplies the
  method; conflicting bounds leave the call unlinked. Type parameter declarations
  and uses in classes, traits, functions, and generic implementations have local
  identities for definition, references, and rename. Enum declarations, type
  annotations, cases, and case uses in expressions and match arms share identities
  across imports, including qualified imports. Class names and field names in
  match destructuring patterns link to their declarations.
  Global initializer expressions are also analyzed for semantic links.
  Semantic links are checked first. References scan project source files and
  configured workspace folders, and match declarations across imports. Rename
  supports local symbols and indexed project functions, classes, traits, enums,
  enum cases, and class fields. It rejects invalid names, collisions, and files whose matching
  uses cannot be fully analyzed. Local rename rejects names that would shadow a
  lambda capture.
- Project source discovery is shared by references and workspace symbols,
  cached per root, and follows directory symlinks without revisiting ancestor
  directories. Directory timestamps detect file creation and deletion even
  without watcher events; source save and config refresh also invalidate the
  cache.
- Go to definition also handles receiver types, imported symbols, and inherited
  trait methods for the cases covered by tests.
- Signature help scans across lines and ignores commas in nested calls and
  strings. Generic completion splits nested type arguments correctly.
- Document symbols include top-level declarations, class or trait members,
  implementation methods, and enum cases.
  Workspace symbol search covers configured workspace folders and unsaved open
  buffers. Semantic tokens use resolved symbol occurrences, including namespace
  tokens for import aliases, module qualifiers, and type parameters. Inlay hints show
  inferred local types within the requested range.
- Code actions can sort a contiguous import group and wrap a standalone raw
  operation in an `@unsafe` block. The unsafe action skips declarations that
  would change scope, and import sorting skips comment-separated groups.
  Requested action kinds filter the response.
- Tests cover open imported buffers, project root imports, versioned document
  changes, rapid edits, stdio framing, multiple workspace roots, Unicode text
  positions, and compiler errors for unsafe allocation and moves. Workspace
  symbol scans honor cancellation, and pending requests respond to cancel
  notifications. Protocol tests cover ownership and unsafe syntax fixtures.

## Remaining work

1. Expand semantic identity coverage to unresolved trait member contexts,
   remaining module uses, type contexts, and field contexts. Extend workspace
   rename to other symbols once every use can be indexed safely.
2. Expand syntax recovery for other multiline malformed declarations. Recovery
   currently handles open parameter lists, generic headers, and bodies,
   line-local errors, and missing closing braces, but other malformed constructs
   can still hide later code.
3. Populate exact spans, related locations, and structured fixes for more
   compiler errors. Trait overrides, imports, and unsafe intrinsics have exact
   spans, and syntax errors carry parser offsets. Most other producers still
   report only a start position and fall back to token scanning.
4. Extend token and hint coverage as semantic identity expands. Broaden
   formatting and safety actions beyond contiguous imports and standalone
   unsafe operations.
