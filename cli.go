package tagmanager

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RunCmdOptions contains options for customizing RunCmd behavior
type RunCmdOptions struct {
	// MCPTransport allows providing a custom transport for MCP server (used for testing)
	MCPTransport *mcp.InMemoryTransport
	// Stdout writer for normal output (defaults to os.Stdout)
	Stdout io.Writer
	// Stderr writer for error output (defaults to os.Stderr)
	Stderr io.Writer
}

// commandContext holds runtime context for command execution
type commandContext struct {
	stdout  io.Writer
	stderr  io.Writer
	manager TagManager
}

func RunCmd(args []string, options *RunCmdOptions) error {
	if len(args) < 1 {
		stdout := io.Writer(os.Stdout)
		if options != nil && options.Stdout != nil {
			stdout = options.Stdout
		}
		return ShowHelp(stdout)
	}

	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)

	var (
		help       = fs.Bool("h", false, "Show help")
		mcpOption  = fs.Bool("mcp", false, "Run as MCP server")
		verbose    = fs.Bool("v", false, "Verbose output")
		dryRun     = fs.Bool("dry-run", false, "Show what would be changed without making changes")
		configFile = fs.String("config", "", "Path to configuration file")
	)

	if len(args) > 1 {
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
	}

	if *help {
		stdout := io.Writer(os.Stdout)
		if options != nil && options.Stdout != nil {
			stdout = options.Stdout
		}
		return ShowHelp(stdout)
	}

	if *mcpOption {
		var transport *mcp.InMemoryTransport
		if options != nil && options.MCPTransport != nil {
			transport = options.MCPTransport
		}
		return RunMCPServer(*configFile, transport)
	}

	remaining := fs.Args()
	if len(remaining) == 0 {
		stdout := io.Writer(os.Stdout)
		if options != nil && options.Stdout != nil {
			stdout = options.Stdout
		}
		return ShowHelp(stdout)
	}

	config, err := LoadConfig(*configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize command context with writers
	cmdCtx := &commandContext{
		stdout: io.Writer(os.Stdout),
		stderr: io.Writer(os.Stderr),
	}

	if options != nil {
		if options.Stdout != nil {
			cmdCtx.stdout = options.Stdout
		}
		if options.Stderr != nil {
			cmdCtx.stderr = options.Stderr
		}
	}

	ctx := context.Background()
	manager, err := NewDefaultTagManager(config)
	if err != nil {
		return fmt.Errorf("failed to create tag manager: %w", err)
	}
	cmdCtx.manager = manager

	switch remaining[0] {
	case "find":
		return findFilesCommand(ctx, cmdCtx, remaining[1:], *verbose)
	case "info":
		return getTagInfoCommand(ctx, cmdCtx, remaining[1:], *verbose)
	case "list":
		return listTagsCommand(ctx, cmdCtx, remaining[1:], *verbose)
	case "replace":
		return replaceTagCommand(ctx, cmdCtx, remaining[1:], *dryRun, *verbose)
	case "update":
		return updateCommand(ctx, cmdCtx, remaining[1:], *dryRun, *verbose)
	case "untagged":
		return untaggedFilesCommand(ctx, cmdCtx, remaining[1:], *verbose)
	case "validate":
		return validateTagsCommand(ctx, cmdCtx, remaining[1:], *verbose)
	case "file-tags":
		return getFileTagsCommand(ctx, cmdCtx, remaining[1:], *verbose)
	default:
		return fmt.Errorf("unknown command: %s", remaining[0])
	}
}

func ShowHelp(w io.Writer) error {
	help := `Obsidian Tag Manager - Manage tags in Obsidian vaults

Usage:
  tag-manager [OPTIONS] COMMAND [ARGS...]
  tag-manager -mcp              Run as MCP server

Options:
  -h, --help           Show this help message
  -v, --verbose        Enable verbose output
  --dry-run            Preview changes without modifying files
  --config FILE        Path to configuration file
  -mcp                 Run as MCP server

Commands:
  find         Find files containing specific tags
  info         Get detailed information about tags
  list         List all tags with usage statistics
  replace      Replace/rename tags across files
  update       Add or remove tags from specific files
  untagged     Find files without any tags
  validate     Validate tag syntax and suggest fixes
  file-tags    Get tags for specific files

Examples:
  tag-manager find --tags="#golang,#python" --root="/path/to/vault"
  tag-manager list --root="/path/to/vault" --min-count=2
  tag-manager replace --old="#old-tag" --new="#new-tag" --root="/path/to/vault" --dry-run
  tag-manager update --add="golang,python" --remove="old-tag" --root="/path/to/vault" --files="file1.md,file2.md" --dry-run
  tag-manager untagged --root="/path/to/vault"
  tag-manager validate --tags="#test,#invalid-tag!"
  tag-manager file-tags --files="/path/file1.md,/path/file2.md"
  tag-manager -mcp --config="/path/to/config.yaml"

For more information, visit: https://github.com/thrawn01/tag-manager
`
	_, _ = fmt.Fprint(w, help)
	return nil
}

