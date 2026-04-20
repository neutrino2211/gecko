# Gecko Language Support for VS Code

Language support for the [Gecko programming language](https://github.com/neutrino2211/gecko).

## Features

- Syntax highlighting for `.gecko` files
- Real-time error diagnostics (via LSP)
- Bracket matching and auto-closing
- Code folding
- Comment toggling (`Cmd+/` or `Ctrl+/`)

## Installation

### 1. Build the LSP Server

From the Gecko repository root:

```bash
go build -o gecko-lsp ./lsp/
```

Add the binary to your PATH, or configure the path in VS Code settings.

### 2. Install the Extension

#### From Source (Development)

```bash
cd editors/vscode
npm install
npm run compile
```

Then symlink to your extensions directory:

```bash
# macOS/Linux
ln -s $(pwd) ~/.cursor/extensions/gecko-lang
# or for VS Code
ln -s $(pwd) ~/.vscode/extensions/gecko-lang
```

Restart your editor.

#### From VSIX (Package)

```bash
cd editors/vscode
npm install
npm run compile
npm run package
```

Install the generated `.vsix` file via the Extensions view.

## Configuration

| Setting | Default | Description |
|---------|---------|-------------|
| `gecko.lsp.enabled` | `true` | Enable/disable the language server |
| `gecko.lsp.path` | `""` | Path to `gecko-lsp` binary (uses PATH if empty) |

## Highlighted Elements

| Element | Example |
|---------|---------|
| Keywords | `if`, `else`, `for`, `while`, `return` |
| Declarations | `func`, `class`, `trait`, `impl`, `enum` |
| Modifiers | `let`, `const`, `public`, `private`, `volatile` |
| Types | `int`, `uint64`, `string`, `bool`, `void` |
| Attributes | `@packed`, `@section(".text")` |
| Intrinsics | `@deref(ptr)`, `@size_of<T>()` |
| Static calls | `Point::new()` |

## LSP Features

Currently implemented:
- [x] Diagnostics (parse errors)

Planned:
- [ ] Hover (type information)
- [ ] Go to definition
- [ ] Completions
- [ ] Find references

## Contributing

This extension is part of the Gecko compiler repository. Issues and PRs welcome!
