version := `git describe --tags --always --dirty 2>/dev/null || echo "dev"`
commit  := `git rev-parse --short HEAD 2>/dev/null || echo "none"`
date    := `date -u +%Y-%m-%dT%H:%M:%SZ`
pkg     := "github.com/janstol/rails-kit/internal/version"
ldflags := "-X " + pkg + ".Version=" + version + " -X " + pkg + ".Commit=" + commit + " -X " + pkg + ".BuildDate=" + date

default:
    @just --list

# Build the binary
build:
    go build -ldflags "{{ldflags}}" -o rails-kit .

# Build and install to $GOBIN (or $GOPATH/bin, falling back to ~/go/bin)
install:
    #!/usr/bin/env sh
    version=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
    commit=$(git rev-parse --short HEAD 2>/dev/null || echo "none")
    date=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    pkg="github.com/janstol/rails-kit/internal/version"
    ldflags="-X $pkg.Version=$version -X $pkg.Commit=$commit -X $pkg.BuildDate=$date"
    gobin="${GOBIN:-$(go env GOBIN)}"
    [ -z "$gobin" ] && gobin="${GOPATH:-$(go env GOPATH)}/bin"
    mkdir -p "$gobin"
    go build -ldflags "$ldflags" -o "$gobin/rails-kit" .

# Run tests
test:
    go test ./...

# Run tests with verbose output
test-verbose:
    go test -v ./...

# Run tests with coverage report
test-coverage:
    go test -coverprofile=coverage.out ./...
    go tool cover -func=coverage.out

# Format code
fmt:
    go fmt ./...

# Run linter (requires golangci-lint)
lint:
    golangci-lint run ./...

# Tidy go.mod and go.sum
tidy:
    go mod tidy

# Remove built binary
clean:
    rm -f rails-kit

# Build and run with args
run *args:
    go run . {{args}}
