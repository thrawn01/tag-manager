# CLI Output Refactoring Implementation Plan

## Overview

Refactor the CLI to support io.Writer injection for output capture in tests. This will enable comprehensive testing of CLI output messages, error formatting, and JSON responses without relying on direct manager method calls. Additionally, simplify the API by consolidating RunCmd and RunCmdWithOptions into a single function.

## Current State Analysis

The CLI currently has:
- Two functions: `RunCmd(args)` and `RunCmdWithOptions(args, options)`
- Hardcoded output destinations using `fmt.Print*` and `json.NewEncoder(os.Stdout)`
- Error messages returned as errors and printed by main()
- Tests that call manager methods directly, bypassing CLI output validation

### Key Discoveries:
- `RunCmdOptions` struct already exists for dependency injection (cli.go:16)
- `RunCmd` just wraps `RunCmdWithOptions` with nil options (cli.go:21-23)
- Pattern established for MCP transport injection in testing
- All output is centralized in cli.go (no output in manager.go)
- Tests in manager_test.go call UpdateTags() directly instead of using CLI

## Desired End State

After implementation:
- Single `RunCmd(args, options)` function with optional options parameter
- `RunCmdOptions` supports io.Writer injection for stdout/stderr capture
- All CLI commands write to injected writers instead of os.Stdout
- Tests can capture and validate CLI output messages
- manager_test.go uses RunCmd() for integration testing
- Error messages in output can be validated in tests

### Verification:
- Run `go test -v ./...` - all tests pass
- Tests capture and validate output for all commands
- JSON and text output modes both testable
- Error messages properly captured and validated
- All existing callers updated to new signature

## What We're NOT Doing

- Modifying business logic in manager.go
- Changing MCP server functionality
- Altering command-line argument parsing
- Modifying the main() function beyond updating the RunCmd call

## Implementation Approach

Simplify the API by removing the wrapper function and extending `RunCmdOptions` to include io.Writer injection, following the established dependency injection pattern used for MCP transport testing.

## Phase 1: Simplify API and Add Writer Support

### Overview
Consolidate RunCmd and RunCmdWithOptions into a single function, add io.Writer fields to RunCmdOptions, and create a command context structure to pass writers through the command execution chain.

### Changes Required:

#### 1. Remove Old RunCmd and Rename RunCmdWithOptions
**File**: `cli.go`
**Changes**: Delete old RunCmd wrapper and rename RunCmdWithOptions

```go
// Delete lines 21-23 (old RunCmd function)
// Rename RunCmdWithOptions to RunCmd
func RunCmd(args []string, options *RunCmdOptions) error
```

**Function Responsibilities:**
- Single entry point for CLI execution
- Handle nil options gracefully
- Maintain all existing functionality

#### 2. Update RunCmdOptions Structure
**File**: `cli.go`
**Changes**: Extend RunCmdOptions to include output writers

```go
// RunCmdOptions contains options for customizing RunCmd behavior
type RunCmdOptions struct {
    // MCPTransport allows providing a custom transport for MCP server (used for testing)
    MCPTransport *mcp.InMemoryTransport
    // Output writer for normal output (defaults to os.Stdout)
    Stdout io.Writer
    // Output writer for error output (defaults to os.Stderr)  
    Stderr io.Writer
}

// commandContext holds runtime context for command execution
type commandContext struct {
    stdout io.Writer
    stderr io.Writer
    manager TagManager
}
```

**Function Responsibilities:**
- Initialize default writers if not provided
- Pass writers through command context
- Maintain backward compatibility with nil options

#### 3. Update RunCmd Implementation
**File**: `cli.go`
**Changes**: Initialize writers and create command context

```go
func RunCmd(args []string, options *RunCmdOptions) error
```

**Function Responsibilities:**
- Create commandContext with appropriate writers
- Default to os.Stdout/os.Stderr if options is nil
- Pass context to command functions
- Pattern: Follow existing MCPTransport initialization pattern (cli.go:51-54)

#### 4. Update All Callers
**Files**: `cmd/tag-manager/main.go`, `cli_test.go`
**Changes**: Update all RunCmd calls to new signature

