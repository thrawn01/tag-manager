# Implementation Plan: Tag Add/Remove Functionality

## Requirements Summary

### Functional Requirements
- **REQ-001**: Add new `update` command accepting `--add` and/or `--remove` parameters with comma-separated tag lists
- **REQ-002**: Support adding multiple tags in single operation, skip tags that already exist anywhere in the file
- **REQ-003**: Support removing multiple tags from both frontmatter and body (strip # prefix from body tags)
- **REQ-004**: Always add new tags to frontmatter in quoted list format, create frontmatter section if none exists
- **REQ-005**: Migrate top-of-file hashtags to frontmatter during add/remove operations
- **REQ-006**: Convert existing array format frontmatter tags `["tag1", "tag2"]` to list format during updates
- **REQ-007**: Preserve other frontmatter fields when modifying tags
- **REQ-008**: Support dry-run mode for CLI only (not MCP)
- **REQ-009**: Filter conflicting tags from both add/remove lists, error if no operations remain
- **REQ-010**: Process specific files relative to `--root` path, not entire vault directory

### Technical Requirements
- **Performance**: Process files individually without loading entire vault into memory
- **Security**: Use existing path validation to prevent directory traversal
- **Reliability**: Best-effort processing with individual error reporting per file
- **Compatibility**: All existing CLI commands and MCP tools must work unchanged

### Top-of-File Definition
Top-of-file hashtags are defined as: hashtags that appear at the beginning of the file content where no non-hashtag words precede them before the first occurrence of words without hashtags.

**Example:**
```markdown
#tag1 #tag2 #tag3
# Document Title ← First non-hashtag words
Content with #body-tag remains in body
```
Only `#tag1 #tag2 #tag3` are considered top-of-file and will migrate to frontmatter.

### Command Format Examples
```bash
tag-manager update --add="tag1,tag2" --remove="old-tag" --root="/vault" file1.md file2.md [--dry-run]
tag-manager update --add="new-tag" --root="/vault" notes/document.md
tag-manager update --remove="obsolete" --root="/vault" *.md
```

### Expected Frontmatter Format
Always create/update to quoted list format:
```yaml
---
title: "Document Title"
tags:
  - "tag1"
  - "tag2" 
  - "migrated-hashtag"
date: 2024-01-15
---
```

## Pattern Analysis

### Existing Code Patterns with References

**Interface-Based Architecture** (`manager.go:13-21`)
- `TagManager` interface defines all operations with consistent signatures
- Dependency injection using Scanner and Validator interfaces  
- New functionality should extend the interface maintaining this pattern

**Error Handling** (`manager.go:42-45`, `validator.go:126-149`)
- Path validation at method start: `validator.ValidatePath(rootPath)`
- Error wrapping: `fmt.Errorf("context: %w", err)`
- Non-atomic batch operations: continue on individual failures, collect errors

**CLI Command Pattern** (`cli.go:73-91`)
- Switch statement routing: `case "command": return commandFunction(ctx, manager, args...)`
- Flag parsing: `fs := flag.NewFlagSet("command", flag.ContinueOnError)`
- JSON output support with `--json` flag
- Command signature: `func commandNameCommand(ctx context.Context, manager TagManager, args []string, flags...) error`

**MCP Tool Pattern** (`mcp.go:55-141`)
- Parameter structs with JSON tags (e.g., `FindFilesByTagsParams`)
- Tool handlers: `func(ctx context.Context, req *mcp.CallToolRequest, args ParamsStruct, manager TagManager) (*mcp.CallToolResult, any, error)`
- Tool registration: `mcp.AddTool(server, &mcp.Tool{...}, handler)`
- Result limiting helpers for performance

**File Processing Pattern** (`manager.go:265-293`)
- Read file → modify content string → write if changed and not dry-run
- Regex-based replacement using `regexp.QuoteMeta()` for safety
- Content modification pattern: `originalContent -> modifiedContent -> conditional write`

**Tag Normalization** (`manager.go:295-307`)
- `normalizeTag()`: `strings.TrimSpace()` + `strings.TrimPrefix(tag, "#")`
- Batch normalization with `normalizeTags()` helper
- Applied consistently across all tag operations

**YAML Processing** (`config.go:42-48`)
- Uses `gopkg.in/yaml.v3` for parsing (already available)
- Merges defaults with user configuration
- Error handling for malformed YAML

**Test Patterns** (`manager_test.go:1-50`, `cli_test.go:1-50`)
- External test package: `package tagmanager_test`
- Setup: `t.TempDir()` + `os.WriteFile()` for test data
- Integration tests using actual CLI with in-memory MCP transports
- Table-driven tests with `[]struct` pattern

## Implementation Plan

### Phase 1: Core Infrastructure
**Functionality**: Add UpdateTags method to TagManager interface and implement basic YAML frontmatter parsing

**Acceptance Criteria:**
- UpdateTags method added to TagManager interface
- Basic YAML frontmatter parsing implemented
- Can read existing frontmatter and preserve other fields
- Unit tests validate YAML parsing functionality

**Code Architecture:**

**Interface Extension** (`manager.go:13-21`)
```go
// Add to TagManager interface
UpdateTags(ctx context.Context, addTags []string, removeTags []string, rootPath string, filePaths []string, dryRun bool) (*TagUpdateResult, error)
```

**Function Responsibilities:**
- Path validation: Follow pattern from `manager.go:42-45` using `validator.ValidatePath()`
- Tag normalization: Apply `normalizeTags()` pattern from `manager.go:301-307`
- File processing: Use individual file processing approach from `manager.go:265-293`
- Error handling: Non-atomic pattern from `ReplaceTagsBatch` - continue on errors, collect failures
- Dry-run handling: When dryRun=true, perform all operations but skip file writes (CLI passes true/false, MCP always passes false)

**Data Structures** (add to `types.go` at end of file)
```go
// Add to types.go
type TagUpdateParams struct {
    AddTags    []string `json:"add_tags"`
    RemoveTags []string `json:"remove_tags"`
    Root       string   `json:"root"`
    FilePaths  []string `json:"file_paths"`
}

type TagUpdateResult struct {
    ModifiedFiles []string          `json:"modified_files"`
    Errors        []string          `json:"errors,omitempty"`
    TagsAdded     map[string]int    `json:"tags_added"`
    TagsRemoved   map[string]int    `json:"tags_removed"`
    FilesMigrated []string          `json:"files_migrated"`
}
```

**YAML Parsing Functions:**
```go
// YAML frontmatter parsing utilities
func (m *DefaultTagManager) parseFrontmatter(content string) (map[string]interface{}, string, error)
func (m *DefaultTagManager) serializeFrontmatter(data map[string]interface{}) (string, error)
func (m *DefaultTagManager) updateFrontmatterTags(data map[string]interface{}, addTags, removeTags []string) ([]string, []string)
```

**Function Responsibilities:**
- YAML parsing: Use `gopkg.in/yaml.v3` following pattern from `config.go:42-48`
- Tag format conversion: Always output quoted list format per specification
- Preserve other fields: Maintain all non-tag frontmatter data
- Error handling: On YAML parse error, return error immediately - file will be skipped entirely

**Testing Requirements:**
```go
func TestUpdateTagsInterface(t *testing.T)
func TestYAMLFrontmatterParsing(t *testing.T)
func TestFrontmatterFieldPreservation(t *testing.T)
```

**Test Objectives:**
- Interface method exists and has correct signature
- YAML parsing handles various frontmatter formats correctly
- Non-tag fields preserved during tag updates
- Error handling for malformed YAML

**Validation Commands:**
```bash
go test -v -run TestUpdateTags
go test -v -run TestYAMLFrontmatter
```

**Context for Implementation:**
- Use YAML library already imported in `config.go:6`
- Follow interface pattern from existing TagManager methods
- Reference error handling from `ReplaceTagsBatch` method

---

### Phase 2: Tag Operation Logic
**Functionality**: Implement tag addition, removal, and conflict resolution logic

**Acceptance Criteria:**
- Tags can be added to frontmatter in quoted list format
- Tags can be removed from both frontmatter and body content
- Conflicting tags filtered from add/remove lists
- Duplicate tag detection prevents redundant additions

**Code Architecture:**

**Core Tag Operations:**
```go
// Add to DefaultTagManager
func (m *DefaultTagManager) addTagsToFile(filePath string, addTags []string, dryRun bool) (*TagOperationResult, error)
func (m *DefaultTagManager) removeTagsFromFile(filePath string, removeTags []string, dryRun bool) (*TagOperationResult, error)
func (m *DefaultTagManager) resolveTagConflicts(addTags, removeTags []string) ([]string, []string, error)
```

**Function Responsibilities:**
- Tag conflict resolution: Case-insensitive matching, remove duplicates and overlapping tags from both lists, error if no operations remain (REQ-009)
- Add operation: Check for existing tags case-insensitively, add only new ones to frontmatter (REQ-002, REQ-004)
- Remove operation: Remove from both frontmatter and body, strip # prefix from body tags (REQ-003)
- File modification: Follow pattern from `replaceTagsInFile` using string manipulation

**Tag Processing Logic:**
```go
// Internal helper functions
func (m *DefaultTagManager) extractExistingTags(content string) []string
func (m *DefaultTagManager) createFrontmatterSection(tags []string) string
func (m *DefaultTagManager) removeHashtagsFromBody(content string, tags []string) string
```

**Function Responsibilities:**
- Existing tag detection: Use Scanner pattern from `scanner.go:117-155`
- Frontmatter creation: Generate YAML section if none exists (REQ-004)
- Hashtag removal: Use regex replacement pattern from `replaceTagsInFile`
- Format conversion: Always output quoted list format, convert arrays to lists (REQ-004, REQ-006)

**Testing Requirements:**
```go
func TestTagConflictResolution(t *testing.T)
func TestAddTagsToFile(t *testing.T)
func TestRemoveTagsFromFile(t *testing.T)
func TestDuplicateTagHandling(t *testing.T)
```

**Test Objectives:**
- Conflict resolution removes overlapping tags appropriately
- Tags added only if not already present anywhere in file
- Tags removed from both frontmatter and body content
- Error handling for empty operations after conflict resolution

**Validation Commands:**
```bash
go test -v -run TestTagConflict
go test -v -run TestAddTags
go test -v -run TestRemoveTags
```

**Context for Implementation:**
- Use tag extraction from `scanner.go:117-155` for existing tag detection
- Follow regex replacement pattern from `manager.go:274-286`
- Reference tag normalization from `manager.go:295-307`

---

### Phase 3: Hashtag Migration Logic
**Functionality**: Detect and migrate top-of-file hashtags to frontmatter during tag operations

**Acceptance Criteria:**
- Top-of-file hashtags identified per specification definition
- Migration occurs automatically during add/remove operations
- Original hashtag locations cleaned up after migration
- Files with migrations tracked in result

**Code Architecture:**

**Migration Detection:**
```go
// Add to DefaultTagManager
func (m *DefaultTagManager) detectTopOfFileHashtags(content string) []string
func (m *DefaultTagManager) migrateHashtagsToFrontmatter(content string, topHashtags []string) string
func (m *DefaultTagManager) shouldMigrateFile(content string) bool
```

**Function Responsibilities:**
- Top-of-file detection: Identify hashtags before any non-hashtag content (ignoring whitespace/empty lines)
- Content parsing: Stop at first non-hashtag content as boundary
- Migration trigger: Only occurs when top-of-file hashtags are detected during update operations
- Content cleanup: Remove ONLY top-of-file hashtags, leave body hashtags unchanged (REQ-005)

**Hashtag Processing:**
```go
// Migration helper functions
func (m *DefaultTagManager) findFirstNonHashtagContent(content string) int
func (m *DefaultTagManager) extractTopHashtags(content string, boundary int) []string
func (m *DefaultTagManager) removeTopHashtags(content string, hashtags []string) string
```

**Function Responsibilities:**
- Boundary detection: Find first non-hashtag content (ignoring whitespace/empty lines)
- Hashtag extraction: Collect hashtags that appear before boundary only
- Content modification: Remove ONLY top-of-file hashtags, preserve all body hashtags per requirement
- Tag deduplication: Prevent duplicate tags when migrating to existing frontmatter (REQ-002)

**Testing Requirements:**
```go
func TestTopOfFileDetection(t *testing.T)
func TestHashtagMigration(t *testing.T)
func TestMigrationBoundaryDetection(t *testing.T)
func TestMigrationWithExistingFrontmatter(t *testing.T)
```

**Test Objectives:**
- Top-of-file hashtags correctly identified per specification
- Migration preserves document structure and formatting
- Boundary detection works with various content patterns
- Integration with existing frontmatter tags

**Validation Commands:**
```bash
go test -v -run TestTopOfFile
go test -v -run TestHashtagMigration
```

**Context for Implementation:**
- Use hashtag detection patterns from `scanner.go:120-126`
- Follow content modification approach from `manager.go:265-293`
- Reference boundary checking from `scanner.go:273-318`

---

### Phase 4: CLI Integration
**Functionality**: Add `update` command to CLI with file path handling and dry-run support

**Acceptance Criteria:**
- `update` command accepts `--add` and `--remove` parameters
- File paths resolved relative to `--root` parameter
- Dry-run mode supported for CLI only
- JSON output available with comprehensive results

**Code Architecture:**

**CLI Command Function:**
```go
// Add to cli.go
func updateCommand(ctx context.Context, manager TagManager, args []string, globalDryRun bool, verbose bool) error
```

**Function Responsibilities:**
- Flag parsing: Follow pattern from `replaceTagCommand` in `cli.go:276-347`
- Parameter validation: Ensure at least one of `--add` or `--remove` provided
- File path resolution: Convert relative paths to absolute using `--root`
- Result display: Support both JSON and human-readable formats

**CLI Route Integration** (`cli.go:73-91`)
```go
// Add to switch statement
case "update":
    return updateCommand(ctx, manager, remaining[1:], *dryRun, *verbose)
```

**File Path Processing:**
```go
// Helper functions for file path handling
func resolveFilePaths(rootPath string, filePaths []string) ([]string, error)
func validateUpdateParameters(addTags, removeTags []string, filePaths []string) error
```

**Function Responsibilities:**
- Path resolution: Convert relative paths to absolute using `filepath.Join(rootPath, path)` - absolute paths not allowed (REQ-010)
- Parameter validation: Check for empty operations, invalid paths (all paths must be relative to root)
- Error handling: Standard file processing errors, no special handling for non-existent vs permission errors
- Input parsing: Split comma-separated tag lists, trim whitespace (REQ-001)

**Testing Requirements:**
```go
func TestUpdateCommand(t *testing.T)
func TestFilePathResolution(t *testing.T)
func TestUpdateParameterValidation(t *testing.T)
func TestUpdateDryRunMode(t *testing.T)
```

**Test Objectives:**
- Command correctly parses flags and calls TagManager.UpdateTags
- File paths resolved relative to root directory correctly
- Dry-run mode produces expected output without file changes
- Error handling for invalid parameters and missing files

**Validation Commands:**
```bash
go test -v -run TestUpdateCommand
./tag-manager update --help
./tag-manager update --add="test" --root="/tmp/test" file.md --dry-run
```

**Context for Implementation:**
- Follow CLI command pattern from `replaceTagCommand` in `cli.go:276-347`
- Use flag parsing approach from existing commands
- Reference help text formatting from `ShowHelp` in `cli.go:93-129`

---

### Phase 5: MCP Integration
**Functionality**: Add UpdateTagsTool to MCP server for Claude Code integration

**Acceptance Criteria:**
- MCP tool registered with proper parameter structure
- Tool handler follows existing MCP patterns
- Parameter validation handled appropriately
- Results formatted for Claude Code consumption

**Code Architecture:**

**MCP Tool Handler:**
```go
// Add to mcp.go
func UpdateTagsTool(ctx context.Context, req *mcp.CallToolRequest, args TagUpdateParams, manager TagManager) (*mcp.CallToolResult, any, error)
```

**Function Responsibilities:**
- Parameter processing: Extract and validate TagUpdateParams
- Manager delegation: Call TagManager.UpdateTags with processed parameters (always pass dryRun=false for MCP)
- Result formatting: Return TagUpdateResult directly as structured data
- Error handling: Follow pattern from existing tools like `ReplaceTagsBatchTool`

**Tool Registration** (`mcp.go:178-267`)
```go
// Add to RunMCPServer tool registration section
mcp.AddTool(server, &mcp.Tool{
    Name:        "update_tags",
    Description: "Add and remove tags from specific files with automatic hashtag migration",
}, func(ctx context.Context, req *mcp.CallToolRequest, args TagUpdateParams) (*mcp.CallToolResult, any, error) {
    return UpdateTagsTool(ctx, req, args, manager)
})
```

**Parameter Structure** (already defined in Phase 1)
- Reuse `TagUpdateParams` and `TagUpdateResult` from types.go
- No additional parameter processing needed beyond existing pattern

**Testing Requirements:**
```go
func TestUpdateTagsTool(t *testing.T)
func TestMCPUpdateTagsIntegration(t *testing.T)
```

**Test Objectives:**
- MCP tool properly registered and discoverable
- Tool handler processes parameters correctly
- Integration test using in-memory transport validates full flow
- Error handling consistent with other MCP tools

**Validation Commands:**
```bash
go test -v -run TestUpdateTagsTool
go test -v -run TestMCPIntegration
```

**Context for Implementation:**
- Follow MCP tool pattern from `ReplaceTagsBatchTool` in `mcp.go:102-109`
- Use parameter struct pattern from existing MCP tools
- Reference integration testing from `cli_test.go` MCP patterns

---

### Phase 6: Comprehensive Testing and Integration
**Functionality**: Complete test coverage including integration tests and edge case handling

**Acceptance Criteria:**
- All components tested with full integration scenarios
- Edge cases covered (malformed YAML, empty files, permission errors)
- CLI and MCP integration validated end-to-end
- Performance verified with typical vault sizes

**Testing Requirements:**

**Integration Tests:**
```go
func TestUpdateTagsE2E(t *testing.T)
func TestUpdateTagsWithMigration(t *testing.T)
func TestUpdateTagsErrorHandling(t *testing.T)
func TestUpdateTagsCLIIntegration(t *testing.T)
func TestUpdateTagsMCPIntegration(t *testing.T)
```

**Test Objectives:**
- Complete workflow from CLI command to file modification
- Hashtag migration integrated with tag operations
- Error scenarios handled gracefully with proper reporting
- MCP integration using in-memory transport validates full tool chain
- Performance acceptable for typical vault sizes (100+ files)

**Edge Case Coverage:**
- Malformed YAML frontmatter (skip with error)
- Files without existing frontmatter (create new section)
- Empty add/remove lists after conflict resolution (error)
- File permission errors (skip with error report)
- Non-existent files (error on file open)

**Validation Commands:**
```bash
# Run all tests with race detection
go test -v ./... -race

# Run specific E2E tests
go test -v -run TestUpdateTagsE2E

# Test with sample vault
./tag-manager update --add="golang,testing" --remove="old" --root="/tmp/vault" *.md --dry-run

# Performance validation - create test vault and measure memory/time
mkdir -p /tmp/test-vault
for i in {1..100}; do echo -e "---\ntags: [\"tag$i\"]\n---\n#top$i\n\nContent with #body$i" > /tmp/test-vault/file$i.md; done
/usr/bin/time -l ./tag-manager update --add="new" --remove="old" --root="/tmp/test-vault" *.md

# Benchmark tests
go test -bench=BenchmarkUpdateTags -benchmem -run=^$ ./...
```

**Context for Implementation:**
- Follow E2E testing pattern from `manager_test.go:13-50`
- Use MCP integration testing from `cli_test.go` patterns
- Reference error handling validation from existing test suites

## Implementation Notes

### Top-of-File Migration Rules
1. Detection stops at any non-hashtag content (ignoring whitespace and empty lines)
2. Only top-of-file hashtags are removed during migration - body hashtags remain unchanged
3. Migration occurs automatically when top-of-file hashtags are detected during any tag update operation
4. Users are notified via `FilesMigrated` field in result when migration occurs
5. No option to disable automatic migration

### Example Transformations

**Top-of-File Migration Example:**
```markdown
// Before
#golang #programming #tutorial

# My Go Tutorial
This content has #body-tag that stays.

// After (when any update operation occurs)
---
tags: ["golang", "programming", "tutorial"]
---

# My Go Tutorial
This content has #body-tag that stays.
```

**Tag Update with Existing Frontmatter:**
```markdown
// Before
---
title: "My Document"
tags: ["existing-tag"]
author: "John"
---

Content with #old-tag to remove.

// After: update --add="new-tag" --remove="old-tag"
---
title: "My Document"
tags:
  - "existing-tag"
  - "new-tag"
author: "John"
---

Content with old-tag to remove.  // Note: # prefix stripped
```

### Conflict Resolution Algorithm
```go
// Pseudo-code for resolveTagConflicts
func resolveTagConflicts(addTags, removeTags []string) ([]string, []string, error) {
    // 1. Normalize and deduplicate both lists (case-insensitive)
    // 2. Remove intersection from both lists
    // 3. Return error if both lists become empty
    // 4. Return filtered lists
}
```

### Top-of-File Boundary Detection Algorithm
```go
// Pseudo-code for findFirstNonHashtagContent
func findFirstNonHashtagContent(content string) int {
    // 1. Split content into lines
    // 2. For each line:
    //    a. Trim whitespace
    //    b. If empty, continue to next line
    //    c. If line contains only hashtags (regex: ^(#\w+\s*)+$), continue
    //    d. If line contains any non-hashtag content, return line position
    // 3. If all lines are hashtags or empty, return end of content
}
```

### YAML Processing Strategy
- Use `gopkg.in/yaml.v3` for parsing and generation
- Always convert array format `["tag1", "tag2"]` to quoted list format during updates
- Comments in frontmatter can be removed during processing - preservation not required
- Field ordering preservation not required

### File Path Resolution Strategy  
- All file paths must be relative to `--root` parameter - absolute paths not allowed
- Use `filepath.Join(rootPath, relativePath)` for resolution
- Maintain absolute paths internally for consistency
- No special handling for symlinks or paths outside root

### Error Handling Strategy
- Malformed YAML frontmatter: Skip entire file, add to errors list with format: "filename: malformed YAML frontmatter: [error details]"
- File permission errors: Skip file, add to errors list with format: "filename: permission denied"
- Non-existent files: Standard error handling with format: "filename: no such file or directory"
- Empty operation after conflict resolution: Return error before processing any files (REQ-009)
- Non-atomic operations: Continue processing on individual file failures, report individual errors (best-effort processing requirement)
- YAML parsing during creation: If unable to create valid frontmatter, skip file and report error

### Memory and Performance Considerations
- Process files individually to maintain memory efficiency (performance requirement)
- No need for iterator pattern since operating on specific file list (REQ-010)
- Maintain existing performance characteristics of other TagManager methods

### Backward Compatibility
- All existing CLI commands and MCP tools continue to work unchanged (compatibility requirement)
- No breaking changes to existing interfaces or data structures
- New functionality is purely additive

## Dependencies and Integration Points

**External Dependencies:**
- `gopkg.in/yaml.v3` (already imported in config.go)
- `github.com/modelcontextprotocol/go-sdk/mcp` (already imported)

**Internal Integration Points:**
- TagManager interface extension (manager.go)
- CLI command routing addition (cli.go)
- MCP tool registration addition (mcp.go)
- Type definitions addition (types.go)

**File Modifications Required:**
- `manager.go`: Add UpdateTags method and supporting functions
- `cli.go`: Add updateCommand function and routing
- `mcp.go`: Add UpdateTagsTool and registration
- `types.go`: Add TagUpdateParams and TagUpdateResult structs
- Test files: Add comprehensive test coverage for all components