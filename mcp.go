package tagmanager

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Parameter structures for MCP tools
type FindFilesByTagsParams struct {
	Tags       []string `json:"tags"`
	Root       string   `json:"root"`
	MaxResults *int     `json:"max_results,omitempty"`
}

type GetTagsInfoParams struct {
	Tags           []string `json:"tags"`
	Root           string   `json:"root"`
	MaxFilesPerTag *int     `json:"max_files_per_tag,omitempty"`
}

type ListAllTagsParams struct {
	Root       string `json:"root"`
	MinCount   int    `json:"min_count"`
	Pattern    string `json:"pattern,omitempty"`
	MaxResults *int   `json:"max_results,omitempty"`
}

type ReplaceTagsBatchParams struct {
	Replacements []TagReplacement `json:"replacements"`
	Root         string           `json:"root"`
	DryRun       bool             `json:"dry_run"`
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

// Tool handler functions
func FindFilesByTagsTool(ctx context.Context, req *mcp.CallToolRequest, args FindFilesByTagsParams, manager TagManager) (*mcp.CallToolResult, any, error) {
	result, err := manager.FindFilesByTags(ctx, args.Tags, args.Root)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find files by tags: %w", err)
	}

	if args.MaxResults != nil {
		result = limitFilesByTagsResults(result, *args.MaxResults)
	}

	return nil, result, nil
}

func GetTagsInfoTool(ctx context.Context, req *mcp.CallToolRequest, args GetTagsInfoParams, manager TagManager) (*mcp.CallToolResult, any, error) {
	result, err := manager.GetTagsInfo(ctx, args.Tags, args.Root)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get tags info: %w", err)
	}

	if args.MaxFilesPerTag != nil {
		result = limitTagInfoFiles(result, *args.MaxFilesPerTag)
	}

	return nil, result, nil
}

func ListAllTagsTool(ctx context.Context, req *mcp.CallToolRequest, args ListAllTagsParams, manager TagManager) (*mcp.CallToolResult, any, error) {
	result, err := manager.ListAllTags(ctx, args.Root, args.MinCount)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list tags: %w", err)
	}

	if args.Pattern != "" {
		pattern, err := regexp.Compile(args.Pattern)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid pattern: %w", err)
		}
		result = filterTagsByPattern(result, pattern)
	}

	if args.MaxResults != nil && len(result) > *args.MaxResults {
		result = result[:*args.MaxResults]
	}

	return nil, result, nil
}

func ReplaceTagsBatchTool(ctx context.Context, req *mcp.CallToolRequest, args ReplaceTagsBatchParams, manager TagManager) (*mcp.CallToolResult, any, error) {
	result, err := manager.ReplaceTagsBatch(ctx, args.Replacements, args.Root, args.DryRun)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to replace tags: %w", err)
	}

	return nil, result, nil
}

func GetUntaggedFilesTool(ctx context.Context, req *mcp.CallToolRequest, args GetUntaggedFilesParams, manager TagManager) (*mcp.CallToolResult, any, error) {
	result, err := manager.GetUntaggedFiles(ctx, args.Root)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get untagged files: %w", err)
	}

	if args.MaxResults != nil && len(result) > *args.MaxResults {
		result = result[:*args.MaxResults]
	}

	return nil, result, nil
}

func ValidateTagsTool(ctx context.Context, req *mcp.CallToolRequest, args ValidateTagsParams, manager TagManager) (*mcp.CallToolResult, any, error) {
	result := manager.ValidateTags(ctx, args.Tags)
	return nil, result, nil
}

func GetFilesTagsTool(ctx context.Context, req *mcp.CallToolRequest, args GetFilesTagsParams, manager TagManager) (*mcp.CallToolResult, any, error) {
	filePaths := args.FilePaths
	if args.MaxFiles != nil && len(filePaths) > *args.MaxFiles {
		filePaths = filePaths[:*args.MaxFiles]
	}

	result, err := manager.GetFilesTags(ctx, filePaths)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get files tags: %w", err)
	}

	return nil, result, nil
}

// Helper functions for result limiting
func limitFilesByTagsResults(result map[string][]string, maxResults int) map[string][]string {
	limited := make(map[string][]string)
	count := 0
	for tag, files := range result {
		if count >= maxResults {
			break
		}
		limited[tag] = files
		count++
	}
	return limited
}

func limitTagInfoFiles(tagInfos []TagInfo, maxFilesPerTag int) []TagInfo {
	limited := make([]TagInfo, len(tagInfos))
	for i, tagInfo := range tagInfos {
		limited[i] = tagInfo
		if len(tagInfo.Files) > maxFilesPerTag {
			limited[i].Files = tagInfo.Files[:maxFilesPerTag]
		}
	}
	return limited
}

func filterTagsByPattern(tagInfos []TagInfo, pattern *regexp.Regexp) []TagInfo {
	var filtered []TagInfo
	for _, tagInfo := range tagInfos {
		if pattern.MatchString(tagInfo.Name) {
			filtered = append(filtered, tagInfo)
		}
	}
	return filtered
}

// RunMCPServer starts the MCP server implementation using the official Go SDK
// If transport is nil, it will use stdio transport
func RunMCPServer(configPath string, transport *mcp.InMemoryTransport) error {
	config, err := LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	manager, err := NewDefaultTagManager(config)
	if err != nil {
		return fmt.Errorf("failed to create tag manager: %w", err)
	}

	// Create MCP server
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "tag-manager",
		Version: "1.0.0",
	}, nil)

	// Register all MCP tools
	mcp.AddTool(server, &mcp.Tool{
		Name:        "find_files_by_tags",
		Description: "Find files containing specific tags",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args FindFilesByTagsParams) (*mcp.CallToolResult, any, error) {
		return FindFilesByTagsTool(ctx, req, args, manager)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_tags_info",
		Description: "Get detailed information about specific tags including file lists",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args GetTagsInfoParams) (*mcp.CallToolResult, any, error) {
		return GetTagsInfoTool(ctx, req, args, manager)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_all_tags",
		Description: "List all tags with usage statistics and optional filtering",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ListAllTagsParams) (*mcp.CallToolResult, any, error) {
		return ListAllTagsTool(ctx, req, args, manager)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "replace_tags_batch",
		Description: "Replace/rename tags across multiple files with batch operation",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ReplaceTagsBatchParams) (*mcp.CallToolResult, any, error) {
		return ReplaceTagsBatchTool(ctx, req, args, manager)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_untagged_files",
		Description: "Find files that don't have any tags",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args GetUntaggedFilesParams) (*mcp.CallToolResult, any, error) {
		return GetUntaggedFilesTool(ctx, req, args, manager)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "validate_tags",
		Description: "Validate tag syntax and get suggestions for invalid tags",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ValidateTagsParams) (*mcp.CallToolResult, any, error) {
		return ValidateTagsTool(ctx, req, args, manager)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_files_tags",
		Description: "Get all tags associated with specific files",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args GetFilesTagsParams) (*mcp.CallToolResult, any, error) {
		return GetFilesTagsTool(ctx, req, args, manager)
	})

	// Set up context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	// Use provided transport or default to stdio
	if transport != nil {
		// Use the provided InMemoryTransport for testing
		return server.Run(ctx, transport)
	} else {
		// Use stdio transport for production
		return server.Run(ctx, &mcp.StdioTransport{})
	}
}
