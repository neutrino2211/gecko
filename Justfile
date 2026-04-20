# Gecko build commands

# Local bin directory for installation
LOCAL_BIN := env_var_or_default("LOCAL_BIN", env_var("HOME") + "/.local/bin")

# Default recipe: show available commands
default:
    @just --list

# Install Go dependencies
deps:
    go get

# Build the gecko compiler
build:
    go build -o gecko .

# Run all tests
test:
    go test ./tests/... -v

# Run a specific test by name
test-one name:
    go test ./tests/... -v -run "TestCompileAndRun/{{name}}"

# Generate Starlight docs content
docs:
    go run . doc stdlib --output docs --format starlight

# Install docs dependencies
docs-install:
    cd docs && npm install

# Start docs dev server
docs-dev: docs
    cd docs && npm run dev

# Build static docs site
docs-build: docs
    cd docs && npm run build

# Generate docs and open dev server
docs-open: docs docs-install
    cd docs && npm run dev

# Compile a gecko file to C (prints IR)
compile file:
    go run . compile --print-ir --ir-only {{file}}

# Build and run a gecko file
run file:
    go run . run {{file}}

# Build a gecko file to executable
build-file file output="a.out":
    go run . build {{file}} -o {{output}}

# Clean build artifacts
clean:
    rm -rf gecko docs/api *.o *.c

# Run compiler with debug logging
debug file:
    GECKO_DEBUG=1 go run . compile {{file}}

# Format Go code
fmt:
    go fmt ./...

# Run Go linter
lint:
    go vet ./...

# Cross-compile for Linux amd64
build-linux file output="a.out":
    go run . build --target-arch=amd64 --target-platform=linux {{file}} -o {{output}}

# Cross-compile for Linux arm64
build-linux-arm file output="a.out":
    go run . build --target-arch=arm64 --target-platform=linux {{file}} -o {{output}}

# === Installation targets ===

# Build gecko compiler for current platform
build-gecko:
    go build -o gecko .

# Build gecko-lsp for current platform
build-lsp:
    go build -o gecko-lsp ./lsp

# Build both binaries for current platform
build-all: build-gecko build-lsp

# Install gecko compiler to LOCAL_BIN
install-gecko: build-gecko
    mkdir -p {{LOCAL_BIN}}
    cp gecko {{LOCAL_BIN}}/gecko
    @echo "Installed gecko to {{LOCAL_BIN}}/gecko"

# Install gecko-lsp to LOCAL_BIN
install-lsp: build-lsp
    mkdir -p {{LOCAL_BIN}}
    cp gecko-lsp {{LOCAL_BIN}}/gecko-lsp
    @echo "Installed gecko-lsp to {{LOCAL_BIN}}/gecko-lsp"

# Install both binaries to LOCAL_BIN
install: install-gecko install-lsp
    @echo "Installation complete. Ensure {{LOCAL_BIN}} is in your PATH."

# === Cross-platform builds ===

# Build gecko for macOS ARM64 (Apple Silicon)
build-gecko-darwin-arm64:
    GOOS=darwin GOARCH=arm64 go build -o gecko-darwin-arm64 .

# Build gecko for macOS AMD64 (Intel)
build-gecko-darwin-amd64:
    GOOS=darwin GOARCH=amd64 go build -o gecko-darwin-amd64 .

# Build gecko for Linux AMD64
build-gecko-linux-amd64:
    GOOS=linux GOARCH=amd64 go build -o gecko-linux-amd64 .

# Build gecko for Linux ARM64
build-gecko-linux-arm64:
    GOOS=linux GOARCH=arm64 go build -o gecko-linux-arm64 .

# Build gecko for Windows AMD64
build-gecko-windows-amd64:
    GOOS=windows GOARCH=amd64 go build -o gecko-windows-amd64.exe .

# Build gecko-lsp for macOS ARM64 (Apple Silicon)
build-lsp-darwin-arm64:
    GOOS=darwin GOARCH=arm64 go build -o gecko-lsp-darwin-arm64 ./lsp

# Build gecko-lsp for macOS AMD64 (Intel)
build-lsp-darwin-amd64:
    GOOS=darwin GOARCH=amd64 go build -o gecko-lsp-darwin-amd64 ./lsp

# Build gecko-lsp for Linux AMD64
build-lsp-linux-amd64:
    GOOS=linux GOARCH=amd64 go build -o gecko-lsp-linux-amd64 ./lsp

# Build gecko-lsp for Linux ARM64
build-lsp-linux-arm64:
    GOOS=linux GOARCH=arm64 go build -o gecko-lsp-linux-arm64 ./lsp

# Build gecko-lsp for Windows AMD64
build-lsp-windows-amd64:
    GOOS=windows GOARCH=amd64 go build -o gecko-lsp-windows-amd64.exe ./lsp

# Build all binaries for all platforms
build-all-platforms: build-gecko-darwin-arm64 build-gecko-darwin-amd64 build-gecko-linux-amd64 build-gecko-linux-arm64 build-gecko-windows-amd64 build-lsp-darwin-arm64 build-lsp-darwin-amd64 build-lsp-linux-amd64 build-lsp-linux-arm64 build-lsp-windows-amd64
    @echo "Built all platform binaries"

# Clean all built binaries
clean-binaries:
    rm -f gecko gecko-lsp
    rm -f gecko-darwin-* gecko-linux-* gecko-windows-*
    rm -f gecko-lsp-darwin-* gecko-lsp-linux-* gecko-lsp-windows-*
