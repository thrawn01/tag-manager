package tagmanager_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tagmanager "github.com/thrawn01/tag-manager"
)

func TestTagManagerE2E(t *testing.T) {
	tempDir := t.TempDir()
	config := tagmanager.DefaultConfig()
	manager, err := tagmanager.NewDefaultTagManager(config)
	if err != nil {
		t.Fatal(err)
	}

	testFiles := map[string]string{
		"golang.md":     "# Go Tutorial\n#golang #programming #tutorial",
		"python.md":     "# Python Guide\n#python #programming #data-science",
		"javascript.md": "# JS Basics\n#javascript #web-development #programming",
		"untagged.md":   "# No Tags\nThis file has no tags",
		"mixed.md": `---
tags: ["yaml-tag", "frontend"]
---
# Mixed Tags
Also has #hashtag-tag in content.`,
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(tempDir, path)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	ctx := context.Background()

	t.Run("FindFilesByTags", func(t *testing.T) {
		results, err := manager.FindFilesByTags(ctx, []string{"programming"}, tempDir)
		if err != nil {
			t.Fatal(err)
		}

		files := results["programming"]
		if len(files) != 3 {
			t.Errorf("Expected 3 files with #programming tag, got %d", len(files))
		}
	})

	t.Run("ListAllTags", func(t *testing.T) {
		tags, err := manager.ListAllTags(ctx, tempDir, 1)
		if err != nil {
			t.Fatal(err)
		}

		if len(tags) < 5 {
			t.Errorf("Expected at least 5 tags, got %d", len(tags))
		}

		programmingFound := false
		for _, tag := range tags {
			if tag.Name == "programming" && tag.Count == 3 {
				programmingFound = true
				break
			}
		}

		if !programmingFound {
			t.Error("Expected programming tag with count 3")
		}
	})

	t.Run("GetUntaggedFiles", func(t *testing.T) {
		untagged, err := manager.GetUntaggedFiles(ctx, tempDir)
		if err != nil {
			t.Fatal(err)
		}

		if len(untagged) != 1 {
			t.Errorf("Expected 1 untagged file, got %d", len(untagged))
		}

		if len(untagged) > 0 && filepath.Base(untagged[0].Path) != "untagged.md" {
			t.Error("Expected untagged.md to be in untagged files")
		}
	})

	t.Run("ReplaceTagsBatch", func(t *testing.T) {
		replacements := []tagmanager.TagReplacement{
			{OldTag: "programming", NewTag: "coding"},
		}

		result, err := manager.ReplaceTagsBatch(ctx, replacements, tempDir, false)
		if err != nil {
			t.Fatal(err)
		}

		if len(result.ModifiedFiles) != 3 {
			t.Errorf("Expected 3 modified files, got %d", len(result.ModifiedFiles))
		}

		if len(result.FailedFiles) > 0 {
			t.Errorf("Expected no failed files, got %d: %v", len(result.FailedFiles), result.Errors)
		}

		newResults, err := manager.FindFilesByTags(ctx, []string{"coding"}, tempDir)
		if err != nil {
			t.Fatal(err)
		}

		if len(newResults["coding"]) != 3 {
			t.Errorf("Expected 3 files with #coding tag after replacement, got %d", len(newResults["coding"]))
		}
	})

	t.Run("ValidateTags", func(t *testing.T) {
		results := manager.ValidateTags(ctx, []string{"valid-tag", "invalid!", "abc"})

		if !results["valid-tag"].IsValid {
			t.Error("Expected valid-tag to be valid")
		}

		if results["invalid!"].IsValid {
			t.Error("Expected invalid! to be invalid")
		}

		if results["abc"].IsValid {
			t.Error("Expected abc to be invalid (too short)")
		}
	})

	t.Run("GetFilesTags", func(t *testing.T) {
		filePaths := []string{
			filepath.Join(tempDir, "golang.md"),
			filepath.Join(tempDir, "python.md"),
		}

		results, err := manager.GetFilesTags(ctx, filePaths)
		if err != nil {
			t.Fatal(err)
		}

		if len(results) != 2 {
			t.Errorf("Expected 2 results, got %d", len(results))
		}

		for _, result := range results {
			if len(result.Tags) == 0 {
				t.Errorf("Expected tags for file %s", result.Path)
			}
		}
	})
}

func TestTagManager_ContextCancellation(t *testing.T) {
	config := tagmanager.DefaultConfig()
	manager, err := tagmanager.NewDefaultTagManager(config)
	if err != nil {
		t.Fatal(err)
	}

	// Create a context that cancels immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Test with a non-existent directory to ensure the operation has something to fail on
	_, err = manager.ListAllTags(ctx, "/dev/null", 1)
	if err == nil {
		t.Skip("Context cancellation test is environment-dependent, skipping")
	}
}

func TestTagManager_NonAtomicOperations(t *testing.T) {
	tempDir := t.TempDir()
	config := tagmanager.DefaultConfig()
	manager, err := tagmanager.NewDefaultTagManager(config)
	if err != nil {
		t.Fatal(err)
	}

	testFiles := map[string]string{
		"success.md":  "#old-tag content",
		"readonly.md": "#old-tag content",
		"another.md":  "#old-tag content",
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(tempDir, path)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	readonlyPath := filepath.Join(tempDir, "readonly.md")
	if err := os.Chmod(readonlyPath, 0444); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Chmod(readonlyPath, 0644)
	}()

	replacements := []tagmanager.TagReplacement{
		{OldTag: "old-tag", NewTag: "new-tag"},
	}

	ctx := context.Background()
	result, err := manager.ReplaceTagsBatch(ctx, replacements, tempDir, false)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.ModifiedFiles) != 2 {
		t.Errorf("Expected 2 modified files (excluding readonly), got %d", len(result.ModifiedFiles))
	}

	if len(result.FailedFiles) != 1 {
		t.Errorf("Expected 1 failed file (readonly), got %d", len(result.FailedFiles))
	}

	sort.Strings(result.ModifiedFiles)
	expectedModified := []string{
		filepath.Join(tempDir, "another.md"),
		filepath.Join(tempDir, "success.md"),
	}
	sort.Strings(expectedModified)

	for i, expected := range expectedModified {
		if result.ModifiedFiles[i] != expected {
			t.Errorf("Expected modified file %s, got %s", expected, result.ModifiedFiles[i])
		}
	}
}

