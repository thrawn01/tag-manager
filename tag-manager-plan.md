# Obsidian Tag Manager MCP Server - Design Document

## Overview

Transform the existing `tag-collector.go` into a hybrid CLI/MCP tool that provides Claude Code with precise tag management capabilities for Obsidian repositories, eliminating error-prone file system operations.

## Requirements Summary

- **Primary Goal**: Create an MCP server that Claude Code can use to efficiently manage Obsidian tags
- **Secondary Goal**: Provide CLI interface for direct user interaction
- **Architecture**: Single binary with both CLI and MCP server modes
- **Integration**: No caching/indexing - scan files fresh each time
- **File Support**: Only `.md` files, exclude `.excalidraw.md` files
- **Tag Operations**: Support hashtag and YAML frontmatter formats
- **Replacement Strategy**: Non-atomic batch operations with individual error reporting
- **Installation**: `go install github.com/thrawn01/tag-manager`
- **Configuration**: YAML format with slog structured logging
- **Dependencies**: Official MCP Go SDK (`github.com/modelcontextprotocol/go-sdk`) - direct usage without abstraction layer

## Design Decisions & Constraints

### Supported Features
- Memory-efficient streaming file processing using `iter.Seq2`
- Individual operation error reporting within batch operations
- Context cancellation for long-running operations
- Structured logging with slog for debugging

### Explicitly Unsupported Features
- **Concurrent file modifications**: No protection against files being modified during operations
- **Atomic batch operations**: Failed operations are skipped, successful ones continue
- **Performance metrics**: No runtime performance monitoring or metrics collection
- **MCP SDK abstraction**: Direct dependency on official Go SDK without fallback layers
- **File backup/restoration**: Relies on Obsidian sync and git for recovery

## Architecture

### Core Components

```go
// types.go
type TagInfo struct {
    Name  string `json:"name"`
    Count int    `json:"count"`
    Files []string `json:"files"`
}

type FileTagInfo struct {
    Path string   `json:"path"`
    Tags []string `json:"tags"`
}

type TagReplaceResult struct {
    ModifiedFiles []string `json:"modified_files"`
    FailedFiles   []string `json:"failed_files,omitempty"`
    Errors        []string `json:"errors,omitempty"`
}

type ScanStats struct {
    TotalFiles    int
    ProcessedFiles int
    ErrorCount    int
    LastError     error
}

type TagReplacement struct {
    OldTag string `json:"old_tag"`
    NewTag string `json:"new_tag"`
}
```

```go
// config.go
type Config struct {
    ExcludeDirs     []string `yaml:"exclude_dirs"`
    ExcludePatterns []string `yaml:"exclude_patterns"`
    HashtagPattern  string   `yaml:"hashtag_pattern"`
    YAMLTagPattern  string   `yaml:"yaml_tag_pattern"`
    YAMLListPattern string   `yaml:"yaml_list_pattern"`
    MinTagLength    int      `yaml:"min_tag_length"`
    MaxDigitRatio   float64  `yaml:"max_digit_ratio"`
    ExcludeKeywords []string `yaml:"exclude_keywords"`
}

func DefaultConfig() *Config {
    return &Config{
        HashtagPattern:  `#[a-zA-Z][\w\-]*`,
        YAMLTagPattern:  `(?m)^tags:\s*\[([^\]]+)\]`,
        YAMLListPattern: `(?m)^tags:\s*$\n((?:\s+-\s+.+\n?)+)`,
        ExcludeKeywords: []string{"bibr", "ftn", "issuecomment", "discussion", "diff-"},
        ExcludeDirs:     []string{"100 Archive", "Attachments", ".git"},
        ExcludePatterns: []string{"*.excalidraw.md"},
        MaxDigitRatio:   0.5,
        MinTagLength:    3,
    }
}
```


```go
// scanner.go
import "iter"

type Scanner interface {
    ScanDirectory(ctx context.Context, rootPath string, excludePaths []string) iter.Seq2[FileTagInfo, error]
    ScanFile(ctx context.Context, filePath string) (FileTagInfo, error)
    ExtractTags(content string) []string
    ExtractTagsFromReader(ctx context.Context, reader io.Reader) []string
}

type FilesystemScanner struct {
    config            *Config
    hashtagPattern    *regexp.Regexp
    yamlTagPattern    *regexp.Regexp
    yamlTagListPattern *regexp.Regexp
}