```go
// In main.go
err := tagmanager.RunCmd(os.Args, nil)

// In tests that don't need options
err := tagmanager.RunCmd(test.args, nil)

// In tests with options
options := &tagmanager.RunCmdOptions{...}
err := tagmanager.RunCmd(args, options)
```

**Testing Requirements:**
```go
func TestRunCmdWithWriterOptions(t *testing.T)
func TestRunCmdNilOptions(t *testing.T)
```

**Test Objectives:**
- Verify default writers are used when options is nil
- Verify custom writers are used when provided
- Ensure backward compatibility
- Confirm all existing tests still pass

**Context for implementation:**
- Existing RunCmd at cli.go:21-23
- RunCmdWithOptions at cli.go:25-94
- main.go call at line 11
- Test calls throughout cli_test.go (lines 80, 113, 289, 406)
- MCPTransport option handling at cli.go:51-54

**Validation Commands:**
```bash
go test -v -run TestRunCmd ./...
go build ./cmd/tag-manager
```

## Phase 2: Refactor Command Functions to Use Writers

### Overview
Update all command functions to accept commandContext and replace ALL direct stdout/stderr output with writes to the injected io.Writer instances.

### Changes Required:

#### 1. Update Command Function Signatures
**File**: `cli.go`
**Changes**: Add commandContext parameter to all command functions

```go
func findFilesCommand(ctx context.Context, cmdCtx *commandContext, args []string, verbose bool) error
func getTagInfoCommand(ctx context.Context, cmdCtx *commandContext, args []string, verbose bool) error
func listTagsCommand(ctx context.Context, cmdCtx *commandContext, args []string, verbose bool) error
func replaceTagCommand(ctx context.Context, cmdCtx *commandContext, args []string, globalDryRun bool, verbose bool) error
func updateCommand(ctx context.Context, cmdCtx *commandContext, args []string, globalDryRun bool, verbose bool) error
func untaggedFilesCommand(ctx context.Context, cmdCtx *commandContext, args []string, verbose bool) error
func validateTagsCommand(ctx context.Context, cmdCtx *commandContext, args []string, verbose bool) error
func getFileTagsCommand(ctx context.Context, cmdCtx *commandContext, args []string, verbose bool) error
func ShowHelp(w io.Writer) error
```

**Function Responsibilities:**
- Accept commandContext parameter
- Use cmdCtx.stdout for normal output
- Use cmdCtx.stderr for error output (if needed)
- NO direct writes to os.Stdout or os.Stderr

#### 2. Update ALL Output Statements
**File**: `cli.go`  
**Changes**: Replace every instance of direct output with writer-based calls

**ShowHelp (lines 96-134):**
- Line 132: `fmt.Print(help)` → `fmt.Fprint(w, help)`

**findFilesCommand (lines 136-188):**
- Line 177: `json.NewEncoder(os.Stdout).Encode(results)` → `json.NewEncoder(cmdCtx.stdout).Encode(results)`
- Line 181: `fmt.Printf("\n#%s (%d files):\n", tag, len(files))` → `fmt.Fprintf(cmdCtx.stdout, "\n#%s (%d files):\n", tag, len(files))`
- Line 183: `fmt.Printf("  %s\n", file)` → `fmt.Fprintf(cmdCtx.stdout, "  %s\n", file)`

**getTagInfoCommand (lines 190-236):**
- Line 221: `json.NewEncoder(os.Stdout).Encode(infos)` → `json.NewEncoder(cmdCtx.stdout).Encode(infos)`
- Line 225: `fmt.Printf("\n#%s:\n", info.Name)` → `fmt.Fprintf(cmdCtx.stdout, "\n#%s:\n", info.Name)`
- Line 226: `fmt.Printf("  Count: %d\n", info.Count)` → `fmt.Fprintf(cmdCtx.stdout, "  Count: %d\n", info.Count)`
- Line 228: `fmt.Printf("  Files:\n")` → `fmt.Fprintf(cmdCtx.stdout, "  Files:\n")`
- Line 230: `fmt.Printf("    %s\n", file)` → `fmt.Fprintf(cmdCtx.stdout, "    %s\n", file)`

