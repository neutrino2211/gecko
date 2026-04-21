---
title: CLI Reference
description: Gecko compiler commands and options
sidebar:
  order: 1
---

The Gecko compiler provides several commands for compiling, building, and managing projects.

## Commands Overview

| Command | Alias | Description |
|---------|-------|-------------|
| `gecko compile` | `c` | Compile to C/LLVM IR |
| `gecko build` | `b` | Compile to executable |
| `gecko run` | `r` | Compile and run |
| `gecko check` | `ck` | Type-check without compiling |
| `gecko doc` | `d` | Generate documentation |
| `gecko deps` | - | Manage dependencies |

## gecko compile

Compiles Gecko source files to intermediate representation (C or LLVM IR).

```bash
gecko compile [options] <sources...>
gecko compile --entry <name>
```

### Options

| Flag | Description | Default |
|------|-------------|---------|
| `--output-dir` | Output directory | `.` |
| `--entry, -e` | Entry point from gecko.toml | - |
| `--type` | Output type: `executable` or `library` | `executable` |
| `--backend` | Backend: `c` or `llvm` | `c` |
| `--target-arch` | Target architecture | Host arch |
| `--target-platform` | Target OS | Host OS |
| `--target-vendor` | Target vendor | - |
| `--print-ir` | Print generated IR | false |
| `--ir-only` | Only generate IR (no object file) | false |
| `--llc-args` | Arguments for llc (LLVM backend) | - |
| `--log-level` | Logging: silent, error, warn, info, debug, trace | `silent` |

### Examples

```bash
# Compile to C code
gecko compile src/main.gecko

# View generated C code
gecko compile --print-ir --ir-only src/main.gecko

# Compile with debug logging
gecko compile --log-level debug src/main.gecko

# Cross-compile for Linux
gecko compile --target-arch=amd64 --target-platform=linux src/main.gecko
```

## gecko build

Compiles Gecko source to an executable binary.

```bash
gecko build [options] <source.gecko>
gecko build --entry <name>
```

### Options

| Flag | Description | Default |
|------|-------------|---------|
| `--output, -o` | Output executable name | Source name |
| `--release` | Build with optimizations (`-O2`) | false |
| `--keep-c` | Keep generated C file | false |
| All `compile` flags | Inherited from compile | - |

### Examples

```bash
# Build executable
gecko build src/main.gecko -o myapp

# Release build with optimizations
gecko build --release src/main.gecko -o myapp

# Build from gecko.toml entry
gecko build --entry main -o myapp
```

## gecko run

Compiles and immediately runs a Gecko program.

```bash
gecko run <source.gecko> [-- args...]
```

Arguments after `--` are passed to the compiled program.

### Examples

```bash
# Run a program
gecko run examples/hello.gecko

# Run with arguments
gecko run myapp.gecko -- --config settings.json

# Run with debug output
gecko run --log-level debug src/main.gecko
```

## gecko check

Type-checks source files without generating output. Useful for editor integration and CI.

```bash
gecko check <sources...>
```

Returns exit code 1 if errors are found.

### Examples

```bash
# Check a file
gecko check src/main.gecko

# Check multiple files
gecko check src/*.gecko

# Check in CI pipeline
gecko check src/main.gecko || exit 1
```

## gecko doc

Generates documentation from Gecko source files.

```bash
gecko doc [options] <file_or_directory>
```

### Options

| Flag | Description | Default |
|------|-------------|---------|
| `--output, -o` | Output directory | `./docs` |
| `--name, -n` | Project name | `Project` |
| `--format, -f` | Format: `astro`/`starlight` or `html` | `astro` |
| `--private` | Include private items | false |

### Examples

```bash
# Generate docs for stdlib
gecko doc stdlib/ -o docs/stdlib -n "Gecko Stdlib"

# Generate HTML documentation
gecko doc src/ --format html -o docs

# Include private symbols
gecko doc src/ --private
```

## gecko deps

Manages project dependencies defined in gecko.toml.

### Subcommands

```bash
gecko deps fetch   # Download all dependencies
gecko deps update  # Update dependencies to latest
```

### Examples

```bash
# Fetch all dependencies
gecko deps fetch

# Update dependencies
gecko deps update
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `GECKO_HOME` | Standard library location | Auto-detected |
| `GECKO_DEBUG` | Enable debug logging | Not set |
| `GECKO_LOG_LEVEL` | Log level (silent to trace) | `silent` |

### GECKO_HOME Search Order

1. `$GECKO_HOME/stdlib/`
2. `/usr/local/lib/gecko/`
3. `~/.gecko/`
4. `%LOCALAPPDATA%/gecko/` (Windows)
5. Current working directory

### Examples

```bash
# Set stdlib location
export GECKO_HOME=/opt/gecko

# Enable debug output
GECKO_DEBUG=1 gecko compile src/main.gecko

# Set specific log level
GECKO_LOG_LEVEL=debug gecko build src/main.gecko
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Compilation error |
| 2 | Invalid arguments |

## Tips

1. **Use `--ir-only --print-ir`** to inspect generated code
2. **Set `GECKO_LOG_LEVEL=debug`** when debugging compiler issues
3. **Use `gecko check`** in CI for fast validation
4. **Use `--keep-c`** to debug C backend output