type ValidationResult struct {
    IsValid   bool     `json:"is_valid"`
    Issues    []string `json:"issues,omitempty"`
    Suggestions []string `json:"suggestions,omitempty"`
}

type Validator interface {
    ValidateTag(tag string) *ValidationResult
    ValidatePath(path string) error
    ValidateConfig(config *Config) error
}
```

```go
// manager.go
type TagManager interface {
    FindFilesByTags(ctx context.Context, tags []string, rootPath string) (map[string][]string, error)
    GetTagsInfo(ctx context.Context, tags []string, rootPath string) ([]TagInfo, error)
    ListAllTags(ctx context.Context, rootPath string, minCount int) ([]TagInfo, error)
    ReplaceTagsBatch(ctx context.Context, replacements []TagReplacement, rootPath string, dryRun bool) (*TagReplaceResult, error)
    GetUntaggedFiles(ctx context.Context, rootPath string) ([]FileTagInfo, error)
    GetFilesTags(ctx context.Context, filePaths []string) ([]FileTagInfo, error)
    ValidateTags(ctx context.Context, tags []string) map[string]*ValidationResult
}

type DefaultTagManager struct {
    scanner   Scanner
    validator Validator
    config    *Config
}

// Memory-Efficient Iterator Pattern
// The scanner uses iter.Seq2 for streaming file processing:
// 1. Lazy evaluation - files processed one at a time
// 2. Early termination via yield function return value
// 3. No intermediate slice allocations for large directories
// 4. Context cancellation support for long operations
// 5. Error handling without stopping entire operation
```

## MCP Tool Response Formats

```go
// Response structures for all MCP tools
type FindFilesResponse struct {
    Results     map[string][]string `json:"results"`  // tag -> files mapping
    TotalFiles  int                 `json:"total_files"`
    TruncatedAt *int                `json:"truncated_at,omitempty"`
}

type TagInfoResponse struct {
    Tags        []TagInfo `json:"tags"`
    TruncatedAt *int      `json:"truncated_at,omitempty"`
}

type ListTagsResponse struct {
    Tags        []TagInfo `json:"tags"`
    TotalCount  int       `json:"total_count"`
    TruncatedAt *int      `json:"truncated_at,omitempty"`
}

type ReplaceTagResponse struct {
    Replacements  []TagReplacement `json:"replacements"`
    ModifiedFiles []string         `json:"modified_files"`
    FailedFiles   []string         `json:"failed_files,omitempty"`
    Errors        []string         `json:"errors,omitempty"`
    DryRun        bool             `json:"dry_run"`
}

type UntaggedFilesResponse struct {
    Files       []FileTagInfo `json:"files"`
    Count       int           `json:"count"`
    TruncatedAt *int          `json:"truncated_at,omitempty"`
}

type ValidateTagsResponse struct {
    Results map[string]ValidationResult `json:"results"`
}

type FileTagsResponse struct {
    Files       []FileTagInfo `json:"files"`
    TruncatedAt *int          `json:"truncated_at,omitempty"`
}
```

## Main Entry Point Structure (Following Flake Template)

```go
// main.go
func main() {
    if err := RunCmd(os.Args); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}

