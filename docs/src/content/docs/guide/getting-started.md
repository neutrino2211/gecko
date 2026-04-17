---
title: Getting Started
description: Install Gecko and write your first program
sidebar:
  order: 1
---

Gecko is a compiled systems programming language with TypeScript-like ergonomics and C ABI interoperability.

## Installation

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

```ts
package main

declare external variardic func printf(format: string): int

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

```ts
import math

func main(): int {
    let result: int = math.add(1, 2)
    return result
}
```