**listTagsCommand (lines 238-281):**
- Line 272: `json.NewEncoder(os.Stdout).Encode(tags)` → `json.NewEncoder(cmdCtx.stdout).Encode(tags)`
- Line 275: `fmt.Printf("\nFound %d tags:\n", len(tags))` → `fmt.Fprintf(cmdCtx.stdout, "\nFound %d tags:\n", len(tags))`
- Line 277: `fmt.Printf("  #%-30s %d files\n", tag.Name, tag.Count)` → `fmt.Fprintf(cmdCtx.stdout, "  #%-30s %d files\n", tag.Name, tag.Count)`

**replaceTagCommand (lines 283-354):**
- Line 327: `fmt.Println("DRY RUN MODE - No files will be modified")` → `fmt.Fprintln(cmdCtx.stdout, "DRY RUN MODE - No files will be modified")`
- Line 336: `json.NewEncoder(os.Stdout).Encode(result)` → `json.NewEncoder(cmdCtx.stdout).Encode(result)`
- Line 339: `fmt.Printf("\nModified files: %d\n", len(result.ModifiedFiles))` → `fmt.Fprintf(cmdCtx.stdout, "\nModified files: %d\n", len(result.ModifiedFiles))`
- Line 342: `fmt.Printf("  %s\n", file)` → `fmt.Fprintf(cmdCtx.stdout, "  %s\n", file)`
- Line 347: `fmt.Printf("\nFailed files: %d\n", len(result.FailedFiles))` → `fmt.Fprintf(cmdCtx.stdout, "\nFailed files: %d\n", len(result.FailedFiles))`
- Line 349: `fmt.Printf("  %s: %s\n", file, result.Errors[i])` → `fmt.Fprintf(cmdCtx.stdout, "  %s: %s\n", file, result.Errors[i])`

**untaggedFilesCommand (lines 356-386):**
- Line 377: `json.NewEncoder(os.Stdout).Encode(files)` → `json.NewEncoder(cmdCtx.stdout).Encode(files)`
- Line 380: `fmt.Printf("\nFound %d untagged files:\n", len(files))` → `fmt.Fprintf(cmdCtx.stdout, "\nFound %d untagged files:\n", len(files))`
- Line 382: `fmt.Printf("  %s\n", file.Path)` → `fmt.Fprintf(cmdCtx.stdout, "  %s\n", file.Path)`

**validateTagsCommand (lines 388-427):**
- Line 409: `json.NewEncoder(os.Stdout).Encode(results)` → `json.NewEncoder(cmdCtx.stdout).Encode(results)`
- Line 414: `fmt.Printf("\n✓ %s: VALID\n", tag)` → `fmt.Fprintf(cmdCtx.stdout, "\n✓ %s: VALID\n", tag)`
- Line 416: `fmt.Printf("\n✗ %s: INVALID\n", tag)` → `fmt.Fprintf(cmdCtx.stdout, "\n✗ %s: INVALID\n", tag)`
- Line 418: `fmt.Printf("  Issue: %s\n", issue)` → `fmt.Fprintf(cmdCtx.stdout, "  Issue: %s\n", issue)`
- Line 421: `fmt.Printf("  → %s\n", suggestion)` → `fmt.Fprintf(cmdCtx.stdout, "  → %s\n", suggestion)`

**getFileTagsCommand (lines 429-468):**
- Line 453: `json.NewEncoder(os.Stdout).Encode(fileTags)` → `json.NewEncoder(cmdCtx.stdout).Encode(fileTags)`
- Line 457: `fmt.Printf("\n%s:\n", file.Path)` → `fmt.Fprintf(cmdCtx.stdout, "\n%s:\n", file.Path)`
- Line 459: `fmt.Printf("  (no tags)\n")` → `fmt.Fprintf(cmdCtx.stdout, "  (no tags)\n")`
- Line 462: `fmt.Printf("  #%s\n", tag)` → `fmt.Fprintf(cmdCtx.stdout, "  #%s\n", tag)`

