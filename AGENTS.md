# AGENTS.md

Code hygiene instructions for the Gecko compiler codebase.

## Before Making Changes

1. Run the full test suite to establish a baseline: `go test ./tests/... -timeout 120s`
2. Read the file you're editing end-to-end before touching it
3. Understand the surrounding code's conventions (naming, error handling, patterns)

## While Editing

- Match existing code style exactly (indentation, naming, comment style)
- Prefer editing existing files over creating new ones
- Keep functions focused on a single responsibility
- Avoid deep nesting — extract helpers when logic branches exceed 3 levels
- Never introduce commented-out code
- Never add comments unless explicitly asked
- Never hardcode values that should be configurable or derived

## Before Committing

1. Run `go build ./...` to verify compilation
2. Run `go test ./tests/... -timeout 120s` to verify no regressions
3. Run `go vet ./...` for static analysis
4. If you modified parser/lexer code, run `go test ./tests/... -run TestCompileAndRun -v` to verify end-to-end behavior
5. Check that generated C output looks correct: `go run . compile --print-ir --ir-only <modified_test_file>`

## Error Handling

- Always check errors — never discard them with `_`
- Wrap errors with context: `fmt.Errorf("parsing type: %w", err)`
- Use early returns to avoid deep nesting in error paths
- Never panic in library code — return errors instead

## Testing

- Add a test for every behavioral change
- Tests go in `test_sources/compile_tests/<feature_name>/main.gecko`
- Register new tests in `tests/compiler_test.go` in the `compileTests` array
- Tests should exercise both the happy path and error cases
- Keep test files self-contained (no external dependencies unless necessary)

## Naming Conventions

- Go code: standard Go conventions (camelCase for unexported, PascalCase for exported)
- Gecko code: PascalCase for types and exported names, camelCase for variables and functions
- Test names: descriptive snake_case matching the feature being tested

## Refactoring

- Never refactor and add features in the same change
- When refactoring, keep all tests passing between commits
- Prefer small, incremental refactors over large rewrites

## Security

- Never log or expose secrets, keys, or credentials
- Never commit secrets or keys to the repository
- Validate all external input at boundaries

## Dependencies

- Before adding a new dependency, check if the functionality already exists in the codebase
- Prefer stdlib over third-party packages
- Document why a new dependency is necessary
