package tagmanager_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		require.NoError(t, os.WriteFile(fullPath, []byte(content), tagmanager.DefaultFilePermissions))
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
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCLIGlobalFlags(t *testing.T) {
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "test.md")
	require.NoError(t, os.WriteFile(testFile, []byte("# Test\n#golang"), tagmanager.DefaultFilePermissions))

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
			assert.NoError(t, err)
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

		// Create MCP client and connect
		session, err := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v1.0.0"}, nil).Connect(ctx, clientTransport, nil)
		require.NoError(t, err)
		defer func() {
			_ = session.Close()
		}()

		// Test that we can ping the server
		err = session.Ping(ctx, nil)
		require.NoError(t, err)

		// List available tools from the server
		tools, err := session.ListTools(ctx, &mcp.ListToolsParams{})
		require.NoError(t, err)

		// Verify all expected tools are available with correct descriptions
		expectedTools := map[string]string{
			"find_files_by_tags": "Find files containing specific tags",
			"get_tags_info":      "Get detailed information about specific tags including file lists",
			"list_all_tags":      "List all tags with usage statistics and optional filtering",
			"replace_tags_batch": "Replace/rename tags across multiple files with batch operation",
			"get_untagged_files": "Find files that don't have any tags",
			"validate_tags":      "Validate tag syntax and get suggestions for invalid tags",
			"get_files_tags":     "Get all tags associated with specific files",
			"update_tags":        "Add and remove tags from specific files with automatic hashtag migration",
		}

		foundTools := make(map[string]bool)
		for _, tool := range tools.Tools {
			if expectedDesc, expected := expectedTools[tool.Name]; expected {
				foundTools[tool.Name] = true
				assert.Equal(t, expectedDesc, tool.Description)
			} else {
				t.Errorf("Unexpected tool found: %s", tool.Name)
			}
		}

		// Check that all expected tools were found
		for toolName := range expectedTools {
			assert.True(t, foundTools[toolName])
		}

		// Verify we have exactly 8 tools
		assert.Len(t, tools.Tools, 8)

	})
}

func TestUpdateTagsTool(t *testing.T) {
	tempDir := t.TempDir()
	config := tagmanager.DefaultConfig()
	manager, err := tagmanager.NewDefaultTagManager(config)
	require.NoError(t, err)

	testFile := filepath.Join(tempDir, "test.md")
	content := `#migrate-tag
# Test File
Content with #body-tag`
	require.NoError(t, os.WriteFile(testFile, []byte(content), tagmanager.DefaultFilePermissions))

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	tests := []struct {
		name          string
		args          tagmanager.TagUpdateParams
		expectError   bool
		expectedMigrated int
	}{
		{
			name: "AddTags",
			args: tagmanager.TagUpdateParams{
				AddTags:   []string{"new-tag"},
				FilePaths: []string{"test.md"},
				Root:      tempDir,
			},
			expectedMigrated: 1,
		},
		{
			name: "RemoveTags", 
			args: tagmanager.TagUpdateParams{
				RemoveTags: []string{"migrate-tag"},
				FilePaths:  []string{"test.md"},
				Root:       tempDir,
			},
		},
		{
			name: "AddAndRemoveTags",
			args: tagmanager.TagUpdateParams{
				AddTags:    []string{"added-tag"},
				RemoveTags: []string{"migrate-tag"},
				FilePaths:  []string{"test.md"},
				Root:       tempDir,
			},
		},
		{
			name: "InvalidRoot",
			args: tagmanager.TagUpdateParams{
				AddTags:   []string{"tag"},
				FilePaths: []string{"test.md"},
				Root:      "/nonexistent",
			},
			expectError: false, // UpdateTags doesn't fail, but reports errors in result
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Reset test file for each test
			require.NoError(t, os.WriteFile(testFile, []byte(content), tagmanager.DefaultFilePermissions))

			result, data, err := tagmanager.UpdateTagsTool(ctx, req, test.args, manager)
			
			if test.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.Nil(t, data)
			} else {
				assert.NoError(t, err)
				assert.Nil(t, result)
				assert.NotNil(t, data)
				
				updateResult, ok := data.(*tagmanager.TagUpdateResult)
				require.True(t, ok)
				
				if test.expectedMigrated > 0 {
					assert.Len(t, updateResult.FilesMigrated, test.expectedMigrated)
				}
			}
		})
	}
}

func TestMCPUpdateTagsIntegration(t *testing.T) {
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "test.md")
	content := `#migrate-tag #keep-tag
# Test File
Content here`
	require.NoError(t, os.WriteFile(testFile, []byte(content), tagmanager.DefaultFilePermissions))

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

	// Create MCP client and connect
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v1.0.0"}, nil).Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	defer func() {
		_ = session.Close()
	}()

	// Test that we can ping the server
	err = session.Ping(ctx, nil)
	require.NoError(t, err)

	// Test the update_tags tool through MCP
	toolParams := map[string]interface{}{
		"add_tags":   []string{"new-tag"},
		"file_paths": []string{"test.md"},
		"root":       tempDir,
	}

	toolResult, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "update_tags",
		Arguments: toolParams,
	})
	require.NoError(t, err)
	assert.NotNil(t, toolResult)

	// Verify the tool executed successfully
	assert.NotNil(t, toolResult)

	// Read the modified file to verify changes
	modifiedContent, err := os.ReadFile(testFile)
	require.NoError(t, err)
	contentStr := string(modifiedContent)

	// Should have migrated hashtags and added new tag
	assert.Contains(t, contentStr, "- migrate-tag")
	assert.Contains(t, contentStr, "- keep-tag") 
	assert.Contains(t, contentStr, "- new-tag")
	assert.NotContains(t, contentStr, "#migrate-tag")
	assert.NotContains(t, contentStr, "#keep-tag")
}

