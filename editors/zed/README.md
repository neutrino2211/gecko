# Gecko for Zed

This development extension associates `.gecko` files with the Gecko language
server. It uses a temporary JavaScript Tree-sitter grammar for Zed's structural
editing; Gecko syntax is not fully represented by that grammar. The language
server provides diagnostics, navigation, completion, and semantic tokens.

## Install locally

1. From the Gecko repository root, build the language server:

   ```sh
   go build -o gecko-lsp ./lsp
   ```

2. Open the repository in Zed. Run **`zed: install dev extension`** from the
   command palette and select this `editors/zed` directory.
3. Open a `.gecko` file. Zed should identify it as **Gecko** and start
   **Gecko Language Server**. If it does not, run **`zed: open log`** to inspect
   the extension error. Gecko writes `gecko-lsp.log` in the server process's
   temporary directory.

When editing the compiler repository, the extension runs the `gecko-lsp` binary
at its root and sets `GECKO_HOME` there so local standard-library imports
resolve. When editing a project inside it, such as `examples/GNote`, Zed may
open that project as a separate worktree. Set the server path explicitly in
Zed's settings in that case:

```json
{
  "lsp": {
    "gecko-lsp": {
      "binary": {
        "path": "/absolute/path/to/gecko/gecko-lsp",
        "env": {
          "GECKO_HOME": "/absolute/path/to/gecko"
        }
      }
    }
  }
}
```

For other Gecko projects, you can instead install `gecko-lsp` on the
worktree's `PATH`. The explicit `binary.path` setting takes precedence over
the compiler repository binary and `PATH`.

Zed disables semantic tokens by default. To highlight Gecko keywords, literals,
comments, types, and names, add this to Zed's settings:

```json
{
  "languages": {
    "Gecko": {
      "semantic_tokens": "full"
    }
  }
}
```

Rebuild `gecko-lsp` after compiler or language-server changes, then restart the
language server in Zed. Reinstall the development extension after changing its
manifest or Rust launcher.