func RunCmd(args []string) error {
    var (
        help      = flag.Bool("h", false, "Show help")
        mcp       = flag.Bool("mcp", false, "Run as MCP server")
        verbose   = flag.Bool("v", false, "Verbose output")
        dryRun    = flag.Bool("dry-run", false, "Show what would be changed without making changes")
        configFile = flag.String("config", "", "Path to configuration file")
    )
    
    // Parse flags from args[1:] to handle test scenarios
    fs := flag.NewFlagSet(args[0], flag.ExitOnError)
    fs.BoolVar(help, "h", false, "Show help")
    fs.BoolVar(mcp, "mcp", false, "Run as MCP server")
    fs.BoolVar(verbose, "v", false, "Verbose output")
    fs.BoolVar(dryRun, "dry-run", false, "Show what would be changed without making changes")
    fs.StringVar(configFile, "config", "", "Path to configuration file")
    
    if len(args) > 1 {
        fs.Parse(args[1:])
    }
    
    if *help {
        return showHelp()
    }
    
    if *mcp {
        return runMCPServer(*configFile)
    }
    
    // Handle CLI commands
    remaining := fs.Args()
    if len(remaining) == 0 {
        return showHelp()
    }
    
    // Load configuration
    config, err := loadConfig(*configFile)
    if err != nil {
        return fmt.Errorf("failed to load config: %w", err)
    }
    
    ctx := context.Background()
    manager := NewDefaultTagManager(config)
    
    switch remaining[0] {
    case "find":
        return findFilesCommand(ctx, manager, remaining[1:], *verbose)
    case "info":
        return getTagInfoCommand(ctx, manager, remaining[1:], *verbose)
    case "list":
        return listTagsCommand(ctx, manager, remaining[1:], *verbose)
    case "replace":
        return replaceTagCommand(ctx, manager, remaining[1:], *dryRun, *verbose)
    case "untagged":
        return untaggedFilesCommand(ctx, manager, remaining[1:], *verbose)
    case "validate":
        return validateTagsCommand(ctx, manager, remaining[1:], *verbose)
    case "file-tags":
        return getFileTagsCommand(ctx, manager, remaining[1:], *verbose)
    default:
        return fmt.Errorf("unknown command: %s", remaining[0])
    }
}
```

## CLI Usage Examples

```bash
# Direct CLI usage with subcommands
obsidian-tag-manager find --tag="#golang" --root="/path/to/notes"
obsidian-tag-manager list --root="/path/to/notes" --min-count=2 --pattern="^#dev"
obsidian-tag-manager replace --old="#old" --new="#new" --root="/path/to/notes" --dry-run
obsidian-tag-manager untagged --root="/path/to/notes"
obsidian-tag-manager validate --tags="#test,#invalid-tag!"
obsidian-tag-manager file-tags --file="/path/to/notes/example.md"

# Global options
obsidian-tag-manager -v list --root="/path/to/notes"  # Verbose output
obsidian-tag-manager --config="/path/to/config.yaml" find --tag="#golang"