**updateCommand (lines 470-551):**
- Line 502: `fmt.Println("DRY RUN MODE - No files will be modified")` → `fmt.Fprintln(cmdCtx.stdout, "DRY RUN MODE - No files will be modified")`
- Line 511: `json.NewEncoder(os.Stdout).Encode(result)` → `json.NewEncoder(cmdCtx.stdout).Encode(result)`
- Line 515: `fmt.Printf("Files with migrated hashtags: %d\n", len(result.FilesMigrated))` → `fmt.Fprintf(cmdCtx.stdout, "Files with migrated hashtags: %d\n", len(result.FilesMigrated))`
- Line 517: `fmt.Printf("  %s\n", file)` → `fmt.Fprintf(cmdCtx.stdout, "  %s\n", file)`
- Line 522: `fmt.Printf("Modified files: %d\n", len(result.ModifiedFiles))` → `fmt.Fprintf(cmdCtx.stdout, "Modified files: %d\n", len(result.ModifiedFiles))`
- Line 524: `fmt.Printf("  %s\n", file)` → `fmt.Fprintf(cmdCtx.stdout, "  %s\n", file)`
- Line 529: `fmt.Println("Tags added:")` → `fmt.Fprintln(cmdCtx.stdout, "Tags added:")`
- Line 531: `fmt.Printf("  %s: %d files\n", tag, count)` → `fmt.Fprintf(cmdCtx.stdout, "  %s: %d files\n", tag, count)`
- Line 536: `fmt.Println("Tags removed:")` → `fmt.Fprintln(cmdCtx.stdout, "Tags removed:")`
- Line 538: `fmt.Printf("  %s: %d files\n", tag, count)` → `fmt.Fprintf(cmdCtx.stdout, "  %s: %d files\n", tag, count)`
- Line 543: `fmt.Printf("Errors: %d\n", len(result.Errors))` → `fmt.Fprintf(cmdCtx.stdout, "Errors: %d\n", len(result.Errors))`
- Line 545: `fmt.Printf("  %s\n", errMsg)` → `fmt.Fprintf(cmdCtx.stdout, "  %s\n", errMsg)`

**Testing Requirements:**
```go
func TestCommandsWithCustomWriters(t *testing.T)
func TestAllOutputCaptured(t *testing.T)
```

**Test Objectives:**
- Verify each command writes to injected writer
- Test both JSON and text output modes
- Validate NO output goes to os.Stdout/os.Stderr directly
- Confirm all output is captured in tests

**Context for implementation:**
- Total of 38 direct output statements to update
- 8 JSON encoder creations
- 30 fmt.Print* statements
- ShowHelp function needs writer parameter

**Validation Commands:**
```bash
go test -v -run TestCommandsWithCustomWriters ./...
# Verify no direct stdout usage remains:
grep -n "fmt.Print\|os.Stdout\|os.Stderr" cli.go | grep -v "//\|RunCmdOptions"
```

## Phase 3: Create Output Capture Test Utilities

### Overview
Build test utilities to simplify output capture and validation in tests.

### Changes Required:

#### 1. Create Test Helper Functions
**File**: `cli_test.go`
**Changes**: Add output capture utilities

```go
// captureOutput executes a command and captures its output
func captureOutput(t *testing.T, args []string) (stdout string, stderr string, err error)

// captureJSONOutput executes a command and unmarshals JSON output
func captureJSONOutput(t *testing.T, args []string, v interface{}) error

// assertOutputContains verifies output contains expected strings
func assertOutputContains(t *testing.T, output string, expected []string)

// assertJSONOutput verifies JSON output structure
func assertJSONOutput(t *testing.T, args []string, validate func(t *testing.T, data interface{}))
```

**Function Responsibilities:**
- Create bytes.Buffer for output capture
- Execute command with custom writers
- Return captured output for validation
- Provide convenient assertion helpers

**Testing Requirements:**
```go
func TestOutputCaptureUtilities(t *testing.T)
```

**Test Objectives:**
- Verify capture utilities work correctly
- Test JSON unmarshaling functionality
- Validate assertion helpers

**Context for implementation:**
- Existing test patterns in cli_test.go
- bytes.Buffer usage for io.Writer capture
- Standard testing assertion patterns

**Validation Commands:**
```bash
go test -v -run TestOutputCaptureUtilities ./...
```

## Phase 4: Update Existing Tests to Validate Output

### Overview
Enhance existing tests to validate command output messages and error formatting.

### Changes Required:

#### 1. Update CLI Integration Tests
**File**: `cli_test.go`
**Changes**: Add output validation to existing tests

```go
func TestCLIIntegration(t *testing.T)
func TestUpdateCommand(t *testing.T)
```