func findFilesCommand(ctx context.Context, cmdCtx *commandContext, args []string, verbose bool) error {
	fs := flag.NewFlagSet("find", flag.ContinueOnError)

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	const defaultMaxResults = 100

	tags := fs.String("tags", "", "Comma-separated list of tags to search for")
	root := fs.String("root", cwd, "Root directory to search")
	maxResults := fs.Int("max-results", defaultMaxResults, "Maximum files per tag")
	jsonOutput := fs.Bool("json", false, "Output as JSON")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *tags == "" {
		return fmt.Errorf("--tags is required")
	}

	tagList := strings.Split(*tags, ",")
	for i := range tagList {
		tagList[i] = strings.TrimSpace(tagList[i])
	}

	results, err := cmdCtx.manager.FindFilesByTags(ctx, tagList, *root)
	if err != nil {
		return err
	}

	for tag, files := range results {
		if len(files) > *maxResults {
			files = files[:*maxResults]
			results[tag] = files
		}
	}

	if *jsonOutput {
		return json.NewEncoder(cmdCtx.stdout).Encode(results)
	}

	for tag, files := range results {
		_, _ = fmt.Fprintf(cmdCtx.stdout, "\n#%s (%d files):\n", tag, len(files))
		for _, file := range files {
			_, _ = fmt.Fprintf(cmdCtx.stdout, "  %s\n", file)
		}
	}

	return nil
}

func getTagInfoCommand(ctx context.Context, cmdCtx *commandContext, args []string, verbose bool) error {
	fs := flag.NewFlagSet("info", flag.ContinueOnError)

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	tags := fs.String("tags", "", "Comma-separated list of tags")
	root := fs.String("root", cwd, "Root directory to search")
	jsonOutput := fs.Bool("json", false, "Output as JSON")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *tags == "" {
		return fmt.Errorf("--tags is required")
	}

	tagList := strings.Split(*tags, ",")
	for i := range tagList {
		tagList[i] = strings.TrimSpace(tagList[i])
	}

	infos, err := cmdCtx.manager.GetTagsInfo(ctx, tagList, *root)
	if err != nil {
		return err
	}

	if *jsonOutput {
		return json.NewEncoder(cmdCtx.stdout).Encode(infos)
	}

	for _, info := range infos {
		_, _ = fmt.Fprintf(cmdCtx.stdout, "\n#%s:\n", info.Name)
		_, _ = fmt.Fprintf(cmdCtx.stdout, "  Count: %d\n", info.Count)
		if verbose {
			_, _ = fmt.Fprintf(cmdCtx.stdout, "  Files:\n")
			for _, file := range info.Files {
				_, _ = fmt.Fprintf(cmdCtx.stdout, "    %s\n", file)
			}
		}
	}

	return nil
}

func listTagsCommand(ctx context.Context, cmdCtx *commandContext, args []string, verbose bool) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	root := fs.String("root", cwd, "Root directory to search")
	minCount := fs.Int("min-count", 1, "Minimum usage count")
	pattern := fs.String("pattern", "", "Optional regex pattern to filter tags")
	jsonOutput := fs.Bool("json", false, "Output as JSON")

	if err := fs.Parse(args); err != nil {
		return err
	}

	tags, err := cmdCtx.manager.ListAllTags(ctx, *root, *minCount)
	if err != nil {
		return err
	}

	if *pattern != "" {
		// Filter tags by pattern
		var filtered []TagInfo
		for _, tag := range tags {
			if strings.Contains(tag.Name, *pattern) {
				filtered = append(filtered, tag)
			}
		}
		tags = filtered
	}

	if *jsonOutput {
		return json.NewEncoder(cmdCtx.stdout).Encode(tags)
	}

	_, _ = fmt.Fprintf(cmdCtx.stdout, "\nFound %d tags:\n", len(tags))
	for _, tag := range tags {
		_, _ = fmt.Fprintf(cmdCtx.stdout, "  #%-30s %d files\n", tag.Name, tag.Count)
	}

	return nil
}