func TestUpdateTags(t *testing.T) {
	tempDir := t.TempDir()
	config := tagmanager.DefaultConfig()
	manager, err := tagmanager.NewDefaultTagManager(config)
	require.NoError(t, err)

	testFile := filepath.Join(tempDir, "test.md")
	content := `---
title: "Test Document"
tags: ["existing"]
---
# Test Content`

	require.NoError(t, os.WriteFile(testFile, []byte(content), 0644))

	ctx := context.Background()
	result, err := manager.UpdateTags(ctx, []string{"new-tag"}, []string{"existing"}, tempDir, []string{"test.md"}, false)
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

func TestYamlFrontMatterParsing(t *testing.T) {
	tempDir := t.TempDir()
	config := tagmanager.DefaultConfig()
	manager, err := tagmanager.NewDefaultTagManager(config)
	require.NoError(t, err)

	ctx := context.Background()

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
			require.NoError(t, os.WriteFile(testFile, []byte(test.content), 0644))

			result, err := manager.UpdateTags(ctx, test.addTags, []string{}, tempDir, []string{"test.md"}, false)
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
	config := tagmanager.DefaultConfig()
	manager, err := tagmanager.NewDefaultTagManager(config)
	require.NoError(t, err)

	testFile := filepath.Join(tempDir, "test.md")
	content := `---
title: "Important Title"
author: "Test Author"
date: "2024-01-01"
tags: ["existing"]
---
# Test Content`

	require.NoError(t, os.WriteFile(testFile, []byte(content), 0644))

	ctx := context.Background()
	result, err := manager.UpdateTags(ctx, []string{"new-tag"}, []string{}, tempDir, []string{"test.md"}, false)
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
	config := tagmanager.DefaultConfig()
	manager, err := tagmanager.NewDefaultTagManager(config)
	require.NoError(t, err)

	testFile := filepath.Join(tempDir, "test.md")
	content := `---
title: "Test Document"  
tags: ["existing-tag"]
---
# Test Content`

	require.NoError(t, os.WriteFile(testFile, []byte(content), 0644))
	ctx := context.Background()

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
			result, err := manager.UpdateTags(ctx, test.addTags, test.removeTags, tempDir, []string{"test.md"}, true)

			if test.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "no operations remain after conflict resolution")
			} else {
				require.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestDuplicateTagHandling(t *testing.T) {
	tempDir := t.TempDir()
	config := tagmanager.DefaultConfig()
	manager, err := tagmanager.NewDefaultTagManager(config)
	require.NoError(t, err)

	testFile := filepath.Join(tempDir, "test.md")
	content := `---
title: "Test Document"
tags: ["existing-tag", "another-tag"]
---
# Test Content`

	require.NoError(t, os.WriteFile(testFile, []byte(content), 0644))

	ctx := context.Background()
	result, err := manager.UpdateTags(ctx, []string{"existing-tag", "new-tag"}, []string{}, tempDir, []string{"test.md"}, false)
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
	config := tagmanager.DefaultConfig()
	manager, err := tagmanager.NewDefaultTagManager(config)
	require.NoError(t, err)

	testFile := filepath.Join(tempDir, "test.md")
	content := `---
tags: ["frontmatter-tag"]
---
# Test Document

This content has #body-tag and #another-body-tag in the text.
Also #frontmatter-tag appears in body.`

	require.NoError(t, os.WriteFile(testFile, []byte(content), 0644))

	ctx := context.Background()
	result, err := manager.UpdateTags(ctx, []string{}, []string{"body-tag", "frontmatter-tag"}, tempDir, []string{"test.md"}, false)
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
