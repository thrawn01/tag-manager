# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

### Build and Run
```bash
# Build the binary
go build -o tag-manager ./cmd/tag-manager

# Install to GOPATH/bin
go install ./cmd/tag-manager

# Run tests
go test -v ./...

# Run tests with race detection (matches CI)
go test -race -v ./...

# Run specific test
go test -v -run TestMCPServerCapabilities

# Run linting
golangci-lint run
```

### Testing the MCP Server
```bash
# Test MCP server integration 
go test -v -run TestMCPServerCapabilities

# Manual MCP server testing (runs stdio server)
go run ./cmd/tag-manager -mcp

# Test CLI functionality 
go test -v -run TestCLIIntegration
```

### Testing with Sample Data
```bash
# Create temporary test files for development
mkdir -p /tmp/test-vault
echo -e "---\ntags: [golang, programming]\n---\n# Test Note\n\nThis is a #test note." > /tmp/test-vault/test.md

# Test commands
./tag-manager list --root=/tmp/test-vault
./tag-manager find --tags=golang --root=/tmp/test-vault
```

## Architecture Overview

This is a dual-purpose tool: a CLI application and an MCP (Model Context Protocol) server for Claude Code integration. The architecture follows a clean separation of concerns with well-defined interfaces.

### Core Architecture Layers

**1. CLI Layer (`cli.go`)**
- Entry point via `RunCmd(args)` and `RunCmdWithOptions(args, options)`
- Flag parsing and command routing
- The `RunCmdOptions` struct allows injecting custom MCP transports for testing
- Delegates to either MCP server (`RunMCPServer`) or TagManager operations

**2. MCP Server (`mcp.go`)**
- Implements Model Context Protocol using `github.com/modelcontextprotocol/go-sdk`
- Exposes `list_all_tags` tool for Claude Code integration
- Uses `ListAllTagsTool` function as the tool handler
- Supports both stdio (production) and in-memory (testing) transports

**3. Core Business Logic (`manager.go`)**
- `TagManager` interface defines all tag operations
- `DefaultTagManager` implements the interface using Scanner and Validator
- Main operations: FindFilesByTags, ListAllTags, ReplaceTagsBatch, GetUntaggedFiles, etc.
- Uses Go 1.23+ iterators for memory-efficient file processing

**4. File System Layer (`scanner.go`)**
- `Scanner` interface for file system operations
- `FilesystemScanner` implementation handles directory traversal and tag extraction
- Supports hashtags (`#tag`) and YAML frontmatter (array and list formats)
- Smart filtering of false positives (hex colors, GitHub issues, URLs, etc.)

**5. Validation Layer (`validator.go`)**
- `Validator` interface for tag and path validation
- `DefaultValidator` implements validation rules
- Configurable rules via `Config` struct (min length, digit ratio, exclude patterns)

### Key Design Patterns

**Interface-Based Design**: All major components are interfaces (`TagManager`, `Scanner`, `Validator`) making the code testable and extensible.

**Dependency Injection**: Components are injected rather than created directly, enabling easy testing with mocks.

**Iterator Pattern**: Uses Go 1.23 iterators for memory-efficient processing of large file sets without loading everything into memory.

**Non-Atomic Batch Operations**: Tag replacement operations continue on errors, reporting successes and failures separately. This prevents one bad file from stopping an entire batch operation.

**Transport Abstraction**: MCP server supports multiple transports (stdio for production, in-memory for testing) via the MCP SDK's transport interface.

## Testing Architecture

### Test Structure
- **Unit Tests**: Each major component has dedicated test files (`*_test.go`)
- **Integration Tests**: `cli_test.go` contains full CLI integration tests
- **MCP Testing**: Uses `mcp.NewInMemoryTransports()` for testing client-server communication
- **External Test Package**: Tests use `package tagmanager_test` to test public API only

### MCP Testing Pattern
The MCP server testing uses a sophisticated pattern:
```go
// Create in-memory transports for testing
clientTransport, serverTransport := mcp.NewInMemoryTransports()

// Start MCP server using actual CLI with custom transport
options := &tagmanager.RunCmdOptions{MCPTransport: serverTransport}
go func() {
    tagmanager.RunCmdWithOptions([]string{"tag-manager", "-mcp"}, options)
}()

// Create client and test tool discovery
client := mcp.NewClient(...)
session, err := client.Connect(ctx, clientTransport, nil)
tools, err := session.ListTools(ctx, &mcp.ListToolsParams{})
```

This tests the full integration from CLI flags → MCP server → tool registration → client discovery.

## Configuration System

The `Config` struct in `config.go` controls all behavior:
- **Tag Extraction**: Regex patterns for hashtags and YAML frontmatter  
- **Filtering Rules**: Minimum length, maximum digit ratio, excluded keywords
- **File System**: Excluded directories and file patterns
- **Validation**: Path security (prevents directory traversal)

Default configuration is optimized for Obsidian vaults but can be overridden via YAML files.

## Data Types and JSON API

All operations support JSON output for programmatic use. Key types in `types.go`:
- `TagInfo`: Tag name, count, and associated files
- `FileTagInfo`: File path and its tags
- `TagReplaceResult`: Results of batch operations including successes and failures
- `TagReplacement`: Old/new tag pairs for batch replacements

The JSON API maintains backward compatibility and provides structured error reporting.

## Memory Efficiency

The codebase is designed for large Obsidian vaults (1000+ files):
- **Streaming Processing**: Uses iterators instead of loading all files into memory
- **Lazy Evaluation**: Files are processed on-demand during iteration
- **Constant Memory**: Memory usage stays constant regardless of vault size
- **Early Filtering**: Invalid files/tags are filtered early in the pipeline

## MCP Integration Notes

When working with MCP functionality:
- The MCP server runs in stdio mode by default (for Claude Code)
- Tool handlers must match the exact signature expected by the MCP SDK
- Use `mcp.NewInMemoryTransports()` for testing client-server communication
- The `ListAllTagsTool` function signature requires a `TagManager` parameter that must be injected
- Always test both the CLI and MCP modes when making changes to core functionality