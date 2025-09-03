package tagmanager

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RunCmdOptions contains options for customizing RunCmd behavior
type RunCmdOptions struct {
	// MCPTransport allows providing a custom transport for MCP server (used for testing)
	MCPTransport *mcp.InMemoryTransport
}

func RunCmd(args []string) error {
	return RunCmdWithOptions(args, nil)
}

func RunCmdWithOptions(args []string, options *RunCmdOptions) error {
	if len(args) < 1 {
		return ShowHelp()
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
		return ShowHelp()
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
		return ShowHelp()
	}

	config, err := LoadConfig(*configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	ctx := context.Background()
	manager, err := NewDefaultTagManager(config)
	if err != nil {
		return fmt.Errorf("failed to create tag manager: %w", err)
	}

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

func ShowHelp() error {
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
  untagged     Find files without any tags
  validate     Validate tag syntax and suggest fixes
  file-tags    Get tags for specific files

Examples:
  tag-manager find --tags="#golang,#python" --root="/path/to/vault"
  tag-manager list --root="/path/to/vault" --min-count=2
  tag-manager replace --old="#old-tag" --new="#new-tag" --root="/path/to/vault" --dry-run
  tag-manager untagged --root="/path/to/vault"
  tag-manager validate --tags="#test,#invalid-tag!"
  tag-manager file-tags --files="/path/file1.md,/path/file2.md"
  tag-manager -mcp --config="/path/to/config.yaml"

For more information, visit: https://github.com/thrawn01/tag-manager
`
	fmt.Print(help)
	return nil
}

func findFilesCommand(ctx context.Context, manager TagManager, args []string, verbose bool) error {
	fs := flag.NewFlagSet("find", flag.ContinueOnError)

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	tags := fs.String("tags", "", "Comma-separated list of tags to search for")
	root := fs.String("root", cwd, "Root directory to search")
	maxResults := fs.Int("max-results", 100, "Maximum files per tag")
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

	results, err := manager.FindFilesByTags(ctx, tagList, *root)
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
		return json.NewEncoder(os.Stdout).Encode(results)
	}

	for tag, files := range results {
		fmt.Printf("\n#%s (%d files):\n", tag, len(files))
		for _, file := range files {
			fmt.Printf("  %s\n", file)
		}
	}

	return nil
}

func getTagInfoCommand(ctx context.Context, manager TagManager, args []string, verbose bool) error {
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

	infos, err := manager.GetTagsInfo(ctx, tagList, *root)
	if err != nil {
		return err
	}

	if *jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(infos)
	}

	for _, info := range infos {
		fmt.Printf("\n#%s:\n", info.Name)
		fmt.Printf("  Count: %d\n", info.Count)
		if verbose {
			fmt.Printf("  Files:\n")
			for _, file := range info.Files {
				fmt.Printf("    %s\n", file)
			}
		}
	}

	return nil
}

func listTagsCommand(ctx context.Context, manager TagManager, args []string, verbose bool) error {
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

	tags, err := manager.ListAllTags(ctx, *root, *minCount)
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
		return json.NewEncoder(os.Stdout).Encode(tags)
	}

	fmt.Printf("\nFound %d tags:\n", len(tags))
	for _, tag := range tags {
		fmt.Printf("  #%-30s %d files\n", tag.Name, tag.Count)
	}

	return nil
}

func replaceTagCommand(ctx context.Context, manager TagManager, args []string, globalDryRun bool, verbose bool) error {
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
		fmt.Println("DRY RUN MODE - No files will be modified")
	}

	result, err := manager.ReplaceTagsBatch(ctx, replaceList, *root, dryRun)
	if err != nil {
		return err
	}

	if *jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(result)
	}

	fmt.Printf("\nModified files: %d\n", len(result.ModifiedFiles))
	if verbose {
		for _, file := range result.ModifiedFiles {
			fmt.Printf("  %s\n", file)
		}
	}

	if len(result.FailedFiles) > 0 {
		fmt.Printf("\nFailed files: %d\n", len(result.FailedFiles))
		for i, file := range result.FailedFiles {
			fmt.Printf("  %s: %s\n", file, result.Errors[i])
		}
	}

	return nil
}

func untaggedFilesCommand(ctx context.Context, manager TagManager, args []string, verbose bool) error {
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

	files, err := manager.GetUntaggedFiles(ctx, *root)
	if err != nil {
		return err
	}

	if *jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(files)
	}

	fmt.Printf("\nFound %d untagged files:\n", len(files))
	for _, file := range files {
		fmt.Printf("  %s\n", file.Path)
	}

	return nil
}

func validateTagsCommand(ctx context.Context, manager TagManager, args []string, verbose bool) error {
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

	results := manager.ValidateTags(ctx, tagList)

	if *jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(results)
	}

	for tag, result := range results {
		if result.IsValid {
			fmt.Printf("\n✓ %s: VALID\n", tag)
		} else {
			fmt.Printf("\n✗ %s: INVALID\n", tag)
			for _, issue := range result.Issues {
				fmt.Printf("  Issue: %s\n", issue)
			}
			for _, suggestion := range result.Suggestions {
				fmt.Printf("  → %s\n", suggestion)
			}
		}
	}

	return nil
}

func getFileTagsCommand(ctx context.Context, manager TagManager, args []string, verbose bool) error {
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

	fileTags, err := manager.GetFilesTags(ctx, fileList)
	if err != nil {
		return err
	}

	if *jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(fileTags)
	}

	for _, file := range fileTags {
		fmt.Printf("\n%s:\n", file.Path)
		if len(file.Tags) == 0 {
			fmt.Printf("  (no tags)\n")
		} else {
			for _, tag := range file.Tags {
				fmt.Printf("  #%s\n", tag)
			}
		}
	}

	return nil
}