# MCP server mode for Claude Code
obsidian-tag-manager -mcp
obsidian-tag-manager -mcp --config="/path/to/config.json"
```

## MCP Tools Implementation

### 1. find_files_by_tags
```json
{
  "name": "find_files_by_tags",
  "description": "Find files containing specific tags",
  "inputSchema": {
    "type": "object",
    "properties": {
      "tags": {"type": "array", "items": {"type": "string"}, "description": "Tags to search for (with or without #)"},
      "root_path": {"type": "string", "description": "Root directory to search"},
      "exclude_paths": {"type": "array", "items": {"type": "string"}, "description": "Additional paths to exclude"},
      "max_results": {"type": "integer", "minimum": 1, "maximum": 1000, "default": 100, "description": "Maximum files per tag"}
    },
    "required": ["tags", "root_path"]
  }
}
```

### 2. get_tags_info
```json
{
  "name": "get_tags_info", 
  "description": "Get detailed information about multiple tags",
  "inputSchema": {
    "type": "object",
    "properties": {
      "tags": {"type": "array", "items": {"type": "string"}, "description": "Tags to get info for"},
      "root_path": {"type": "string", "description": "Root directory to search"},
      "max_files_per_tag": {"type": "integer", "minimum": 1, "maximum": 1000, "default": 100, "description": "Maximum files to return per tag"}
    },
    "required": ["tags", "root_path"]
  }
}
```

### 3. list_all_tags
```json
{
  "name": "list_all_tags",
  "description": "List all tags with usage statistics", 
  "inputSchema": {
    "type": "object",
    "properties": {
      "root_path": {"type": "string", "description": "Root directory to search"},
      "min_count": {"type": "integer", "minimum": 1, "default": 1, "description": "Minimum usage count"},
      "pattern": {"type": "string", "description": "Optional regex pattern to filter tags"}
    },
    "required": ["root_path"]
  }
}
```

### 4. replace_tags_batch
```json
{
  "name": "replace_tags_batch",
  "description": "Replace/rename multiple tags across files in batch operations",
  "inputSchema": {
    "type": "object", 
    "properties": {
      "replacements": {
        "type": "array",
        "items": {
          "type": "object",
          "properties": {
            "old_tag": {"type": "string", "description": "Tag to replace"},
            "new_tag": {"type": "string", "description": "New tag name"}
          },
          "required": ["old_tag", "new_tag"]
        },
        "description": "Array of tag replacements to perform"
      },
      "root_path": {"type": "string", "description": "Root directory to search"},
      "exclude_paths": {"type": "array", "items": {"type": "string"}, "description": "Paths to exclude from replacement"},
      "dry_run": {"type": "boolean", "default": false, "description": "Show what would be changed without making changes"}
    },
    "required": ["replacements", "root_path"]
  }
}
```

### 5. get_untagged_files
```json
{
  "name": "get_untagged_files",
  "description": "Find files without any tags",
  "inputSchema": {
    "type": "object",
    "properties": {
      "root_path": {"type": "string", "description": "Root directory to search"}
    },
    "required": ["root_path"]
  }
}
```

### 6. validate_tags
```json
{
  "name": "validate_tags",
  "description": "Validate tag syntax and suggest fixes",
  "inputSchema": {
    "type": "object",
    "properties": {
      "tags": {"type": "array", "items": {"type": "string"}, "description": "Tags to validate"},
      "include_suggestions": {"type": "boolean", "default": true, "description": "Include fix suggestions"}
    },
    "required": ["tags"]
  }
}
```

### 7. get_files_tags
```json
{
  "name": "get_files_tags",
  "description": "Get all tags for multiple files",
  "inputSchema": {
    "type": "object",
    "properties": {
      "file_paths": {"type": "array", "items": {"type": "string"}, "description": "Paths to the files"},
      "include_content_preview": {"type": "boolean", "default": false, "description": "Include content preview around tags"},
      "max_files": {"type": "integer", "minimum": 1, "maximum": 1000, "default": 100, "description": "Maximum number of files to process"}
    },
    "required": ["file_paths"]
  }
}
```

### Enhanced Parameters for Existing Tools

**list_all_tags** additional parameters:
```json
{
  "sort_by": {"enum": ["count", "name", "recent"], "default": "count"},
  "max_results": {"type": "integer", "minimum": 1, "maximum": 10000, "default": 1000}
}
```

## Implementation Phases

### Phase 1: Core Tag Processing Engine
**Milestone**: Refactor existing tag extraction and filtering logic into reusable components

**Deliverables**:
- `types.go` - Shared data structures
- `config.go` - Configuration management with all existing patterns
- `scanner.go` - File scanning and tag extraction with streaming support
- `validator.go` - Tag validation and syntax checking
- `manager.go` - Tag management operations with context support and direct file operations

**Critical Requirements Preserved**:
- **Complex Tag Filtering**: All existing logic (hex colors, IDs, URL fragments, digit ratios)
- **Boundary Validation**: Sophisticated hashtag boundary checking from existing code
- **Tag Cleaning**: All normalization and filtering rules preserved
- **Exclusion Patterns**: All hardcoded exclusions (bibr, ftn, issuecomment, etc.)

**Acceptance Criteria**:
- All existing tag extraction patterns work (hashtags, YAML arrays, YAML lists)
- Complex filtering logic fully preserved with tests
- File exclusions work (excalidraw, archive, attachments directories)  
- Configurable root directory and patterns support
- Non-atomic batch operations: failed operations skip individual files, successful operations continue
- Tag replacement using Go's standard `os.WriteFile` with individual error reporting
- Context cancellation support for all operations
- Streaming support for large repositories
- Memory usage remains reasonable for 1000+ files

**End-to-End Test**:
```go
package tagmanager_test

