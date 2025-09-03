package tagmanager

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type ListTagsParams struct {
	Root     string `json:"root"`
	MinCount int    `json:"min_count"`
}

func ListAllTagsTool(ctx context.Context, req *mcp.CallToolRequest, args ListTagsParams, manager TagManager) (*mcp.CallToolResult, any, error) {
	tags, err := manager.ListAllTags(ctx, args.Root, args.MinCount)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list tags: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Found %d tags", len(tags))},
		},
	}, tags, nil
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

	// Register tools with proper handlers
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_all_tags",
		Description: "List all tags with usage statistics",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args ListTagsParams) (*mcp.CallToolResult, any, error) {
		return ListAllTagsTool(ctx, req, args, manager)
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