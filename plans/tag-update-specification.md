# Technical Specification: Tag Add/Remove Functionality

## Review Status
- Review Cycles Completed: 2
- Final Approval: Approved
- Outstanding Concerns: None

## 1. Overview
Add tag add/remove functionality to the tag-manager CLI tool and MCP server. This feature allows users to add and remove tags from specific Obsidian notes, with preference for placing tags in YAML frontmatter and automatic migration of hashtags from the top of files.

## 2. Current State Analysis
- Affected modules: cli.go, manager.go, mcp.go, types.go
- Current behavior: Tool can read tags from both hashtags and YAML frontmatter, but only supports replace operations using regex patterns
- Relevant ADRs reviewed: None found in codebase
- Technical debt identified: Current tag modification uses regex patterns; new functionality will add YAML parsing for frontmatter manipulation

## 3. Architectural Context
### Relevant ADRs
None found in the codebase.

### Architectural Principles
- Interface-based design with TagManager interface
- Non-atomic batch operations (continue on errors, report individual failures)
- Memory-efficient streaming processing using Go 1.23 iterators
- Consistent CLI and MCP tool interfaces

## 4. Requirements
### Functional Requirements
- REQ-001: Add new `update` command accepting `--add` and/or `--remove` parameters with comma-separated tag lists
- REQ-002: Support adding multiple tags in single operation, skip tags that already exist anywhere in the file
- REQ-003: Support removing multiple tags from both frontmatter and body (strip # prefix from body tags)
- REQ-004: Always add new tags to frontmatter in quoted list format, create frontmatter section if none exists
- REQ-005: Migrate top-of-file hashtags to frontmatter during add/remove operations
- REQ-006: Convert existing array format frontmatter tags `["tag1", "tag2"]` to list format during updates
- REQ-007: Preserve other frontmatter fields when modifying tags
- REQ-008: Support dry-run mode for CLI only (not MCP)
- REQ-009: Filter conflicting tags from both add/remove lists, error if no operations remain
- REQ-010: Process specific files relative to `--root` path, not entire vault directory

### Non-Functional Requirements
- Performance: Process files individually without loading entire vault into memory
- Security: Use existing path validation to prevent directory traversal
- Reliability: Best-effort processing with individual error reporting per file

## 5. Technical Approach
### Chosen Solution
Extend TagManager interface with UpdateTags method. Use YAML parsing with gopkg.in/yaml.v3 for frontmatter manipulation while maintaining existing regex patterns for replace operations.

### Rationale
This approach maintains backward compatibility while adding reliable YAML handling. Existing replace functionality remains unchanged using regex patterns.

### Top-of-File Definition
Top-of-file hashtags are defined as: hashtags that appear at the beginning of the file content where no non-hashtag words precede them before the first occurrence of words without hashtags.

**Example:**
```markdown
#tag1 #tag2 #tag3
# Document Title ← First non-hashtag words
Content with #body-tag remains in body
```
Only `#tag1 #tag2 #tag3` are considered top-of-file and will migrate to frontmatter.

### Component Changes
- manager.go: Add UpdateTags method with YAML frontmatter parsing
- cli.go: Add updateCommand function and routing
- mcp.go: Add UpdateTagsTool function and parameter struct
- types.go: Add TagUpdateParams and TagUpdateResult structs

## 6. Dependencies and Impacts
- External dependencies: gopkg.in/yaml.v3 (already imported for config parsing)
- Internal dependencies: Existing Scanner and Validator interfaces unchanged
- Database impacts: None (filesystem-based tool)

## 7. Backward Compatibility
### Is this project in production?
- [x] No - Not need to maintain backward compatibility

### Breaking Changes Allowed
- [x] Yes - Breaking changes are allowed

### Requirements
- Compatibility constraints: All existing CLI commands and MCP tools must work unchanged

## 8. Testing Strategy
- Functional testing approach: Test tag addition, removal, migration logic, frontmatter parsing, conflict resolution via CLI.
- User acceptance criteria: Tags appear in frontmatter quoted list format, accurate file modification reporting

## 9. Implementation Notes
### Interface Definition
```go
// Add to TagManager interface in manager.go
UpdateTags(ctx context.Context, addTags []string, removeTags []string, rootPath string, filePaths []string, dryRun bool) (*TagUpdateResult, error)
```

### Data Structures
```go
// Add to types.go - Single shared struct for both CLI and MCP
type TagUpdateParams struct {
    AddTags    []string `json:"add_tags"`
    RemoveTags []string `json:"remove_tags"`
    Root       string   `json:"root"`
    FilePaths  []string `json:"file_paths"`
}

type TagUpdateResult struct {
    ModifiedFiles []string          `json:"modified_files"`
    Errors        []string          `json:"errors,omitempty"`        // includes filename: error format
    TagsAdded     map[string]int    `json:"tags_added"`              // tag -> count of files
    TagsRemoved   map[string]int    `json:"tags_removed"`            // tag -> count of files  
    FilesMigrated []string          `json:"files_migrated"`          // files with hashtag migration
}
```

### CLI Command Format
```bash
tag-manager update --add="tag1,tag2" --remove="old-tag" --root="/vault" file1.md file2.md [--dry-run]
tag-manager update --add="new-tag" --root="/vault" notes/document.md
tag-manager update --remove="obsolete" --root="/vault" *.md
```

### File Path Resolution Examples
File paths are resolved relative to the `--root` parameter:
```bash
# Command with --root="/Users/john/vault"
tag-manager update --add="golang" --root="/Users/john/vault" projects/readme.md daily/2024-01-15.md

# Resolves to these absolute paths:
# /Users/john/vault/projects/readme.md
# /Users/john/vault/daily/2024-01-15.md

# Current working directory is ignored when --root is specified
cd /tmp
tag-manager update --add="notes" --root="/vault" file.md  # Still resolves to /vault/file.md
```

### Frontmatter Format
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

### Implementation Order
1. Add UpdateTags method signature to TagManager interface
2. Implement YAML frontmatter parsing and manipulation logic
3. Implement top-of-file hashtag detection and migration
4. Add conflict resolution for add/remove tag lists
5. Add CLI update command with file path handling
6. Add MCP UpdateTagsTool
7. Add comprehensive unit and integration tests

### Error Handling
- Malformed YAML frontmatter: Skip file, add to errors list with filename
- File permission errors: Skip file, add to errors list with filename
- Invalid file paths: Skip file, add to errors list with filename
- Empty operation after conflict resolution: Return error before processing any files

## 10. ADR Recommendation
Not recommended - this is a feature addition following existing architectural patterns.

## 11. Open Questions
None - all requirements clarified with stakeholder.
