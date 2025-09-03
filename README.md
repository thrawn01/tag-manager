# Obsidian Tag Manager

[![Test](https://github.com/thrawn01/tag-manager/actions/workflows/test.yml/badge.svg)](https://github.com/thrawn01/tag-manager/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/thrawn01/tag-manager)](https://goreportcard.com/report/github.com/thrawn01/tag-manager)
[![Go Version](https://img.shields.io/github/go-mod/go-version/thrawn01/tag-manager)](https://github.com/thrawn01/tag-manager)
[![License](https://img.shields.io/github/license/thrawn01/tag-manager)](https://github.com/thrawn01/tag-manager/blob/main/LICENSE)

A powerful CLI tool and MCP server for managing tags in Obsidian vaults. Provides Claude Code with precise tag
management capabilities while also offering a comprehensive command-line interface for direct use.

## Features

- **🏷️ Comprehensive Tag Support**: Hashtags (`#tag`) and YAML frontmatter (arrays and lists)
- **🧠 Advanced Filtering**: Intelligent filtering of hex colors, IDs, URLs, and noise
- **⚡ Memory Efficient**: Streaming file processing using Go 1.23 iterators
- **🔄 Batch Operations**: Batch tag replacement with individual error reporting
- **🔌 Dual Mode**: CLI tool + MCP server for Claude Code integration

## Installation

```bash
go install github.com/thrawn01/tag-manager/cmd/tag-manager@latest
```
Verify installation
```bash
$ tag-manager --help
```

## Quick Start

List all tags in your Obsidian vault
```bash
$ tag-manager list

Found 652 tags:
  #daily-notes                    814 files
  #people                         49 files
  #blog                           17 files
  #startups                       17 files
  #book                           16 files
-- snip --
```

Find files with specific tags
```bash
$ tag-manager find --tags="golang,programming"

#golang (6 files):
  Interests/Technology/Concepts/Programming/Language/Golang.md
  Interests/Technology/Concepts/Programming/Patterns/Logging Guide.md
  Interests/Technology/Language/Golang Trace Profile Example.md
  Interests/Technology/Latency.md
  Calendar Notes/2022/07 July/2022-07-25.md

#programming (11 files):
  Projects/Writing/Published/Mastering HTTP REST Design.md
  Things/Education/Education.md
  Interests/Software Development.md
  Interests/Technology/Algorithms and Protocols/Binary Tree.md
  Interests/Technology/Algorithms and Protocols/Ring Buffer.md
  Interests/Technology/Concepts/Cognitive Load.md
  Interests/Technology/Programming/Language/Golang.md
  Interests/Technology/Programming/Patterns/Object Oriented Programming.md
  Interests/Technology/Latency.md
-- snip --
```

Replace a tag across your vault (dry run first!)
```bash
tag-manager replace --old="old-tag" --new="new-tag" --root="/path/to/vault" --dry-run
DRY RUN MODE - No files will be modified

Modified files: 6
```

## CLI Usage Guide

### 📋 Command Reference

| Command | Purpose | Example |
|---------|---------|---------|
| `list` | Show all tags with usage counts | `tag-manager list` |
| `find` | Find files containing specific tags | `tag-manager find --tags="golang,python"` |
| `replace` | Rename/replace tags across files | `tag-manager replace --old="old" --new="new"` |
| `untagged` | Find files without any tags | `tag-manager untagged` |
| `validate` | Check tag syntax and get suggestions | `tag-manager validate --tags="test-tag,invalid!"` |
| `file-tags` | Show tags for specific files | `tag-manager file-tags --files="file1.md,file2.md"` |
| `info` | Get detailed tag information | `tag-manager info --tags="golang,python"` |

### 🔍 **Finding Files by Tags**

```bash
# Find files with single tag
tag-manager find --tags="golang" --root="/Users/john/MyVault"

# Find files with multiple tags (OR logic)
tag-manager find --tags="golang,python,programming" --root="/Users/john/MyVault"

# Limit results and output as JSON
tag-manager find --tags="golang" --root="/Users/john/MyVault" --max-results=10 --json

# Find with hashtag prefix (both work the same)
tag-manager find --tags="#golang" --root="/Users/john/MyVault"
tag-manager find --tags="golang" --root="/Users/john/MyVault"
```

### 📊 **Listing All Tags**

```bash
# List all tags with usage counts
tag-manager list --root="/Users/john/MyVault"

# Filter tags by minimum usage count
tag-manager list --root="/Users/john/MyVault" --min-count=5

# Filter tags by pattern (contains "dev")
tag-manager list --root="/Users/john/MyVault" --pattern="dev"

# Combine filters and output as JSON
tag-manager list --root="/Users/john/MyVault" --min-count=2 --pattern="programming" --json
```

### 🔄 **Replacing/Renaming Tags**

```bash
# Single tag replacement (always dry-run first!)
tag-manager replace --old="javascript" --new="js" --root="/Users/john/MyVault" --dry-run

# Apply the changes after reviewing
tag-manager replace --old="javascript" --new="js" --root="/Users/john/MyVault"

# Multiple tag replacements in one command
tag-manager replace --replacements="js:javascript,py:python,ts:typescript" --root="/Users/john/MyVault" --dry-run

# Global dry-run flag (affects all subcommands that modify files)
tag-manager --dry-run replace --old="test" --new="testing" --root="/Users/john/MyVault"
```

### 🏷️ **Tag Information**

```bash
# Get detailed info about specific tags
tag-manager info --tags="golang,python" --root="/Users/john/MyVault"

# Limit files shown per tag
tag-manager info --tags="golang" --root="/Users/john/MyVault" --max-files-per-tag=5 --json
```

### 📝 **Finding Untagged Files**

```bash
# Find all files without any tags
tag-manager untagged --root="/Users/john/MyVault"

# Output as JSON for processing
tag-manager untagged --root="/Users/john/MyVault" --json | jq '.[] | .path'
```

### ✅ **Validating Tags**

```bash
# Validate tag syntax
tag-manager validate --tags="valid-tag,invalid!,toolong123456789"

# Get suggestions for invalid tags
tag-manager validate --tags="test-tag,123invalid,special@chars" --json
```

### 📄 **Getting Tags from Specific Files**

```bash
# Get tags from specific files
tag-manager file-tags --files="/Users/john/MyVault/note1.md,/Users/john/MyVault/note2.md"

# Process multiple files with JSON output
find /Users/john/MyVault -name "*.md" -print0 | \
  xargs -0 -I {} tag-manager file-tags --files="{}" --json
```

## Global Options

| Option | Description | Example |
|--------|-------------|---------|
| `-h, --help` | Show help message | `tag-manager -h` |
| `-v, --verbose` | Enable verbose output | `tag-manager -v list` |
| `--dry-run` | Preview changes without modifying files | `tag-manager --dry-run replace --old=test --new=testing` |
| `--config FILE` | Use custom configuration file | `tag-manager --config=custom.yaml list` |

## Configuration

### Default Configuration

The tool uses intelligent defaults optimized for Obsidian vaults:

```yaml
# These directories are automatically excluded
exclude_dirs:
  - "100 Archive"      # Common archive folder
  - "Attachments"      # Media files
  - ".git"             # Version control
  - ".obsidian"        # Obsidian settings

# These file patterns are excluded
exclude_patterns:
  - "*.excalidraw.md"  # Excalidraw drawings
  - "*.canvas"         # Canvas files

# Tag extraction patterns (advanced users only)
hashtag_pattern: "#[a-zA-Z][\\w\\-]*"
yaml_tag_pattern: "(?m)^tags:\\s*\\[([^\\]]+)\\]"
yaml_list_pattern: "(?m)^tags:\\s*$\\n((?:\\s+-\\s+.+\\n?)+)"

# Tag validation rules
min_tag_length: 3        # Minimum characters
max_digit_ratio: 0.5     # Maximum 50% digits

# Keywords that are automatically filtered out
exclude_keywords:
  - "bibr"               # Bibliography references
  - "ftn"                # Footnotes
  - "issuecomment"       # GitHub issue comments
  - "discussion"         # GitHub discussions
  - "diff-"              # Git diff markers
```

### Custom Configuration

Create a `config.yaml` file to override defaults:

```yaml
# Example: config.yaml
min_tag_length: 2
max_digit_ratio: 0.7

exclude_dirs:
  - "Archive"
  - "Templates"
  - "Daily Notes"

exclude_keywords:
  - "temp"
  - "draft"
  - "wip"

# Add custom exclusions without losing defaults
additional_exclude_dirs:
  - "Personal"
  - "Private"
```

Use with: `tag-manager --config=config.yaml list --root=~/vault`

## Tag Formats Supported

### 1. Hashtag Format (Inline Tags)
```markdown
# My Programming Note

This note covers #golang and #web-development.
I'm also learning #data-structures and #algorithms.

## Advanced Topics
- #concurrency in Go
- #design-patterns for scalable systems
```

### 2. YAML Frontmatter - Array Format
```markdown
---
title: "Advanced Go Programming"
tags: ["golang", "programming", "concurrency", "web-development"]
date: 2024-01-15
---

# Content goes here
```

### 3. YAML Frontmatter - List Format
```markdown
---
title: "Learning Python"
tags:
  - python
  - programming
  - data-science
  - machine-learning
author: John Doe
---

# Content goes here
```

### 4. Mixed Format Support
```markdown
---
tags: ["yaml-tag", "frontmatter"]
---

# Mixed Tags Example

This note has both YAML frontmatter tags above and 
inline hashtags like #programming and #tutorial.
```

## Smart Tag Filtering

The tool automatically filters out common false positives:

### ❌ **Filtered Out (False Positives)**
- **Hex Colors**: `#ff0000`, `#abc123`, `#ffffff`
- **GitHub References**: `#123`, `#456` (issue numbers)
- **URL Fragments**: `#section`, `#top`, `#http`
- **High Digit Ratio**: `#abc123456789` (>50% digits)
- **Short Tags**: `#go`, `#js` (less than 3 chars)
- **Noise Keywords**: `#bibr123`, `#ftn1`, `#issuecomment`
- **ID-like Strings**: `#aB3dEf9H2jK4lM` (looks like generated ID)

### ✅ **Kept (Real Tags)**
- **Descriptive Tags**: `#golang`, `#programming`, `#web-development`
- **Valid Short Forms**: `#api`, `#css`, `#sql` (configurable)
- **Hyphenated Tags**: `#machine-learning`, `#data-science`
- **Underscore Tags**: `#data_structures`, `#unit_testing`

## Advanced Usage Examples

### Batch Tag Management Workflow

```bash
# 1. First, explore your vault's tags
tag-manager list --root=~/vault --min-count=2

# 2. Find inconsistent naming
tag-manager list --root=~/vault --pattern="js\|javascript"

# 3. Plan replacements (dry-run)
tag-manager replace --replacements="js:javascript,py:python" --root=~/vault --dry-run

# 4. Apply changes
tag-manager replace --replacements="js:javascript,py:python" --root=~/vault

# 5. Verify results
tag-manager find --tags="javascript,python" --root=~/vault --json
```

### Finding Maintenance Issues

```bash
# Find files that need tags
tag-manager untagged --root=~/vault

# Find tags that might be typos (very low usage)
tag-manager list --root=~/vault --min-count=1 --json | jq '.[] | select(.count == 1)'

# Validate existing tags for syntax issues
tag-manager list --root=~/vault --json | \
  jq -r '.[] | .name' | \
  xargs tag-manager validate --tags
```

### Integration with Other Tools

```bash
# Export all tags as a list
tag-manager list --root=~/vault --json | jq -r '.[].name' > all-tags.txt

# Find files with specific tag and open in editor
tag-manager find --tags="todo" --root=~/vault --json | \
  jq -r '.todo[]' | \
  head -5 | \
  xargs code  # Opens in VS Code

# Generate tag usage report
tag-manager list --root=~/vault --json | \
  jq -r '["Tag", "Count", "Files"], (.[] | [.name, .count, (.files | length)]) | @csv' > report.csv
```

## Error Handling & Recovery

### Batch Operations Are Non-Atomic

```json
{
  "replacements": [
    {"old_tag": "javascript", "new_tag": "js"},
    {"old_tag": "python", "new_tag": "py"}
  ],
  "modified_files": [
    "/vault/note1.md",
    "/vault/note2.md"
  ],
  "failed_files": [
    "/vault/readonly.md"
  ],
  "errors": [
    "/vault/readonly.md: permission denied"
  ],
  "dry_run": false
}
```

**Benefits:**
- ✅ Partial success is preserved
- ✅ Individual file errors don't stop the entire operation  
- ✅ Operations are idempotent (safe to retry)
- ✅ Clear error reporting per file

### Common Error Scenarios

```bash
# Permission denied - make files writable
chmod -R u+w /path/to/vault
tag-manager replace --old="test" --new="testing" --root="/path/to/vault"

# Invalid configuration - check regex patterns
tag-manager --config=custom.yaml list --root=~/vault
# Error: invalid hashtag pattern: missing closing bracket

# Path traversal protection
tag-manager list --root="../../../etc"
# Error: path contains directory traversal
```

## MCP Server Mode (Claude Code Integration)

### Setup for Claude Code

1. **Start the MCP server:**
```bash
tag-manager -mcp
# Server starts on stdio, waiting for Claude Code connection
```

2. **Configure Claude Code** (`claude_desktop_config.json`):
```json
{
  "mcpServers": {
    "obsidian-tag-manager": {
      "command": "tag-manager",
      "args": ["-mcp"],
      "env": {
        "OBSIDIAN_VAULT_PATH": "/Users/john/MyVault"
      }
    }
  }
}
```

3. **Custom configuration for MCP:**
```bash
tag-manager -mcp --config="/path/to/vault-specific-config.yaml"
```

### Available MCP Tools

| Tool | Purpose | Parameters |
|------|---------|------------|
| `find_files_by_tags` | Find files containing tags | `tags`, `root_path`, `max_results` |
| `get_tags_info` | Detailed tag information | `tags`, `root_path`, `max_files_per_tag` |
| `list_all_tags` | List all tags with stats | `root_path`, `min_count`, `pattern`, `max_results` |
| `replace_tags_batch` | Batch tag replacement | `replacements`, `root_path`, `dry_run` |
| `get_untagged_files` | Find untagged files | `root_path`, `max_results` |
| `validate_tags` | Validate tag syntax | `tags`, `include_suggestions` |
| `get_files_tags` | Get tags from specific files | `file_paths`, `max_files` |

## Performance & Scalability

### Memory Usage
- **Constant Memory**: Uses Go iterators for streaming processing
- **Large Vaults**: Tested with 1000+ files, memory stays constant

### Performance Tips
```bash
# For very large vaults, use filters to reduce scope
tag-manager list --root=~/huge-vault --min-count=5

# Process subsets of files
tag-manager find --tags="golang" --root=~/vault/programming-notes

# Use JSON output for programmatic processing (faster parsing)
tag-manager list --root=~/vault --json | jq '.[] | select(.count > 10)'
```

### Benchmarks
- **Small Vault** (100 files): ~50ms
- **Medium Vault** (1000 files): ~500ms  
- **Large Vault** (5000 files): ~2.5s
- **Memory Usage**: <10MB regardless of vault size

## Troubleshooting

### Common Issues

#### 🚫 **"No tags found"**
```bash
# Check file extensions (only .md files processed)
find ~/vault -name "*.md" | wc -l

# Check if files have actual tags
grep -r "#" ~/vault/*.md | head -5

# Verify exclude patterns aren't too broad
tag-manager --config=minimal.yaml list --root=~/vault
```

#### 🚫 **"Permission denied"**
```bash
# Make files writable
find ~/vault -name "*.md" -not -writable
chmod u+w ~/vault/*.md

# Run dry-run first to preview changes
tag-manager --dry-run replace --old=test --new=testing --root=~/vault
```

#### 🚫 **"Invalid regex configuration"**
```bash
# Test your custom patterns
tag-manager validate --tags="test" --config=custom.yaml
# Error: invalid hashtag pattern: [missing bracket

# Use default config to verify the tool works
tag-manager list --root=~/vault  # Uses defaults
```

#### 🚫 **"MCP server not responding"**
```bash
# Test MCP server manually
tag-manager -mcp --config=test.yaml
# Should start without errors and wait for input

# Check Claude Code logs for connection issues
# Verify paths in claude_desktop_config.json
```

### Debug Mode

```bash
# Enable verbose logging
tag-manager -v list --root=~/vault

# See what files are being processed
tag-manager -v find --tags=golang --root=~/vault

# Debug MCP server (check stderr)
tag-manager -mcp -v 2>debug.log
```

### Getting Help

```bash
# General help
tag-manager --help

# Command-specific help  
tag-manager find --help
tag-manager replace --help

# Show current configuration
tag-manager list --root=. --json | jq '.config' 2>/dev/null || echo "Using defaults"
```

## Development

### Building from Source

```bash
git clone https://github.com/thrawn01/tag-manager.git
cd tag-manager

# Run tests
go test -v ./...

# Build binary
go build -o tag-manager ./cmd/tag-manager

# Install to GOPATH/bin
go install ./cmd/tag-manager
```

### Using the Makefile

```bash
make help          # Show available targets
make build         # Build binary
make test          # Run tests
make test-coverage # Run tests with coverage
make install       # Install to GOPATH/bin
make clean         # Clean build artifacts
make demo          # Create demo data and run examples
```

### Testing with Sample Data

```bash
# Create test vault
make test-data

# Run demo commands
make demo

# Test MCP server
make mcp-server  # In one terminal
# Test with Claude Code or manual JSON-RPC calls
```

## Contributing

1. **Fork the repository**
2. **Create a feature branch**: `git checkout -b feature/amazing-feature`
3. **Write tests** for new functionality
4. **Ensure tests pass**: `go test ./...`
5. **Submit a pull request**

### Code Style
- Follow standard Go conventions
- Add tests for new features
- Update documentation for user-facing changes
- Use meaningful commit messages

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Acknowledgments

- Built for the Obsidian community
- Designed for Claude Code integration
- Inspired by the need for intelligent tag management
- Built by Claude Code (See tag-manager-plan.md)
