# Makefile for Obsidian Tag Manager

.PHONY: build test clean install run-tests lint fmt help

# Build the binary
build:
	go build -o tag-manager ./cmd/tag-manager

# Run all tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	rm -f tag-manager coverage.out coverage.html
	rm -rf test-vault/

# Install to GOPATH/bin
install:
	go install ./cmd/tag-manager

# Format code
fmt:
	go fmt ./...

# Lint code (requires golangci-lint)
lint:
	golangci-lint run

# Create test data for manual testing
test-data:
	mkdir -p test-vault/subdir
	echo -e "# Golang Tutorial\n#golang #programming #tutorial" > test-vault/golang.md
	echo -e "# Python Guide\n#python #programming #data-science" > test-vault/python.md
	echo -e "---\ntags: [\"yaml-tag\", \"frontend\"]\n---\n# Mixed Tags\nAlso has #hashtag-tag" > test-vault/mixed.md
	echo -e "# No Tags\nThis file has no tags" > test-vault/untagged.md
	echo -e "# JavaScript\n#javascript #web-development" > test-vault/subdir/js.md

# Demo CLI commands using test data
demo: test-data
	@echo "=== Listing all tags ==="
	./tag-manager list --root=test-vault --json
	@echo -e "\n=== Finding files with #programming tag ==="
	./tag-manager find --tags="programming" --root=test-vault --json
	@echo -e "\n=== Finding untagged files ==="
	./tag-manager untagged --root=test-vault --json
	@echo -e "\n=== Validating tags ==="
	./tag-manager validate --tags="valid-tag,invalid!,short" --json

# Run MCP server for testing
mcp-server:
	./tag-manager -mcp

# Help
help:
	@echo "Available targets:"
	@echo "  build         Build the binary"
	@echo "  test          Run all tests" 
	@echo "  test-coverage Run tests with coverage report"
	@echo "  clean         Clean build artifacts"
	@echo "  install       Install to GOPATH/bin"
	@echo "  fmt           Format code"
	@echo "  lint          Lint code (requires golangci-lint)"
	@echo "  test-data     Create test vault for manual testing"
	@echo "  demo          Run demo commands (requires build and test-data)"
	@echo "  mcp-server    Start MCP server mode"
	@echo "  help          Show this help message"