func TestTagManagerE2E(t *testing.T) {
    config := DefaultConfig()
    scanner := NewFilesystemScanner(config)
    manager := NewDefaultTagManager(config)
    ctx := context.Background()
    
    // Test complex filtering preservation using iterator
    var allFiles []FileTagInfo
    for fileInfo, err := range scanner.ScanDirectory(ctx, "/test/obsidian", nil) {
        if err != nil {
            require.NoError(t, err)
            break
        }
        allFiles = append(allFiles, fileInfo)
        
        // Verify hex colors filtered out  
        for _, tag := range fileInfo.Tags {
            assert.False(t, isHexColor(tag))
        }
    }
    assert.NotEmpty(t, allFiles)
    
    // Test non-atomic batch replacement 
    replaceResult, err := manager.ReplaceTagsBatch(ctx, []TagReplacement{{OldTag: "#old", NewTag: "#new"}}, "/test/obsidian", false)
    require.NoError(t, err)
    // Some files may fail individually while others succeed
    assert.NotNil(t, replaceResult)
    
    // Test context cancellation
    ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
    defer cancel()
    _, err = manager.ListAllTags(ctx, "/large/repo", 1)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "context")
}
```

### Phase 2: CLI Implementation  
**Milestone**: Working CLI with all seven core commands plus global options

**Deliverables**:
- `main.go` - CLI entry point with RunCmd function and command handlers
- Global flag support (verbose, dry-run, config)
- Comprehensive help system and usage documentation  
- Input validation and user-friendly error messages

**CLI Commands**:
1. `find --tags="#golang,#python" --root="/path"` - find files with multiple tags
2. `info --tags="#golang,#python" --root="/path"` - detailed information for multiple tags
3. `list --root="/path" --min-count=2 --pattern="^#dev"` - with sorting and limits
4. `replace --replacements="#old1:#new1,#old2:#new2" --root="/path" --dry-run` - batch replace operations
5. `untagged --root="/path"` - find files without tags
6. `validate --tags="#test,#invalid-tag!"` - validation command
7. `file-tags --files="/path/file1.md,/path/file2.md"` - multiple files tags

**Enhanced Features**:
- **Interactive Confirmations**: Dangerous operations require confirmation
- **Progress Indicators**: Show progress for long-running operations
- **Output Formatting**: JSON, table, and plain text output options
- **Error Recovery**: Graceful handling of permission errors and invalid files

**Acceptance Criteria**:
- All 7 CLI commands work with proper argument validation
- Global flags (--verbose, --dry-run, --config) work across all commands
- Interactive confirmations for destructive operations
- Comprehensive help with examples for each command
- Error messages are user-friendly and actionable
- RunCmd function enables comprehensive testing
- Output formatting is consistent and configurable

**End-to-End Test**:
```go
package main_test

func TestCliE2E(t *testing.T) {
    tests := []struct {
        name        string
        args        []string
        expectError bool
    }{
        {
            name: "FindCommand",
            args: []string{"obsidian-tag-manager", "find", "--tags=#golang,#python", "--root=/test"},
        },
        {
            name: "InfoCommand",
            args: []string{"obsidian-tag-manager", "info", "--tags=#golang,#python", "--root=/test"},
        },
        {
            name: "ListWithPattern", 
            args: []string{"obsidian-tag-manager", "list", "--root=/test", "--pattern=^#dev"},
        },
        {
            name: "ReplaceBatch",
            args: []string{"obsidian-tag-manager", "replace", "--replacements=#old1:#new1,#old2:#new2", "--root=/test", "--dry-run"},
        },
        {
            name: "ValidateCommand",
            args: []string{"obsidian-tag-manager", "validate", "--tags=#test,#invalid!"},
        },
        {
            name: "FileTagsCommand",
            args: []string{"obsidian-tag-manager", "file-tags", "--files=/test/file1.md,/test/file2.md"},
        },
        {
            name:        "InvalidCommand",
            args:        []string{"obsidian-tag-manager", "invalid"},
            expectError: true,
        },
        {
            name:        "MissingArgs",
            args:        []string{"obsidian-tag-manager", "find"},
            expectError: true,
        },
    }
    
    for _, test := range tests {
        t.Run(test.name, func(t *testing.T) {
            err := RunCmd(test.args)
            if test.expectError {
                require.Error(t, err)
            } else {
                require.NoError(t, err)
            }
        })
    }
    
    // Test global verbose flag
    err := RunCmd([]string{"obsidian-tag-manager", "-v", "list", "--root=/test"})
    require.NoError(t, err)
}
```

### Phase 3: MCP Server Implementation
**Milestone**: Production-ready MCP server with all seven tools and enhanced capabilities

**Deliverables**:
- `server.go` - MCP server implementation using official Go SDK
- All 8 MCP tools with comprehensive JSON schemas and validation
- Token limit handling with intelligent pagination and truncation
- Graceful shutdown and error recovery
- Structured logging for debugging and monitoring

**MCP Tools (Enhanced)**:
1. **find_files_by_tags** - find files by multiple tags with result limits
2. **get_tags_info** - detailed information for multiple tags with statistics
3. **list_all_tags** - with sorting, filtering, and pagination
4. **replace_tags_batch** - batch replacement of multiple tags
5. **get_untagged_files** - find files without tags
6. **validate_tags** - validation tool with fix suggestions
7. **get_files_tags** - get tags for multiple files with content preview

**Advanced Features**:
- **Token Management**: Smart truncation and pagination for large results
- **Error Recovery**: Graceful handling of file system errors and permission issues
- **Configuration Support**: Runtime configuration via MCP parameters

**MCP Server Structure**:
```go
func runMCPServer(configPath string) error {
    config, err := loadConfig(configPath)
    if err != nil {
        return fmt.Errorf("failed to load config: %w", err)
    }
    
    manager := NewDefaultTagManager(config)
    
    server := mcp.NewServer(&mcp.Implementation{
        Name:    "obsidian-tag-manager",
        Version: "1.0.0",
    }, nil)
    
    // Add all eight MCP tools with proper error handling
    addMCPTools(server, manager)
    
    // Setup graceful shutdown
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    // Handle signals for graceful shutdown
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    go func() {
        <-sigChan
        cancel()
    }()
    
    return server.Run(ctx, &mcp.StdioTransport{})
}
```

**Acceptance Criteria**:
- All 7 MCP tools respond correctly with comprehensive error handling
- Token limits respected with intelligent truncation (max 10k tokens)
- Graceful shutdown handling for interrupted operations
- Comprehensive logging for debugging MCP communication
- Memory usage stays reasonable during large repository operations
- JSON-RPC 2.0 protocol compliance verified through testing

**End-to-End Test**:
```go
package mcp_test

