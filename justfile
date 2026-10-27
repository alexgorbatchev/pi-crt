set dotenv-load := false

# Default recipe: list available recipes
default:
    @just --list

# Run CLI in human mode (default)
run *args:
    go run ./cmd/pi-crt {{ args }}

# Run CLI in agent-facing mode
run-ai *args:
    AGENT=1 go run ./cmd/pi-crt {{ args }}

# Run test suite
test:
    mkdir -p .tmp/go
    GOTMPDIR="{{ justfile_directory() }}/.tmp/go" go test -race ./...

# Build binary into bin/
build:
    mkdir -p bin .tmp/go
    GOTMPDIR="{{ justfile_directory() }}/.tmp/go" go build -trimpath -o bin/pi-crt ./cmd/pi-crt

# Cross-compile for the Pi 1 CPU, without a C runtime dependency
build-pi:
    mkdir -p bin .tmp/go
    GOTMPDIR="{{ justfile_directory() }}/.tmp/go" CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=6 go build -trimpath -ldflags='-s -w' -o bin/pi-crt-linux-armv6 ./cmd/pi-crt

# Real FFmpeg tests; PI_CRT_VLC optionally enables real headless VLC tests
integration:
    mkdir -p .tmp/go
    GOTMPDIR="{{ justfile_directory() }}/.tmp/go" PI_CRT_INTEGRATION=1 go test -race -coverprofile=.tmp/coverage.out ./...

# Build a local install bundle, without publishing
package: build-pi
    tar -czf bin/pi-crt-pi1.tar.gz deploy scripts docs examples README.md AGENTS.md LICENSE -C bin pi-crt-linux-armv6
    shasum -a 256 bin/pi-crt-pi1.tar.gz > bin/pi-crt-pi1.tar.gz.sha256

# Install binary to GOPATH bin directory
install:
    go install ./cmd/pi-crt

# Run static analysis and vet
lint:
    go vet ./...
    golangci-lint run ./...

# Alias for lint
vet: lint

# Format Go code
fmt:
    go fmt ./...

# Run static analysis and test suite in sequence
check:
    go mod tidy -diff
    go vet ./...
    just test
    golangci-lint run ./...
    for script in scripts/*.sh; do sh -n "$script" || exit; done
    just --fmt --check

# Clean up build binaries and temporary files
clean:
    rm -rf bin coverage.out .tmp dist