func replaceTagCommand(ctx context.Context, cmdCtx *commandContext, args []string, globalDryRun bool, verbose bool) error {
	fs := flag.NewFlagSet("replace", flag.ContinueOnError)

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	replacements := fs.String("replacements", "", "Comma-separated replacements (old1:new1,old2:new2)")
	old := fs.String("old", "", "Old tag to replace")
	new := fs.String("new", "", "New tag name")
	root := fs.String("root", cwd, "Root directory to search")
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	localDryRun := fs.Bool("dry-run", false, "Show what would be changed without making changes")

	if err := fs.Parse(args); err != nil {
		return err
	}

	var replaceList []TagReplacement

	if *replacements != "" {
		pairs := strings.Split(*replacements, ",")
		for _, pair := range pairs {
			parts := strings.Split(pair, ":")
			if len(parts) != 2 {
				return fmt.Errorf("invalid replacement format: %s", pair)
			}
			replaceList = append(replaceList, TagReplacement{
				OldTag: strings.TrimSpace(parts[0]),
				NewTag: strings.TrimSpace(parts[1]),
			})
		}
	} else if *old != "" && *new != "" {
		replaceList = append(replaceList, TagReplacement{
			OldTag: *old,
			NewTag: *new,
		})
	} else {
		return fmt.Errorf("either --replacements or both --old and --new are required")
	}

	dryRun := globalDryRun || *localDryRun
	if dryRun {
		_, _ = fmt.Fprintln(cmdCtx.stdout, "DRY RUN MODE - No files will be modified")
	}

	result, err := cmdCtx.manager.ReplaceTagsBatch(ctx, replaceList, *root, dryRun)
	if err != nil {
		return err
	}

	if *jsonOutput {
		return json.NewEncoder(cmdCtx.stdout).Encode(result)
	}

	_, _ = fmt.Fprintf(cmdCtx.stdout, "\nModified files: %d\n", len(result.ModifiedFiles))
	if verbose {
		for _, file := range result.ModifiedFiles {
			_, _ = fmt.Fprintf(cmdCtx.stdout, "  %s\n", file)
		}
	}

	if len(result.FailedFiles) > 0 {
		_, _ = fmt.Fprintf(cmdCtx.stdout, "\nFailed files: %d\n", len(result.FailedFiles))
		for i, file := range result.FailedFiles {
			_, _ = fmt.Fprintf(cmdCtx.stdout, "  %s: %s\n", file, result.Errors[i])
		}
	}

	return nil
}

func untaggedFilesCommand(ctx context.Context, cmdCtx *commandContext, args []string, verbose bool) error {
	fs := flag.NewFlagSet("untagged", flag.ContinueOnError)

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	root := fs.String("root", cwd, "Root directory to search")
	jsonOutput := fs.Bool("json", false, "Output as JSON")

	if err := fs.Parse(args); err != nil {
		return err
	}

	files, err := cmdCtx.manager.GetUntaggedFiles(ctx, *root)
	if err != nil {
		return err
	}

	if *jsonOutput {
		return json.NewEncoder(cmdCtx.stdout).Encode(files)
	}

	_, _ = fmt.Fprintf(cmdCtx.stdout, "\nFound %d untagged files:\n", len(files))
	for _, file := range files {
		_, _ = fmt.Fprintf(cmdCtx.stdout, "  %s\n", file.Path)
	}

	return nil
}

func validateTagsCommand(ctx context.Context, cmdCtx *commandContext, args []string, verbose bool) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	tags := fs.String("tags", "", "Comma-separated list of tags to validate")
	jsonOutput := fs.Bool("json", false, "Output as JSON")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *tags == "" {
		return fmt.Errorf("--tags is required")
	}

	tagList := strings.Split(*tags, ",")
	for i := range tagList {
		tagList[i] = strings.TrimSpace(tagList[i])
	}

	results := cmdCtx.manager.ValidateTags(ctx, tagList)

	if *jsonOutput {
		return json.NewEncoder(cmdCtx.stdout).Encode(results)
	}

	for tag, result := range results {
		if result.IsValid {
			_, _ = fmt.Fprintf(cmdCtx.stdout, "\n✓ %s: VALID\n", tag)
		} else {
			_, _ = fmt.Fprintf(cmdCtx.stdout, "\n✗ %s: INVALID\n", tag)
			for _, issue := range result.Issues {
				_, _ = fmt.Fprintf(cmdCtx.stdout, "  Issue: %s\n", issue)
			}
			for _, suggestion := range result.Suggestions {
				_, _ = fmt.Fprintf(cmdCtx.stdout, "  → %s\n", suggestion)
			}
		}
	}

	return nil
}

func getFileTagsCommand(ctx context.Context, cmdCtx *commandContext, args []string, verbose bool) error {
	fs := flag.NewFlagSet("file-tags", flag.ContinueOnError)
	files := fs.String("files", "", "Comma-separated list of file paths")
	jsonOutput := fs.Bool("json", false, "Output as JSON")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *files == "" {
		return fmt.Errorf("--files is required")
	}

	fileList := strings.Split(*files, ",")
	for i := range fileList {
		fileList[i] = strings.TrimSpace(fileList[i])
	}

	fileTags, err := cmdCtx.manager.GetFilesTags(ctx, fileList)
	if err != nil {
		return err
	}

	if *jsonOutput {
		return json.NewEncoder(cmdCtx.stdout).Encode(fileTags)
	}

	for _, file := range fileTags {
		_, _ = fmt.Fprintf(cmdCtx.stdout, "\n%s:\n", file.Path)
		if len(file.Tags) == 0 {
			_, _ = fmt.Fprintf(cmdCtx.stdout, "  (no tags)\n")
		} else {
			for _, tag := range file.Tags {
				_, _ = fmt.Fprintf(cmdCtx.stdout, "  #%s\n", tag)
			}
		}
	}

	return nil
}

