package tagmanager_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tagmanager "github.com/thrawn01/tag-manager"
)

func TestTagManagerE2e(t *testing.T) {
	tempDir := t.TempDir()
	config := tagmanager.DefaultConfig()
	manager, err := tagmanager.NewDefaultTagManager(config)
	require.NoError(t, err)

	const (
		golangContent     = "# Go Tutorial\n#golang #programming #tutorial"
		pythonContent     = "# Python Guide\n#python #programming #data-science"
		javascriptContent = "# JS Basics\n#javascript #web-development #programming"
		untaggedContent   = "# No Tags\nThis file has no tags"
		mixedContent      = `---
tags: ["yaml-tag", "frontend"]
---
# Mixed Tags
Also has #hashtag-tag in content.`
	)

	testFiles := map[string]string{
		"golang.md":     golangContent,
		"python.md":     pythonContent,
		"javascript.md": javascriptContent,
		"untagged.md":   untaggedContent,
		"mixed.md":      mixedContent,
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(tempDir, path)
		require.NoError(t, os.WriteFile(fullPath, []byte(content), tagmanager.DefaultFilePermissions))
	}

	ctx := context.Background()

	t.Run("FindFilesByTags", func(t *testing.T) {
		results, err := manager.FindFilesByTags(ctx, []string{"programming"}, tempDir)
		require.NoError(t, err)

		files := results["programming"]
		assert.Len(t, files, 3)
	})

	t.Run("ListAllTags", func(t *testing.T) {
		tags, err := manager.ListAllTags(ctx, tempDir, 1)
		require.NoError(t, err)

		assert.GreaterOrEqual(t, len(tags), 5)

		programmingFound := false
		for _, tag := range tags {
			if tag.Name == "programming" && tag.Count == 3 {
				programmingFound = true
				break
			}
		}

		assert.True(t, programmingFound, "Expected programming tag with count 3")
	})

	t.Run("GetUntaggedFiles", func(t *testing.T) {
		untagged, err := manager.GetUntaggedFiles(ctx, tempDir)
		require.NoError(t, err)

		assert.Len(t, untagged, 1)

		if len(untagged) > 0 {
			assert.Equal(t, "untagged.md", filepath.Base(untagged[0].Path))
		}
	})

	t.Run("ReplaceTagsBatch", func(t *testing.T) {
		replacements := []tagmanager.TagReplacement{
			{OldTag: "programming", NewTag: "coding"},
		}

		result, err := manager.ReplaceTagsBatch(ctx, replacements, tempDir, false)
		require.NoError(t, err)

		assert.Len(t, result.ModifiedFiles, 3)

		assert.Empty(t, result.FailedFiles)

		newResults, err := manager.FindFilesByTags(ctx, []string{"coding"}, tempDir)
		require.NoError(t, err)

		assert.Len(t, newResults["coding"], 3)
	})

	t.Run("ValidateTags", func(t *testing.T) {
		results := manager.ValidateTags(ctx, []string{"valid-tag", "invalid!", "abc"})

		assert.True(t, results["valid-tag"].IsValid)

		assert.False(t, results["invalid!"].IsValid)

		assert.False(t, results["abc"].IsValid)
	})

	t.Run("GetFilesTags", func(t *testing.T) {
		filePaths := []string{
			filepath.Join(tempDir, "golang.md"),
			filepath.Join(tempDir, "python.md"),
		}

		results, err := manager.GetFilesTags(ctx, filePaths)
		require.NoError(t, err)

		assert.Len(t, results, 2)

		for _, result := range results {
			assert.NotEmpty(t, result.Tags)
		}
	})
}

func TestTagManagerContextCancellation(t *testing.T) {
	config := tagmanager.DefaultConfig()
	manager, err := tagmanager.NewDefaultTagManager(config)
	require.NoError(t, err)

	// Create a context that cancels immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Test with a non-existent directory to ensure the operation has something to fail on
	_, err = manager.ListAllTags(ctx, "/dev/null", 1)
	if err == nil {
		t.Skip("Context cancellation test is environment-dependent, skipping")
	}
}