func TestMcpServerE2E(t *testing.T) {
    server := setupTestMCPServer()
    client := setupTestMCPClient(server)
    
    tests := []struct {
        name            string
        tool            string
        args            map[string]any
        expectError     bool
        checkTokenLimit bool
    }{
        {
            name: "FindFilesByTags",
            tool: "find_files_by_tags",
            args: map[string]any{
                "root_path": "/test/repo",
                "tags":      []string{"#golang", "#python"},
            },
        },
        {
            name: "LargeResultPagination",
            tool: "list_all_tags",
            args: map[string]any{
                "max_results": 50,
                "root_path":   "/large/repo",
            },
            checkTokenLimit: true,
        },
        {
            name: "TagValidation",
            tool: "validate_tags",
            args: map[string]any{"tags": []string{"#valid", "#invalid!"}},
        },
        {
            name: "InvalidPathError",
            tool: "find_files_by_tags",
            args: map[string]any{
                "root_path": "/nonexistent",
                "tags":      []string{"#test"},
            },
            expectError: true,
        },
    }
    
    for _, test := range tests {
        t.Run(test.name, func(t *testing.T) {
            response, err := client.CallTool(test.tool, test.args)
            
            if test.expectError {
                require.Error(t, err)
                return
            }
            
            require.NoError(t, err)
            require.NotNil(t, response)
            
            if test.checkTokenLimit {
                content := strings.Join(extractTextContent(response), "")
                assert.Less(t, len(content)/4, 10000)
            }
        })
    }
}

func BenchmarkMcpServerLargeRepo(b *testing.B) {
    server := setupTestMCPServer()
    client := setupTestMCPClient(server)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := client.CallTool("list_all_tags", map[string]any{
            "root_path": "/large/test/repo",
        })
        require.NoError(b, err)
    }
}
```

### Phase 4: Production Readiness & Documentation
**Milestone**: Production-ready tool with comprehensive testing, documentation

**Deliverables**:
- **Comprehensive Test Suite** (>70% coverage with critical edge cases)
- **Complete Documentation** including Claude Code integration guide
- **Release Engineering** with CI/CD github actions for running tests on PRs

**Testing Enhancements**:
- **Edge Case Coverage**: UTF-8 BOM, different line endings, malformed YAML
- **Concurrent Access Tests**: File modification during operations
- **Memory Profiling**: Large repository memory usage optimization
- **MCP Protocol Compliance**: Full JSON-RPC 2.0 specification testing
- **Error Recovery Tests**: Permission failures, disk space, network interruptions

**Documentation Deliverables**:
- **User Guide**: Complete CLI usage with examples
- **Claude Code Integration**: Step-by-step MCP setup guide
- **Configuration Reference**: All settings and patterns explained
- **Troubleshooting Guide**: Common issues and solutions
- **API Reference**: Complete MCP tool specification

**Security & Reliability**:
- **Input Sanitization**: Protection against path traversal and injection
- **File System Safety**: Batch operations with proper permissions

**Acceptance Criteria**:
- **Documentation**: Complete user and integration documentation
- **Compatibility**: Works on macOS, Linux, and Windows
- **Claude Code Integration**: Seamless MCP server operation

**Critical Test Scenarios**:
```go
package tagmanager_test

