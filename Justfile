# Gecko build commands

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
