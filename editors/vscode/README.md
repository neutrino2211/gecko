# Gecko Language Support for VS Code

Syntax highlighting for the [Gecko programming language](https://github.com/neutrino2211/gecko).

## Features

- Syntax highlighting for `.gecko` files
- Bracket matching and auto-closing
- Code folding
- Comment toggling (`Cmd+/` or `Ctrl+/`)

## Installation

### From Source (Development)

1. Copy or symlink this folder to your VS Code extensions directory:

   ```bash
   # macOS
   ln -s /path/to/gecko/editors/vscode ~/.vscode/extensions/gecko-lang

   # Linux
   ln -s /path/to/gecko/editors/vscode ~/.vscode/extensions/gecko-lang

   # Windows (PowerShell as Admin)
   New-Item -ItemType SymbolicLink -Path "$env:USERPROFILE\.vscode\extensions\gecko-lang" -Target "C:\path\to\gecko\editors\vscode"
   ```

2. Restart VS Code

### From VSIX (Package)

1. Install `vsce` if you haven't:
   ```bash
   npm install -g @vscode/vsce
   ```

2. Package the extension:
   ```bash
   cd editors/vscode
   vsce package
   ```

3. Install the `.vsix` file:
   - Open VS Code
   - Press `Cmd+Shift+P` (or `Ctrl+Shift+P`)
   - Type "Install from VSIX"
   - Select the generated `.vsix` file

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
| Constants | `true`, `false`, `nil` |
| Comments | `//`, `///`, `/* */` |

## Contributing

This extension is part of the Gecko compiler repository. Issues and PRs welcome!
