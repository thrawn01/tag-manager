package tagmanager_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	tagmanager "github.com/thrawn01/tag-manager"
)

func TestCLIIntegration(t *testing.T) {
	tempDir := t.TempDir()

	testFiles := map[string]string{
		"test1.md": "# Test 1\n#golang #programming",
		"test2.md": "# Test 2\n#python #data-science",
		"test3.md": "# Test 3\nNo tags here",
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(tempDir, path)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name: "Help",
			args: []string{"tag-manager", "-h"},
		},
		{
			name: "FindCommand",
			args: []string{"tag-manager", "find", "--tags=golang", "--root=" + tempDir, "--json"},
		},
		{
			name: "ListCommand",
			args: []string{"tag-manager", "list", "--root=" + tempDir, "--json"},
		},
		{
			name: "UntaggedCommand",
			args: []string{"tag-manager", "untagged", "--root=" + tempDir, "--json"},
		},
		{
			name: "ValidateCommand",
			args: []string{"tag-manager", "validate", "--tags=valid-tag,invalid!", "--json"},
		},
		{
			name: "FileTagsCommand",
			args: []string{"tag-manager", "file-tags", "--files=" + filepath.Join(tempDir, "test1.md"), "--json"},
		},
		{
			name: "ReplaceCommandDryRun",
			args: []string{"tag-manager", "replace", "--old=golang", "--new=go", "--root=" + tempDir, "--dry-run", "--json"},
		},
		{
			name:        "InvalidCommand",
			args:        []string{"tag-manager", "invalid"},
			expectError: true,
		},
		{
			name:        "MissingRequiredArgs",
			args:        []string{"tag-manager", "find"},
			expectError: true,
		},
		{
			name: "InvalidPath",
			args: []string{"tag-manager", "find", "--tags=test", "--root=/nonexistent", "--json"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := tagmanager.RunCmd(test.args)
			if test.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestCLIGlobalFlags(t *testing.T) {
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "test.md")
	if err := os.WriteFile(testFile, []byte("# Test\n#golang"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "VerboseFlag",
			args: []string{"tag-manager", "-v", "list", "--root=" + tempDir, "--json"},
		},
		{
			name: "DryRunFlag",
			args: []string{"tag-manager", "--dry-run", "replace", "--old=golang", "--new=go", "--root=" + tempDir, "--json"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := tagmanager.RunCmd(test.args)
			if err != nil {
				t.Errorf("Unexpected error with global flags: %v", err)
			}
		})
	}
}

func TestMCPServerCapabilities(t *testing.T) {
	t.Run("MCPServerToolDiscovery", func(t *testing.T) {
		ctx := context.Background()

		// Create in-memory transports for testing
		clientTransport, serverTransport := mcp.NewInMemoryTransports()

		// Start our MCP server using RunCmdWithOptions in a goroutine
		serverDone := make(chan error, 1)
		go func() {
			options := &tagmanager.RunCmdOptions{
				MCPTransport: serverTransport,
			}
			serverDone <- tagmanager.RunCmdWithOptions([]string{"tag-manager", "-mcp"}, options)
		}()

		// Create MCP client
		client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v1.0.0"}, nil)
		session, err := client.Connect(ctx, clientTransport, nil)
		if err != nil {
			t.Fatalf("Failed to connect MCP client: %v", err)
		}
		defer func() {
			_ = session.Close()
		}()

		// Test that we can ping the server
		err = session.Ping(ctx, nil)
		if err != nil {
			t.Fatalf("Failed to ping MCP server: %v", err)
		}

		// List available tools from the server
		tools, err := session.ListTools(ctx, &mcp.ListToolsParams{})
		if err != nil {
			t.Fatalf("Failed to list tools: %v", err)
		}

		// Verify that our list_all_tags tool is available
		found := false
		for _, tool := range tools.Tools {
			if tool.Name == "list_all_tags" {
				found = true
				if tool.Description != "List all tags with usage statistics" {
					t.Errorf("Expected tool description 'List all tags with usage statistics', got '%s'", tool.Description)
				}
				break
			}
		}

		if !found {
			t.Error("Expected 'list_all_tags' tool to be available, but it was not found")
		}

		// Verify we have at least one tool
		if len(tools.Tools) == 0 {
			t.Error("Expected at least one tool to be available")
		}

		t.Logf("Successfully discovered %d tools from MCP server", len(tools.Tools))
	})
}
