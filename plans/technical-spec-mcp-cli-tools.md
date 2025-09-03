# Technical Specification: Expose CLI Commands as MCP Server Functions

## Review Status
- Review Cycles Completed: 1
- Final Approval: ✅ APPROVED
- Outstanding Concerns: None

## 1. Overview
Expand the MCP server implementation to expose all CLI commands as streamlined MCP tools, optimized specifically for Claude Code workflows. This will provide Claude with comprehensive tag management capabilities while maintaining clean, focused APIs.

## 2. Current State Analysis
- **Affected modules**: `/Users/thrawn/Development/tag-manager/mcp.go`
- **Current behavior**: MCP server only exposes `list_all_tags` tool with basic parameters
- **Relevant ADRs reviewed**: None found in codebase
- **Technical debt identified**: MCP functionality significantly limited compared to CLI capabilities

## 3. Architectural Context
### Relevant ADRs
- No existing ADRs found in the codebase

### Architectural Principles
- Interface-based design with dependency injection (from existing TagManager interface)
- Non-atomic batch operations that continue on errors (established pattern)
- Memory-efficient streaming using Go 1.23 iterators
- Transport abstraction for testing vs production MCP usage

## 4. Requirements

### Functional Requirements
- **REQ-001**: Expose all 7 CLI commands as MCP tools with streamlined parameter sets
  - **Acceptance Criteria**: `find_files_by_tags`, `get_tags_info`, `list_all_tags` (replaced), `replace_tags_batch`, `get_untagged_files`, `validate_tags`, `get_files_tags` tools available
- **REQ-002**: Return structured data only, optimized for Claude Code processing
  - **Acceptance Criteria**: All tools return structured data as second parameter, no `mcp.CallToolResult` content
- **REQ-003**: Use snake_case parameter names following MCP guidelines
  - **Acceptance Criteria**: All parameter structs use snake_case field names with appropriate JSON tags
- **REQ-004**: No backward compatibility requirements
  - **Acceptance Criteria**: Existing `list_all_tags` tool can be completely replaced with enhanced version
- **REQ-005**: Configure result limits per tool with reasonable defaults for Claude Code usage
  - **Acceptance Criteria**: All tools accept configurable limits, default to return all results (no arbitrary limits)

### Non-Functional Requirements
- **Performance**: Tools should handle 1000+ file vaults efficiently using existing iterator patterns
- **Security**: Maintain existing path validation and directory traversal protection, require absolute paths
- **Reliability**: Preserve non-atomic behavior for batch operations with individual error reporting
- **Safety**: Destructive operations default to safe settings (dry_run=true for replace operations)

## 5. Technical Approach
### Chosen Solution
Replace existing MCP server implementation with 7 comprehensive tools:
- Dedicated parameter struct per tool using snake_case with JSON tags
- Tool handler functions that return structured data only (no mcp.CallToolResult content)
- Regex pattern matching for filtering operations
- Absolute path requirements for all file operations
- Configurable limits with sensible defaults for Claude Code workflows

### Rationale
- **Claude Code Optimized**: Returns only structured data that Claude can process directly
- **MCP Standards Compliant**: Uses snake_case parameters as per MCP guidelines
- **No Legacy Constraints**: Complete rewrite allows optimal design for Claude Code integration
- **Performance Conscious**: Configurable limits allow Claude to control data volume
- **Safety First**: Conservative defaults for destructive operations

### Component Changes
- **mcp.go**: Complete replacement of MCP tool registration and handler functions
- **No changes needed**: TagManager interface, CLI commands, core business logic remain unchanged

## 6. Dependencies and Impacts
- **External dependencies**: No new dependencies required
- **Internal dependencies**: Uses existing TagManager interface methods
- **Database impacts**: None - read-only operations except replace_tags_batch which uses existing file modification patterns

## 7. Backward Compatibility
### Is this project in production?
- [ ] No - Breaking changes are permitted without migration

### Breaking Changes Allowed
- [x] Yes - Complete replacement of MCP tools is permitted

### Breaking Changes
- Complete replacement of `list_all_tags` tool with enhanced version
- New parameter naming convention (snake_case)
- Changed return format (structured data only)