func TestProductionScenarios(t *testing.T) {
    tests := []struct {
        name     string
        scenario func(t *testing.T)
    }{
        {
            name:     "ConcurrentFileModifications", 
            scenario: testConcurrentModifications,
        },
        {
            name:     "InvalidFileEncodingHandling",
            scenario: testEncodingEdgeCases,
        },
        {
            name:     "PermissionErrorRecovery",
            scenario: testPermissionErrors,
        },
        {
            name:     "McpProtocolEdgeCases",
            scenario: testMCPProtocolEdgeCases,
        },
        {
            name:     "ConfigurationValidation",
            scenario: testConfigValidation,
        },
        {
            name:     "BatchOperationErrorHandling",
            scenario: testBatchErrorHandling,
        },
    }
    
    for _, test := range tests {
        t.Run(test.name, test.scenario)
    }
}

func testConcurrentModifications(t *testing.T) {
    const timeout = 2 * time.Second
    const modifyDelay = 100 * time.Millisecond
    
    manager := NewDefaultTagManager(DefaultConfig())
    
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()
    
    go func() {
        time.Sleep(modifyDelay)
        err := os.WriteFile("/test/repo/file.md", []byte("# Modified\n#new-tag"), 0644)
        require.NoError(t, err)
    }()
    
    result, err := manager.ReplaceTagsBatch(ctx, []TagReplacement{{OldTag: "#old", NewTag: "#new"}}, "/test/repo", false)
    // Should handle concurrent modifications gracefully
    assert.NotNil(t, result)  // May have errors but should not panic
}
```

**Final Deliverable**:
- **Production-ready binary** with comprehensive CLI and MCP capabilities
- **Complete documentation suite** for users and developers  
- **Claude Code integration package** with configuration templates
- **Security audit report** confirming safe operation

## Dependencies & Technical Details

### MCP Go SDK
- **Primary**: `github.com/modelcontextprotocol/go-sdk` (Official SDK)
- **Status**: Pre-v1.0.0, actively developed with Google collaboration
- **Target**: v1.0.0 release planned for September 2025
- **Features**: Full JSON-RPC 2.0 support, type-safe interfaces, multiple transports
- **Decision**: Direct usage without abstraction layer - accepting risk of API changes

### Memory-Efficient Iterator Implementation
```go
// Key benefits of iter.Seq2 for large directories:
// 1. Lazy evaluation - files processed one at a time
// 2. Early termination via yield return value
// 3. No intermediate slice allocations
// 4. Context cancellation support
// 5. Automatic memory cleanup as iteration progresses

func (s *FilesystemScanner) ScanDirectory(ctx context.Context, rootPath string, excludePaths []string) iter.Seq2[FileTagInfo, error] {
    return func(yield func(FileTagInfo, error) bool) {
        filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
            // Context cancellation check
            if ctx.Err() != nil {
                return ctx.Err()
            }
            
            // Process file immediately and yield
            // No accumulation in memory
            fileInfo, processErr := s.processFile(path)
            if !yield(fileInfo, processErr) {
                return fmt.Errorf("terminated by consumer")
            }
            return nil
        })
    }
}

