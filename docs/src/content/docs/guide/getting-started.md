---
title: Getting Started
description: Install Gecko and write your first program
sidebar:
  order: 1
---

Gecko is a compiled systems programming language with TypeScript-like ergonomics and C ABI interoperability.

## Installation

### Quick Install (Recommended)

**macOS / Linux:**
```bash
curl -fsSL https://raw.githubusercontent.com/neutrino2211/gecko/main/scripts/install.sh | bash
```

**Windows (PowerShell):**
```powershell
iwr -useb https://raw.githubusercontent.com/neutrino2211/gecko/main/scripts/install.ps1 | iex
```

After installation, restart your terminal and verify:
```bash
gecko --help
```

### Manual Installation

Download the latest release for your platform from [GitHub Releases](https://github.com/neutrino2211/gecko/releases).

| Platform | Architecture | Download |
|----------|--------------|----------|
| Linux | x86_64 | `gecko-linux-amd64.tar.gz` |
| Linux | ARM64 | `gecko-linux-arm64.tar.gz` |
| macOS | Intel | `gecko-darwin-amd64.tar.gz` |
| macOS | Apple Silicon | `gecko-darwin-arm64.tar.gz` |
| Windows | x86_64 | `gecko-windows-amd64.zip` |

Extract and add to your PATH:

```bash
# Linux/macOS
tar -xzf gecko-<platform>.tar.gz
mv gecko ~/.local/bin/
mv stdlib ~/.gecko/
export GECKO_HOME=~/.gecko
```

### Build from Source

```bash
# Clone the repository
git clone https://github.com/neutrino2211/gecko
cd gecko

# Install dependencies
go get

# Build the compiler
go build .

# Verify installation
./gecko --help
```

## Your First Program

Create a file `hello.gecko`:

```gecko
package main

declare external variadic func printf(format: string): int

func main(): int {
    printf("Hello, Gecko!\n")
    return 0
}
```

Compile and run:

```bash
gecko build hello.gecko -o hello
./hello
```

## Compiler Commands

| Command | Description |
|---------|-------------|
| `gecko compile <file>` | Compile to C (default backend) |
| `gecko build <file> -o <out>` | Build executable |
| `gecko run <file>` | Compile and run |
| `gecko doc <path>` | Generate documentation |

## Compiler Flags

| Flag | Description |
|------|-------------|
| `--print-ir` | Print generated IR |
| `--ir-only` | Only generate IR, don't compile |
| `--backend llvm` | Use LLVM backend |
| `--target-arch` | Target architecture (amd64, arm64) |
| `--target-platform` | Target platform (linux, darwin) |
| `--log-level debug` | Enable debug logging |

## Project Structure

```
myproject/
  main.gecko       # Entry point
  math.gecko       # Module file
```

Import modules:

```gecko
import math

func main(): int {
    let result: int = math.add(1, 2)
    return result
}
```
