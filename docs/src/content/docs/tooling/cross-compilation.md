---
title: Cross-Compilation
description: Building Gecko programs for different platforms
sidebar:
  order: 3
---

Gecko supports cross-compilation to different architectures and operating systems.

## Target Specification

Specify targets using the `--target-arch` and `--target-platform` flags:

```bash
gecko build --target-arch=<arch> --target-platform=<platform> source.gecko
```

### Supported Architectures

| Architecture | Flag Value | Description |
|--------------|------------|-------------|
| x86-64 | `amd64` | 64-bit x86 |
| x86 | `i386` | 32-bit x86 |
| ARM64 | `arm64` | 64-bit ARM (Apple Silicon, etc.) |
| ARM | `arm` | 32-bit ARM |

### Supported Platforms

| Platform | Flag Value | Description |
|----------|------------|-------------|
| Linux | `linux` | Linux systems |
| macOS | `darwin` | macOS / Apple systems |
| Windows | `windows` | Windows systems |
| FreeBSD | `freebsd` | FreeBSD systems |
| None | `none` | Freestanding (no OS) |

## Common Cross-Compilation Scenarios

### Linux on macOS

```bash
# Build for Linux x86-64 from macOS
gecko build --target-arch=amd64 --target-platform=linux src/main.gecko -o myapp-linux
```

### macOS ARM on x86

```bash
# Build for Apple Silicon from Intel Mac
gecko build --target-arch=arm64 --target-platform=darwin src/main.gecko -o myapp-arm64
```

### Windows from Linux/macOS

```bash
# Build for Windows
gecko build --target-arch=amd64 --target-platform=windows src/main.gecko -o myapp.exe
```

## Freestanding Builds

For bare-metal or kernel development without an OS:

```bash
gecko build --target-arch=i386 --target-platform=none --target-vendor=none-elf src/kernel.gecko
```

### Freestanding Configuration

Configure in gecko.toml:

```toml
[target.i386-none-elf]
freestanding = true
linker_script = "kernel/linker.ld"
```

### Linker Scripts

For freestanding targets, provide a linker script:

```ld
/* linker.ld */
ENTRY(_start)

SECTIONS {
    . = 0x100000;
    
    .text : {
        *(.text)
    }
    
    .rodata : {
        *(.rodata)
    }
    
    .data : {
        *(.data)
    }
    
    .bss : {
        *(.bss)
    }
}
```

## Cross-Compilation Requirements

### C Toolchain

The C backend requires a cross-compilation toolchain:

| Target | Required Toolchain |
|--------|-------------------|
| Linux | `gcc` or cross-gcc |
| macOS | Xcode command line tools |
| Windows | MinGW-w64 |
| Freestanding | Cross GCC (e.g., `i386-elf-gcc`) |

### Installing Cross Compilers

**macOS (Homebrew):**
```bash
# For Linux targets
brew install x86_64-elf-gcc

# For bare-metal
brew install i386-elf-gcc
```

**Linux (apt):**
```bash
# For Windows targets
sudo apt install mingw-w64

# For ARM targets
sudo apt install gcc-aarch64-linux-gnu
```

## Build Matrix Example

Build for multiple platforms:

```bash
#!/bin/bash
# build-all.sh

# Native build
gecko build src/main.gecko -o dist/myapp

# Linux x86-64
gecko build --target-arch=amd64 --target-platform=linux \
    src/main.gecko -o dist/myapp-linux-amd64

# Linux ARM64
gecko build --target-arch=arm64 --target-platform=linux \
    src/main.gecko -o dist/myapp-linux-arm64

# macOS ARM64
gecko build --target-arch=arm64 --target-platform=darwin \
    src/main.gecko -o dist/myapp-darwin-arm64

# Windows x86-64
gecko build --target-arch=amd64 --target-platform=windows \
    src/main.gecko -o dist/myapp-windows-amd64.exe
```

## Conditional Compilation

Use architecture-specific impl blocks for platform-specific code:

```gecko
// Platform detection at compile time
impl amd64 {
    func get_page_size(): uint64 {
        return 4096
    }
}

impl arm64 {
    func get_page_size(): uint64 {
        return 16384  // Apple Silicon uses 16KB pages
    }
}
```

## Troubleshooting

### Missing Cross Compiler

```
Error: cannot find 'x86_64-linux-gnu-gcc'
```

Install the appropriate cross-compilation toolchain for your target.

### Linker Script Not Found

```
Error: cannot open linker script 'linker.ld'
```

Ensure the linker script path in gecko.toml is correct relative to the project root.

### Incompatible Libraries

When cross-compiling with C libraries, ensure you have the target-architecture versions of those libraries installed.

## Tips

1. **Test on target platform** when possible
2. **Use CI/CD** for automated cross-platform builds
3. **Keep platform-specific code minimal** using abstractions
4. **Document target requirements** for your project