## 8. Testing Strategy
- **Unit testing approach**: Test each new tool handler function with mock TagManager
- **Integration testing needs**: Update existing MCP integration tests to use new tool signatures
- **User acceptance criteria**: All CLI command functionality accessible via MCP tools with appropriate parameter translation

## 9. Implementation Notes
- **Estimated complexity**: Medium - complete rewrite but follows clear patterns
- **Suggested implementation order**: 
  1. Update existing `list_all_tags` with new signature and parameters
  2. Add read-only operations (`find_files_by_tags`, `get_tags_info`, `get_untagged_files`, `get_files_tags`, `validate_tags`)
  3. Add destructive `replace_tags_batch` operation with safety defaults
- **Code style considerations**: Use snake_case for JSON tags, PascalCase for Go field names, follow existing handler function pattern
- **Rollback strategy**: Revert to previous mcp.go version if issues arise

## 10. ADR Recommendation
This change does not warrant a new ADR as it follows established architectural patterns and does not introduce new design decisions or constraints.

## 11. Tool Specifications

### New MCP Tool Parameter Structures:
```go
type FindFilesByTagsParams struct {
    Tags       []string `json:"tags"`
    Root       string   `json:"root"`
    MaxResults *int     `json:"max_results,omitempty"` // nil = return all
}

type GetTagsInfoParams struct {
    Tags            []string `json:"tags"`
    Root            string   `json:"root"`
    MaxFilesPerTag  *int     `json:"max_files_per_tag,omitempty"`
}

type ListAllTagsParams struct {
    Root       string `json:"root"`
    MinCount   int    `json:"min_count"`
    Pattern    string `json:"pattern,omitempty"` // regex pattern
    MaxResults *int   `json:"max_results,omitempty"`
}

type ReplaceTagsBatchParams struct {
    Replacements []TagReplacement `json:"replacements"`
    Root         string           `json:"root"`
    DryRun       bool            `json:"dry_run"` // default: true
}

type GetUntaggedFilesParams struct {
    Root       string `json:"root"`
    MaxResults *int   `json:"max_results,omitempty"`
}

type ValidateTagsParams struct {
    Tags []string `json:"tags"`
}

type GetFilesTagsParams struct {
    FilePaths []string `json:"file_paths"`
    MaxFiles  *int     `json:"max_files,omitempty"`
}
```

### Tool Return Formats:
- **All tools return**: `(nil, structured_data, error)` instead of `(*mcp.CallToolResult, structured_data, error)`
- **Structured data types**: Use existing types from `types.go` (`TagInfo`, `FileTagInfo`, `TagReplaceResult`, `ValidationResult`)

### Tool Descriptions for Registration:
1. **`find_files_by_tags`**: "Find files containing specific tags"
2. **`get_tags_info`**: "Get detailed information about specific tags including file lists"  
3. **`list_all_tags`**: "List all tags with usage statistics and optional filtering"
4. **`replace_tags_batch`**: "Replace/rename tags across multiple files with batch operation"
5. **`get_untagged_files`**: "Find files that don't have any tags"
6. **`validate_tags`**: "Validate tag syntax and get suggestions for invalid tags"
7. **`get_files_tags`**: "Get all tags associated with specific files"

## 12. Error Handling Strategy
- **Parameter validation errors**: Return error immediately with descriptive message
- **Business logic errors**: Return error with context about what failed
- **Partial failures in batch operations**: Return structured data with success/failure details, no error
- **Path validation failures**: Return error immediately for security violations

## 13. Implementation Priority
**Phase 1**: Core read operations that Claude Code will use most frequently
- `list_all_tags` (enhanced)
- `find_files_by_tags`
- `get_tags_info`

**Phase 2**: File-specific operations  
- `get_untagged_files`
- `get_files_tags`
- `validate_tags`

**Phase 3**: Destructive operations with safety controls
- `replace_tags_batch`

## 14. Final Review Results
- **Implementation Readiness**: ✅ READY - Complete implementation details provided
- **Technical Completeness**: ✅ COMPLETE - All technical details present
- **Consistency**: ✅ CONSISTENT - No conflicts identified
- **Clarity**: ✅ CLEAR - All requirements unambiguously defined

**Status**: APPROVED FOR IMPLEMENTATION