func updateCommand(ctx context.Context, cmdCtx *commandContext, args []string, globalDryRun bool, verbose bool) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	addTags := fs.String("add", "", "Comma-separated tags to add")
	removeTags := fs.String("remove", "", "Comma-separated tags to remove")
	files := fs.String("files", "", "Comma-separated file paths relative to root")
	root := fs.String("root", cwd, "Root directory for file paths")
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	localDryRun := fs.Bool("dry-run", false, "Show what would be changed without making changes")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := ValidateUpdateParameters(*addTags, *removeTags, *files); err != nil {
		return err
	}

	addTagList := parseTagList(*addTags)
	removeTagList := parseTagList(*removeTags)
	filePaths, err := ParseFilePaths(*files, *root)
	if err != nil {
		return err
	}

	dryRun := globalDryRun || *localDryRun
	if dryRun {
		_, _ = fmt.Fprintln(cmdCtx.stdout, "DRY RUN MODE - No files will be modified")
	}

	result, err := cmdCtx.manager.UpdateTags(ctx, addTagList, removeTagList, *root, filePaths, dryRun)
	if err != nil {
		return fmt.Errorf("failed to update tags: %w", err)
	}

	if *jsonOutput {
		return json.NewEncoder(cmdCtx.stdout).Encode(result)
	}

	if len(result.FilesMigrated) > 0 {
		_, _ = fmt.Fprintf(cmdCtx.stdout, "Files with migrated hashtags: %d\n", len(result.FilesMigrated))
		for _, file := range result.FilesMigrated {
			_, _ = fmt.Fprintf(cmdCtx.stdout, "  %s\n", file)
		}
	}

	if len(result.ModifiedFiles) > 0 {
		_, _ = fmt.Fprintf(cmdCtx.stdout, "Modified files: %d\n", len(result.ModifiedFiles))
		for _, file := range result.ModifiedFiles {
			_, _ = fmt.Fprintf(cmdCtx.stdout, "  %s\n", file)
		}
	}

	if len(result.TagsAdded) > 0 {
		_, _ = fmt.Fprintln(cmdCtx.stdout, "Tags added:")
		for tag, count := range result.TagsAdded {
			_, _ = fmt.Fprintf(cmdCtx.stdout, "  %s: %d files\n", tag, count)
		}
	}

	if len(result.TagsRemoved) > 0 {
		_, _ = fmt.Fprintln(cmdCtx.stdout, "Tags removed:")
		for tag, count := range result.TagsRemoved {
			_, _ = fmt.Fprintf(cmdCtx.stdout, "  %s: %d files\n", tag, count)
		}
	}

	if len(result.Errors) > 0 {
		_, _ = fmt.Fprintf(cmdCtx.stdout, "Errors: %d\n", len(result.Errors))
		for _, errMsg := range result.Errors {
			_, _ = fmt.Fprintf(cmdCtx.stdout, "  %s\n", errMsg)
		}
		return fmt.Errorf("completed with %d errors", len(result.Errors))
	}

	return nil
}

func ValidateUpdateParameters(addTags, removeTags, files string) error {
	if addTags == "" && removeTags == "" {
		return fmt.Errorf("at least one of --add or --remove must be specified")
	}
	if files == "" {
		return fmt.Errorf("--files parameter is required")
	}
	return nil
}

func parseTagList(tagStr string) []string {
	if tagStr == "" {
		return nil
	}
	parts := strings.Split(tagStr, ",")
	var tags []string
	for _, part := range parts {
		tag := strings.TrimSpace(part)
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

func ParseFilePaths(filesStr, root string) ([]string, error) {
	if filesStr == "" {
		return nil, fmt.Errorf("files parameter cannot be empty")
	}

	parts := strings.Split(filesStr, ",")
	var filePaths []string
	for _, part := range parts {
		path := strings.TrimSpace(part)
		if path != "" {
			if filepath.IsAbs(path) {
				return nil, fmt.Errorf("file path must be relative to root: %s", path)
			}
			filePaths = append(filePaths, path)
		}
	}

	if len(filePaths) == 0 {
		return nil, fmt.Errorf("no valid file paths provided")
	}

	return filePaths, nil
}