**Function Responsibilities:**
- Capture command output using new utilities
- Validate expected messages appear in output
- Check error message formatting
- Verify JSON structure when --json flag used

#### 2. Add Output-Specific Test Cases
**File**: `cli_test.go`
**Changes**: Create focused output validation tests

```go
func TestCommandOutputMessages(t *testing.T)
func TestErrorMessageFormatting(t *testing.T)
func TestJSONOutputStructure(t *testing.T)
func TestDryRunOutputMessages(t *testing.T)
```

**Testing Requirements:**
- Test each command's text output format
- Validate error message consistency
- Verify JSON output structure
- Check dry-run mode messages

**Test Objectives:**
- Ensure output messages match expected format
- Validate error reporting consistency
- Verify JSON output completeness
- Check verbose mode output

**Context for implementation:**
- Existing test structure in cli_test.go:15-88
- Current test patterns for error validation
- JSON output testing patterns

**Validation Commands:**
```bash
go test -v -run TestCommandOutput ./...
go test -v -run TestErrorMessage ./...
```

## Phase 5: Refactor manager_test.go to Use CLI

### Overview
Update manager_test.go tests to use RunCmd() instead of calling UpdateTags() directly, ensuring full integration testing through the CLI layer.

### Changes Required:

#### 1. Convert Direct UpdateTags Calls
**File**: `manager_test.go`
**Changes**: Replace direct manager calls with CLI invocations

```go
func TestUpdateTags(t *testing.T)
func TestYAMLFrontMatterParsing(t *testing.T)
func TestFrontMatterFieldPreservation(t *testing.T)
func TestTagConflictResolution(t *testing.T)
func TestDuplicateTagHandling(t *testing.T)
func TestRemoveTagsFromBody(t *testing.T)
func TestTopOfFileDetection(t *testing.T)
func TestHashtagMigration(t *testing.T)
func TestMigrationBoundaryDetection(t *testing.T)
func TestMigrationWithExistingFrontmatter(t *testing.T)
```

**Function Responsibilities:**
- Build appropriate CLI arguments for each test case
- Use captureOutput or captureJSONOutput utilities
- Validate both operation results and output messages
- Maintain existing test coverage and assertions

#### 2. Add CLI-Specific Test Coverage
**File**: `manager_test.go`
**Changes**: Ensure CLI-specific behaviors are tested

```go
func TestUpdateTagsCLIErrors(t *testing.T)
func TestUpdateTagsCLIOutput(t *testing.T)
```

**Testing Requirements:**
- Test parameter validation through CLI
- Verify error message formatting
- Check output consistency across commands

**Test Objectives:**
- Maintain 100% coverage of UpdateTags functionality
- Add validation of CLI output messages
- Ensure error reporting through CLI layer
- Verify dry-run mode behavior

**Context for implementation:**
- Current UpdateTags test cases at lines 248-703
- Existing test data setup patterns
- File system test utilities usage

**Validation Commands:**
```bash
go test -v -run TestUpdateTags ./...
go test -v -run TestMigration ./...
go test -race -v ./...
```

## Phase 6: Documentation and Cleanup

### Overview
Update documentation and ensure code quality standards are met.

### Changes Required:

#### 1. Update CLAUDE.md Documentation
**File**: `CLAUDE.md`
**Changes**: Document new testing patterns

```markdown
### Testing with Output Capture

Tests can now capture CLI output for validation:
- Use RunCmdOptions with custom io.Writer
- Capture both stdout and stderr
- Validate text and JSON output formats
```

#### 2. Add Test Examples
**File**: `cli_test.go`
**Changes**: Add example test showing output capture pattern

```go
func ExampleOutputCapture(t *testing.T)
```

**Function Responsibilities:**
- Demonstrate output capture pattern
- Show JSON validation approach
- Document best practices

**Testing Requirements:**
```go
func TestExamplePatterns(t *testing.T)
```

**Test Objectives:**
- Ensure examples work correctly
- Validate documentation accuracy

**Context for implementation:**
- Existing CLAUDE.md structure
- Current testing patterns documentation
- Go documentation conventions

**Validation Commands:**
```bash
go test -v ./...
golangci-lint run
go test -race -v ./...
```