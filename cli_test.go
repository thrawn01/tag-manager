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

		// Verify all expected tools are available with correct descriptions
		expectedTools := map[string]string{
			"find_files_by_tags": "Find files containing specific tags",
			"get_tags_info":      "Get detailed information about specific tags including file lists",
			"list_all_tags":      "List all tags with usage statistics and optional filtering",
			"replace_tags_batch": "Replace/rename tags across multiple files with batch operation",
			"get_untagged_files": "Find files that don't have any tags",
			"validate_tags":      "Validate tag syntax and get suggestions for invalid tags",
			"get_files_tags":     "Get all tags associated with specific files",
		}

		foundTools := make(map[string]bool)
		for _, tool := range tools.Tools {
			if expectedDesc, expected := expectedTools[tool.Name]; expected {
				foundTools[tool.Name] = true
				if tool.Description != expectedDesc {
					t.Errorf("Tool %s: expected description '%s', got '%s'", tool.Name, expectedDesc, tool.Description)
				}
			} else {
				t.Errorf("Unexpected tool found: %s", tool.Name)
			}
		}

		// Check that all expected tools were found
		for toolName := range expectedTools {
			if !foundTools[toolName] {
				t.Errorf("Expected tool '%s' was not found", toolName)
			}
		}

		// Verify we have exactly 7 tools
		if len(tools.Tools) != 7 {
			t.Errorf("Expected exactly 7 tools, got %d", len(tools.Tools))
		}

		t.Logf("Successfully discovered %d tools from MCP server", len(tools.Tools))
	})
}