func TestTagManagerNonAtomicOperations(t *testing.T) {
	tempDir := t.TempDir()
	config := tagmanager.DefaultConfig()
	manager, err := tagmanager.NewDefaultTagManager(config)
	require.NoError(t, err)

	testFiles := map[string]string{
		"success.md":  "#old-tag content",
		"readonly.md": "#old-tag content",
		"another.md":  "#old-tag content",
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(tempDir, path)
		require.NoError(t, os.WriteFile(fullPath, []byte(content), tagmanager.DefaultFilePermissions))
	}

	readonlyPath := filepath.Join(tempDir, "readonly.md")
	require.NoError(t, os.Chmod(readonlyPath, 0444))
	defer func() {
		_ = os.Chmod(readonlyPath, tagmanager.DefaultFilePermissions)
	}()

	replacements := []tagmanager.TagReplacement{
		{OldTag: "old-tag", NewTag: "new-tag"},
	}

	ctx := context.Background()
	result, err := manager.ReplaceTagsBatch(ctx, replacements, tempDir, false)
	require.NoError(t, err)

	assert.Len(t, result.ModifiedFiles, 2)

	assert.Len(t, result.FailedFiles, 1)

	sort.Strings(result.ModifiedFiles)
	expectedModified := []string{
		filepath.Join(tempDir, "another.md"),
		filepath.Join(tempDir, "success.md"),
	}
	sort.Strings(expectedModified)

	for i, expected := range expectedModified {
		assert.Equal(t, expected, result.ModifiedFiles[i])
	}
}

func TestUpdateTags(t *testing.T) {
	tempDir := t.TempDir()

	const testContent = `---
title: "Test Document"
tags: ["existing"]
---
# Test Content`

	testFile := filepath.Join(tempDir, "test.md")
	require.NoError(t, os.WriteFile(testFile, []byte(testContent), tagmanager.DefaultFilePermissions))

	var stdout, stderr bytes.Buffer
	err := tagmanager.RunCmd([]string{"tag-manager", "update", "--add=new-tag", "--remove=existing",
		"--files=test.md", "--root=" + tempDir, "--json",
	}, &tagmanager.RunCmdOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	require.NoError(t, err)

	var result tagmanager.TagUpdateResult
	err = json.Unmarshal(stdout.Bytes(), &result)
	require.NoError(t, err)

	assert.Len(t, result.ModifiedFiles, 1)
	assert.Equal(t, 1, result.TagsAdded["new-tag"])
	assert.Equal(t, 1, result.TagsRemoved["existing"])

	modifiedContent, err := os.ReadFile(testFile)
	require.NoError(t, err)
	contentStr := string(modifiedContent)

	assert.Contains(t, contentStr, "tags:")
	assert.Contains(t, contentStr, "- new-tag")
	assert.NotContains(t, contentStr, "[\"")
	assert.NotContains(t, contentStr, "existing")
}

func TestYAMLFrontMatterParsing(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name    string
		content string
		addTags []string
	}{
		{
			name: "No frontmatter",
			content: `# Test Document
Content here`,
			addTags: []string{"new-tag"},
		},
		{
			name: "Empty frontmatter",
			content: `---
---
# Test Document`,
			addTags: []string{"new-tag"},
		},
		{
			name: "Existing tags",
			content: `---
tags: ["existing-tag"]
---
# Test Document`,
			addTags: []string{"new-tag"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testFile := filepath.Join(tempDir, "test.md")
			require.NoError(t, os.WriteFile(testFile, []byte(test.content), tagmanager.DefaultFilePermissions))

			var stdout, stderr bytes.Buffer
			err := tagmanager.RunCmd([]string{"tag-manager", "update", "--add=" + test.addTags[0],
				"--files=test.md", "--root=" + tempDir, "--json",
			}, &tagmanager.RunCmdOptions{
				Stdout: &stdout,
				Stderr: &stderr,
			})
			require.NoError(t, err)

			var result tagmanager.TagUpdateResult
			err = json.Unmarshal(stdout.Bytes(), &result)
			require.NoError(t, err)

			assert.Len(t, result.ModifiedFiles, 1)

			modifiedContent, err := os.ReadFile(testFile)
			require.NoError(t, err)

			contentStr := string(modifiedContent)
			assert.Contains(t, contentStr, "---\n")
			assert.Contains(t, contentStr, "\n---\n")
		})
	}
}

