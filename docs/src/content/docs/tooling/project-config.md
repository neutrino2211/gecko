---
title: Project Configuration
description: Configuring Gecko projects with gecko.toml
sidebar:
  order: 2
---

Gecko projects use a `gecko.toml` file for configuration. This file defines package metadata, build settings, dependencies, and target-specific options.

## Basic Structure

```toml
[package]
name = "myproject"
version = "0.1.0"

[build]
backend = "c"

[build.entries]
main = "src/main.gecko"
```

## Package Section

Define package metadata:

```toml
[package]
name = "myproject"          # Package name
version = "1.0.0"           # Semantic version
description = "My project"  # Optional description
authors = ["Name <email>"]  # Optional authors
license = "MIT"             # Optional license
```

## Build Section

Configure build settings:

```toml
[build]
backend = "c"                           # "c" or "llvm"
default_target = "x86_64-apple-darwin"  # Default target triple
```

### Entry Points

Define named entry points for building:

```toml
[build.entries]
main = "src/main.gecko"
cli = "src/cli.gecko"
server = "src/server/main.gecko"
```

Use with:
```bash
gecko build --entry main
gecko build --entry cli
```

### Build Profiles

Configure optimization and debug settings:

```toml
[build.profiles.debug]
optimize = false

[build.profiles.release]
optimize = true    # or "size" for size optimization
```

## Dependencies

### Git Dependencies

```toml
[dependencies]
# Latest from main branch
mylib = { git = "https://github.com/user/mylib" }

# Specific tag
mylib = { git = "https://github.com/user/mylib", tag = "v1.0.0" }

# Specific branch
mylib = { git = "https://github.com/user/mylib", branch = "develop" }

# Specific commit
mylib = { git = "https://github.com/user/mylib", commit = "abc123" }
```

### Local Dependencies

```toml
[dependencies]
locallib = { path = "./libs/locallib" }
shared = { path = "../shared" }
```

### Using Dependencies

After defining dependencies, fetch them:

```bash
gecko deps fetch
```

Then import in your code:

```gecko
import mylib
import locallib.utils
```

## Target Configuration

Configure settings for specific targets:

```toml
[target.x86_64-unknown-linux-gnu]
# Linux-specific settings

[target.aarch64-apple-darwin]
# macOS ARM settings

[target.i386-none-elf]
freestanding = true
linker_script = "linker.ld"
```

### Freestanding Targets

For bare-metal or kernel development:

```toml
[target.i386-none-elf]
freestanding = true
linker_script = "kernel/linker.ld"

[target.aarch64-none-elf]
freestanding = true
linker_script = "boot/link.ld"
```

## Complete Example

```toml
[package]
name = "mywebserver"
version = "0.2.0"
description = "A simple web server in Gecko"
authors = ["Developer <dev@example.com>"]
license = "MIT"

[build]
backend = "c"
default_target = "x86_64-unknown-linux-gnu"

[build.entries]
server = "src/main.gecko"
cli = "src/cli.gecko"

[build.profiles.debug]
optimize = false

[build.profiles.release]
optimize = true

[dependencies]
http = { git = "https://github.com/gecko-lang/http", tag = "v0.1.0" }
json = { git = "https://github.com/gecko-lang/json", branch = "main" }
utils = { path = "./libs/utils" }

[target.x86_64-unknown-linux-gnu]
# Default Linux target

[target.aarch64-apple-darwin]
# macOS ARM build
```

## Project Structure

Recommended project layout:

```
myproject/
├── gecko.toml          # Project configuration
├── src/
│   ├── main.gecko      # Main entry point
│   ├── lib.gecko       # Library code
│   └── utils/
│       └── helpers.gecko
├── libs/               # Local dependencies
│   └── mylib/
│       └── mod.gecko
├── tests/
│   └── test_main.gecko
└── examples/
    └── demo.gecko
```

## Tips

1. **Use entry points** for multiple binaries from one project
2. **Pin dependency versions** with tags for reproducible builds
3. **Use profiles** to separate debug and release settings
4. **Keep local deps in `libs/`** for organization
5. **Use freestanding** for embedded/kernel targets
