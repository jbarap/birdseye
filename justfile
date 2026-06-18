binary := "be"
pkg := "./cmd/be"
version := `git describe --tags --always --dirty 2>/dev/null || echo dev`
ldflags := "-s -w -X main.version=" + version

# List available recipes.
default:
    @just --list

# Build a static be binary into ./bin.
build:
    CGO_ENABLED=0 go build -trimpath -ldflags "{{ldflags}}" -o bin/{{binary}} {{pkg}}

# Install be into GOBIN.
install:
    CGO_ENABLED=0 go install -trimpath -ldflags "{{ldflags}}" {{pkg}}

# Remove the installed be binary from GOBIN (errors if not installed).
uninstall:
    #!/usr/bin/env sh
    set -eu
    gobin="$(go env GOBIN)"
    [ -n "$gobin" ] || gobin="$(go env GOPATH)/bin"
    rm "$gobin/{{binary}}"

# Run all tests.
test:
    go test ./...

# Run go vet.
vet:
    go vet ./...

# Vet then test.
check: vet test

# Tidy module dependencies.
tidy:
    go mod tidy

# Remove build artifacts.
clean:
    rm -rf bin