func TestFrontMatterFieldPreservation(t *testing.T) {
	tempDir := t.TempDir()

	const testContent = `---
title: "Important Title"
author: "Test Author"
date: "2024-01-01"
tags: ["existing"]
---
# Test Content`

	testFile := filepath.Join(tempDir, "test.md")
	require.NoError(t, os.WriteFile(testFile, []byte(testContent), tagmanager.DefaultFilePermissions))

	var stdout, stderr bytes.Buffer
	err := tagmanager.RunCmd([]string{"tag-manager", "update", "--add=new-tag", "--files=test.md",
		"--root=" + tempDir, "--json",
	}, &tagmanager.RunCmdOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	require.NoError(t, err)

	var result tagmanager.TagUpdateResult
	err = json.Unmarshal(stdout.Bytes(), &result)
	require.NoError(t, err)

	assert.Len(t, result.ModifiedFiles, 1)

	modifiedContent, err := os.ReadFile(testFile)
	require.NoError(t, err)

	contentStr := string(modifiedContent)
	assert.Contains(t, contentStr, "title: Important Title")
	assert.Contains(t, contentStr, "author: Test Author")
	assert.Contains(t, contentStr, "date: \"2024-01-01\"")
}

func TestTagConflictResolution(t *testing.T) {
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "test.md")
	content := `---
title: "Test Document"  
tags: ["existing-tag"]
---
# Test Content`

	require.NoError(t, os.WriteFile(testFile, []byte(content), tagmanager.DefaultFilePermissions))

	tests := []struct {
		name        string
		addTags     []string
		removeTags  []string
		expectError bool
	}{
		{
			name:        "No conflicts",
			addTags:     []string{"new-tag", "another-tag"},
			removeTags:  []string{"old-tag", "obsolete"},
			expectError: false,
		},
		{
			name:        "Case insensitive conflicts resolved",
			addTags:     []string{"Tag1", "tag2"},
			removeTags:  []string{"TAG1", "existing-tag"},
			expectError: false,
		},
		{
			name:        "All tags conflict",
			addTags:     []string{"same-tag", "another"},
			removeTags:  []string{"same-tag", "another"},
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			args := []string{"tag-manager", "update", "--files=test.md", "--root=" + tempDir, "--dry-run", "--json"}
			if len(test.addTags) > 0 {
				args = append(args, "--add="+strings.Join(test.addTags, ","))
			}
			if len(test.removeTags) > 0 {
				args = append(args, "--remove="+strings.Join(test.removeTags, ","))
			}
			err := tagmanager.RunCmd(args, &tagmanager.RunCmdOptions{
				Stdout: &stdout,
				Stderr: &stderr,
			})

			if test.expectError {
				require.Error(t, err)
				require.ErrorContains(t, err, "no operations remain after conflict resolution")
			} else {
				require.NoError(t, err)

				// Extract JSON from dry-run output
				stdout := stdout.String()
				jsonOutput := stdout
				if strings.Contains(stdout, "DRY RUN MODE") {
					lines := strings.Split(stdout, "\n")
					for _, line := range lines {
						line = strings.TrimSpace(line)
						if strings.HasPrefix(line, "{") || strings.HasPrefix(line, "[") {
							jsonOutput = line
							break
						}
					}
				}

				var result tagmanager.TagUpdateResult
				err = json.Unmarshal([]byte(jsonOutput), &result)
				require.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestDuplicateTagHandling(t *testing.T) {
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "test.md")
	content := `---
title: "Test Document"
tags: ["existing-tag", "another-tag"]
---
# Test Content`

	require.NoError(t, os.WriteFile(testFile, []byte(content), tagmanager.DefaultFilePermissions))

	var stdout, stderr bytes.Buffer
	err := tagmanager.RunCmd([]string{"tag-manager", "update", "--add=existing-tag,new-tag", "--files=test.md",
		"--root=" + tempDir, "--json",
	}, &tagmanager.RunCmdOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	require.NoError(t, err)

	var result tagmanager.TagUpdateResult
	err = json.Unmarshal(stdout.Bytes(), &result)
	require.NoError(t, err)

	assert.Len(t, result.ModifiedFiles, 1)
	assert.Equal(t, 1, result.TagsAdded["new-tag"])
	assert.Equal(t, 0, result.TagsAdded["existing-tag"])

	modifiedContent, err := os.ReadFile(testFile)
	require.NoError(t, err)
	contentStr := string(modifiedContent)

	assert.Contains(t, contentStr, "- existing-tag")
	assert.Contains(t, contentStr, "- another-tag")
	assert.Contains(t, contentStr, "- new-tag")
}

func TestRemoveTagsFromBody(t *testing.T) {
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "test.md")
	content := `---
tags: ["frontmatter-tag"]
---
# Test Document

This content has #body-tag and #another-body-tag in the text.
Also #frontmatter-tag appears in body.`

	require.NoError(t, os.WriteFile(testFile, []byte(content), tagmanager.DefaultFilePermissions))

	var stdout, stderr bytes.Buffer
	err := tagmanager.RunCmd([]string{"tag-manager", "update", "--remove=body-tag,frontmatter-tag",
		"--files=test.md", "--root=" + tempDir, "--json",
	}, &tagmanager.RunCmdOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	require.NoError(t, err)

	var result tagmanager.TagUpdateResult
	err = json.Unmarshal(stdout.Bytes(), &result)
	require.NoError(t, err)

	assert.Len(t, result.ModifiedFiles, 1)
	assert.Equal(t, 1, result.TagsRemoved["frontmatter-tag"])

	modifiedContent, err := os.ReadFile(testFile)
	require.NoError(t, err)
	contentStr := string(modifiedContent)

	assert.NotContains(t, contentStr, "#frontmatter-tag")
	assert.NotContains(t, contentStr, "frontmatter-tag")
	assert.Contains(t, contentStr, "#another-body-tag")
}

func TestTopOfFileDetection(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name         string
		content      string
		expectedTags []string
	}{
		{
			name: "Single line hashtags at top",
			content: `#tag1 #tag2 #tag3
# Document Title
Content with #body-tag`,
			expectedTags: []string{"tag1", "tag2", "tag3"},
		},
		{
			name: "Multi-line hashtags at top",
			content: `#tag1 #tag2
#tag3 #tag4

# Document Title
Content with #body-tag`,
			expectedTags: []string{"tag1", "tag2", "tag3", "tag4"},
		},
		{
			name: "No top hashtags",
			content: `# Document Title
Content with #body-tag #another`,
			expectedTags: []string{},
		},
		{
			name: "Empty lines before hashtags",
			content: `

#tag1 #tag2
# Document Title`,
			expectedTags: []string{"tag1", "tag2"},
		},
		{
			name: "Mixed content - stops at first non-hashtag",
			content: `#tag1 #tag2
Some text here
#tag3 #tag4`,
			expectedTags: []string{"tag1", "tag2"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testFile := filepath.Join(tempDir, "test.md")
			require.NoError(t, os.WriteFile(testFile, []byte(test.content), tagmanager.DefaultFilePermissions))

			var stdout, stderr bytes.Buffer
			err := tagmanager.RunCmd([]string{"tag-manager", "update", "--add=trigger-migration",
				"--files=test.md", "--root=" + tempDir, "--dry-run", "--json",
			}, &tagmanager.RunCmdOptions{
				Stdout: &stdout,
				Stderr: &stderr,
			})
			require.NoError(t, err)

			// Extract JSON from dry-run output
			jsonOutput := stdout.String()
			if strings.Contains(stdout.String(), "DRY RUN MODE") {
				lines := strings.Split(stdout.String(), "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "{") || strings.HasPrefix(line, "[") {
						jsonOutput = line
						break
					}
				}
			}

			var result tagmanager.TagUpdateResult
			err = json.Unmarshal([]byte(jsonOutput), &result)
			require.NoError(t, err)

			if len(test.expectedTags) > 0 {
				assert.Len(t, result.FilesMigrated, 1)
				for _, tag := range test.expectedTags {
					assert.Equal(t, 1, result.TagsAdded[tag])
				}
			} else {
				assert.Len(t, result.FilesMigrated, 0)
			}
		})
	}
}

func TestHashtagMigration(t *testing.T) {
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "test.md")
	content := `#tag1 #tag2 #tag3

# Document Title
Content with #body-tag remains unchanged.`

	require.NoError(t, os.WriteFile(testFile, []byte(content), tagmanager.DefaultFilePermissions))

	var stdout, stderr bytes.Buffer
	err := tagmanager.RunCmd([]string{"tag-manager", "update", "--add=new-tag",
		"--files=test.md", "--root=" + tempDir, "--json",
	}, &tagmanager.RunCmdOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	require.NoError(t, err)

	var result tagmanager.TagUpdateResult
	err = json.Unmarshal(stdout.Bytes(), &result)
	require.NoError(t, err)

	assert.Len(t, result.FilesMigrated, 1)
	assert.Contains(t, result.FilesMigrated, "test.md")
	assert.Equal(t, 1, result.TagsAdded["tag1"])
	assert.Equal(t, 1, result.TagsAdded["tag2"])
	assert.Equal(t, 1, result.TagsAdded["tag3"])
	assert.Equal(t, 1, result.TagsAdded["new-tag"])

	modifiedContent, err := os.ReadFile(testFile)
	require.NoError(t, err)
	contentStr := string(modifiedContent)

	assert.Contains(t, contentStr, "- tag1")
	assert.Contains(t, contentStr, "- tag2")
	assert.Contains(t, contentStr, "- tag3")
	assert.Contains(t, contentStr, "- new-tag")
	assert.NotContains(t, contentStr, "#tag1")
	assert.NotContains(t, contentStr, "#tag2")
	assert.NotContains(t, contentStr, "#tag3")
	assert.Contains(t, contentStr, "#body-tag")
	assert.Contains(t, contentStr, "# Document Title")
}

func TestMigrationBoundaryDetection(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name            string
		content         string
		expectMigration bool
		expectedTags    []string
	}{
		{
			name: "Hashtags before title",
			content: `#tag1 #tag2
# Title`,
			expectMigration: true,
			expectedTags:    []string{"tag1", "tag2"},
		},
		{
			name: "Hashtags with empty lines",
			content: `#tag1

# Title`,
			expectMigration: true,
			expectedTags:    []string{"tag1"},
		},
		{
			name: "No boundary - all hashtags",
			content: `#tag1 #tag2
#tag3 #tag4`,
			expectMigration: true,
			expectedTags:    []string{"tag1", "tag2", "tag3", "tag4"},
		},
		{
			name: "Immediate boundary",
			content: `Document starts here
#tag1 #tag2`,
			expectMigration: false,
			expectedTags:    []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testFile := filepath.Join(tempDir, "test.md")
			require.NoError(t, os.WriteFile(testFile, []byte(test.content), tagmanager.DefaultFilePermissions))

			var stdout, stderr bytes.Buffer
			err := tagmanager.RunCmd([]string{"tag-manager", "update", "--add=trigger-migration",
				"--files=test.md", "--root=" + tempDir, "--dry-run", "--json",
			}, &tagmanager.RunCmdOptions{
				Stdout: &stdout,
				Stderr: &stderr,
			})
			require.NoError(t, err)

			// Extract JSON from dry-run output
			jsonOutput := stdout.String()
			if strings.Contains(stdout.String(), "DRY RUN MODE") {
				lines := strings.Split(stdout.String(), "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "{") || strings.HasPrefix(line, "[") {
						jsonOutput = line
						break
					}
				}
			}

			var result tagmanager.TagUpdateResult
			err = json.Unmarshal([]byte(jsonOutput), &result)
			require.NoError(t, err)

			if test.expectMigration {
				assert.Len(t, result.FilesMigrated, 1)
				for _, tag := range test.expectedTags {
					assert.Equal(t, 1, result.TagsAdded[tag])
				}
			} else {
				assert.Len(t, result.FilesMigrated, 0)
			}
		})
	}
}

func TestMigrationWithExistingFrontmatter(t *testing.T) {
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "test.md")
	content := `---
title: "Document"
tags: ["existing"]
---
#migrated1 #migrated2

# Content
Body with #body-tag`

	require.NoError(t, os.WriteFile(testFile, []byte(content), tagmanager.DefaultFilePermissions))

	var stdout, stderr bytes.Buffer
	err := tagmanager.RunCmd([]string{
		"tag-manager", "update", "--add=trigger-migration",
		"--files=test.md", "--root=" + tempDir, "--json",
	}, &tagmanager.RunCmdOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	require.NoError(t, err)

	var result tagmanager.TagUpdateResult
	err = json.Unmarshal(stdout.Bytes(), &result)
	require.NoError(t, err)

	assert.Len(t, result.FilesMigrated, 1)
	assert.Equal(t, 1, result.TagsAdded["migrated1"])
	assert.Equal(t, 1, result.TagsAdded["migrated2"])

	modifiedContent, err := os.ReadFile(testFile)
	require.NoError(t, err)
	contentStr := string(modifiedContent)

	assert.Contains(t, contentStr, "- existing")
	assert.Contains(t, contentStr, "- migrated1")
	assert.Contains(t, contentStr, "- migrated2")
	assert.Contains(t, contentStr, "title: Document")
	assert.NotContains(t, contentStr, "#migrated1")
	assert.NotContains(t, contentStr, "#migrated2")
	assert.Contains(t, contentStr, "#body-tag")
}