// Consumer usage - memory stays constant
for fileInfo, err := range scanner.ScanDirectory(ctx, rootPath, nil) {
    if err != nil {
        log.Printf("Error processing file: %v", err)
        continue // Skip failed files, continue processing
    }
    // Process fileInfo immediately
    // Previous fileInfo is eligible for GC
}
```

### Non-Atomic Batch Operations
```go
// Batch operations continue despite individual failures
func (m *DefaultTagManager) ReplaceTagsBatch(ctx context.Context, replacements []TagReplacement, rootPath string, dryRun bool) (*TagReplaceResult, error) {
    result := &TagReplaceResult{
        ModifiedFiles: []string{},
        FailedFiles:   []string{},
        Errors:        []string{},
    }
    
    for _, file := range filesToProcess {
        if ctx.Err() != nil {
            break // Respect context cancellation
        }
        
        if err := m.replaceTagsInFile(file, replacements); err != nil {
            // Individual failure - record and continue
            result.FailedFiles = append(result.FailedFiles, file)
            result.Errors = append(result.Errors, err.Error())
            continue
        }
        
        // Individual success
        result.ModifiedFiles = append(result.ModifiedFiles, file)
    }
    
    return result, nil // Always returns result with success/failure breakdown
}
```

### Structured Logging with slog
```go
// logger.go
import "log/slog"

func setupLogger(verbose bool) *slog.Logger {
    opts := &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }
    if verbose {
        opts.Level = slog.LevelDebug
    }
    
    handler := slog.NewTextHandler(os.Stderr, opts)
    return slog.New(handler)
}

// Usage throughout codebase
// Example slog usage - structured logging for debugging
func (s *FilesystemScanner) ScanDirectory(ctx context.Context, rootPath string, excludePaths []string) iter.Seq2[FileTagInfo, error] {
    logger := slog.With("component", "scanner", "root_path", rootPath)
    logger.Info("Starting directory scan", "exclude_paths", excludePaths)
    
    return func(yield func(FileTagInfo, error) bool) {
        err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
            if err != nil {
                logger.Error("Walk error", "path", path, "error", err)
                // Continue despite errors - don't fail entire operation
                yield(FileTagInfo{}, err)
                return nil
            }
            
            fileInfo, processErr := s.ScanFile(ctx, path)
            logger.Debug("Processing file", "path", path, "tag_count", len(fileInfo.Tags))
            
            if !yield(fileInfo, processErr) {
                return fmt.Errorf("scan terminated by consumer")
            }
            return nil
        })
        
        if err != nil {
            logger.Error("Directory scan failed", "error", err)
        }
    }
}
```

### Installation
```bash
# Install via go install
go install github.com/thrawn01/tag-manager@latest

# Verify installation
tag-manager --help
```

### Explicitly Unsupported Features

**The following features are intentionally NOT implemented:**

1. **Concurrent File Modification Protection**
   - No file locking or modification detection
   - Files modified during operations may cause inconsistent results
   - Rationale: Obsidian sync and git provide sufficient conflict resolution

2. **Atomic Batch Operations** 
   - Failed individual operations do not rollback successful ones
   - Partial success is expected and reported
   - Rationale: Tag operations are idempotent and can be safely retried

3. **Performance Metrics Collection**
   - No runtime performance monitoring
   - No operation timing or throughput metrics
   - Rationale: Keep implementation simple, use external tools for monitoring

4. **MCP SDK Abstraction Layer**
   - Direct dependency on official MCP Go SDK
   - No fallback to alternative libraries
   - Rationale: Accept breaking changes risk for simpler codebase

5. **File Backup/Restoration**
   - No automatic backup creation
   - No built-in rollback functionality
   - Rationale: Obsidian sync and git already provide version control

6. **Complex Error Type Hierarchy**
   - Simple string-based error reporting
   - No typed errors unless required for flow control
   - Rationale: Avoid over-engineering for edge cases

## Claude Code Integration

### MCP Configuration
```json
{
  "mcpServers": {
    "obsidian-tags": {
      "command": "tag-manager",
      "args": ["-mcp"],
      "env": {}
    }
  }
}
```

### Usage Flow
1. Claude Code spawns: `tag-manager -mcp`
2. Communicates via stdio with JSON-RPC calls
3. Shuts down the process when done

## Performance Considerations

- **Token Limits**: Implement pagination for large result sets
- **File I/O**: Minimal file reading, no caching (per requirements)  
- **Memory**: Stream processing for large directories

## Error Handling Strategy

- **Invalid paths**: Clear error messages with suggested fixes
- **Permission issues**: Graceful degradation with partial results
- **Malformed files**: Continue processing, report issues
- **Network issues**: Not applicable (local stdio server)

## Testing Strategy

- **Unit Tests**: All core functionality with >90% coverage
- **Integration Tests**: End-to-end CLI and MCP scenarios
- **CLI Testing**: RunCmd function enables easy CLI testing