func TestUpdateCommand(t *testing.T) {
	tempDir := t.TempDir()

	testFiles := map[string]string{
		"test1.md": `#old-tag #keep-tag
# Test File 1
Content with #body-tag`,
		"test2.md": `---
title: "Test 2"
tags: ["existing"]
---
#migrate-tag
Content here`,
		"test3.md": "# Test 3\nNo tags here",
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(tempDir, path)
		require.NoError(t, os.WriteFile(fullPath, []byte(content), tagmanager.DefaultFilePermissions))
	}

	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name: "AddTags",
			args: []string{"tag-manager", "update", "--add=new-tag", "--files=test1.md", "--root=" + tempDir, "--json"},
		},
		{
			name: "RemoveTags",
			args: []string{"tag-manager", "update", "--remove=old-tag", "--files=test1.md", "--root=" + tempDir, "--json"},
		},
		{
			name: "AddAndRemoveTags",
			args: []string{"tag-manager", "update", "--add=added-tag", "--remove=old-tag", "--files=test1.md", "--root=" + tempDir, "--json"},
		},
		{
			name: "DryRunMode",
			args: []string{"tag-manager", "update", "--add=test-tag", "--files=test1.md", "--root=" + tempDir, "--dry-run", "--json"},
		},
		{
			name: "MultipleFiles",
			args: []string{"tag-manager", "update", "--add=bulk-tag", "--files=test1.md,test2.md", "--root=" + tempDir, "--json"},
		},
		{
			name: "HashtagMigration",
			args: []string{"tag-manager", "update", "--add=added-tag", "--files=test2.md", "--root=" + tempDir, "--json"},
		},
		{
			name:        "MissingAddAndRemove",
			args:        []string{"tag-manager", "update", "--files=test1.md", "--root=" + tempDir},
			expectError: true,
		},
		{
			name:        "MissingFiles",
			args:        []string{"tag-manager", "update", "--add=tag", "--root=" + tempDir},
			expectError: true,
		},
		{
			name:        "AbsoluteFilePath",
			args:        []string{"tag-manager", "update", "--add=tag", "--files=/absolute/path/file.md", "--root=" + tempDir},
			expectError: true,
		},
		{
			name:        "NonExistentFile",
			args:        []string{"tag-manager", "update", "--add=tag", "--files=nonexistent.md", "--root=" + tempDir, "--json"},
			expectError: false, // Should complete but report error in result
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := tagmanager.RunCmd(test.args)
			if test.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestUpdateParameterValidation(t *testing.T) {
	tests := []struct {
		name        string
		addTags     string
		removeTags  string
		files       string
		expectError bool
	}{
		{
			name:    "ValidAddOnly",
			addTags: "tag1,tag2",
			files:   "file.md",
		},
		{
			name:       "ValidRemoveOnly",
			removeTags: "tag1,tag2",
			files:      "file.md",
		},
		{
			name:       "ValidAddAndRemove",
			addTags:    "add-tag",
			removeTags: "remove-tag",
			files:      "file.md",
		},
		{
			name:        "NoAddOrRemove",
			files:       "file.md",
			expectError: true,
		},
		{
			name:        "NoFiles",
			addTags:     "tag",
			expectError: true,
		},
		{
			name:        "EmptyFiles",
			addTags:     "tag",
			files:       "",
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := tagmanager.ValidateUpdateParameters(test.addTags, test.removeTags, test.files)
			if test.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFilePathParsing(t *testing.T) {
	tests := []struct {
		name           string
		filesStr       string
		expectedPaths  []string
		expectError    bool
	}{
		{
			name:          "SingleFile",
			filesStr:      "file.md",
			expectedPaths: []string{"file.md"},
		},
		{
			name:          "MultipleFiles",
			filesStr:      "file1.md,file2.md,file3.md",
			expectedPaths: []string{"file1.md", "file2.md", "file3.md"},
		},
		{
			name:          "FilesWithSpaces",
			filesStr:      " file1.md , file2.md , file3.md ",
			expectedPaths: []string{"file1.md", "file2.md", "file3.md"},
		},
		{
			name:        "AbsolutePath",
			filesStr:    "/absolute/path.md",
			expectError: true,
		},
		{
			name:        "EmptyString",
			filesStr:    "",
			expectError: true,
		},
		{
			name:        "OnlySpaces",
			filesStr:    "   ",
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			paths, err := tagmanager.ParseFilePaths(test.filesStr, "/root")
			if test.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expectedPaths, paths)
			}
		})
	}
}
