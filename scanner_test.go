package tagmanager_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tagmanager "github.com/thrawn01/tag-manager"
)

func TestFilesystemScannerExtractTags(t *testing.T) {
	config := tagmanager.DefaultConfig()
	scanner, err := tagmanager.NewFilesystemScanner(config)
	require.NoError(t, err)

	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "HashtagsOnly",
			content:  "This is a note with #golang and #programming tags.",
			expected: []string{"golang", "programming"},
		},
		{
			name:     "YAMLArrayTags",
			content:  "---\ntags: [\"golang\", \"programming\", \"tutorial\"]\n---\nContent here",
			expected: []string{"golang", "programming", "tutorial"},
		},
		{
			name: "YAMLListTags",
			content: `---
tags:
  - golang
  - programming
  - tutorial
---
Content here`,
			expected: []string{"golang", "programming", "tutorial"},
		},
		{
			name: "MixedTags",
			content: `---
tags: ["yaml-tag", "another"]
---
# Title
This has #hashtag and #more-tags in the content.`,
			expected: []string{"yaml-tag", "another", "hashtag", "more-tags"},
		},
		{
			name:     "FilterHexColors",
			content:  "Color codes #ff0000 and #abc123 should be filtered out, but #golang should remain.",
			expected: []string{"golang"},
		},
		{
			name:     "FilterShortTags",
			content:  "Short tags like #go and #ab should be filtered, but #golang should remain.",
			expected: []string{"golang"},
		},
		{
			name:     "FilterExcludedKeywords",
			content:  "Tags with #bibr123 and #ftnote should be filtered, but #valid should remain.",
			expected: []string{"valid"},
		},
		{
			name:     "FilterHighDigitRatio",
			content:  "Tags like #abc123456789 should be filtered due to high digit ratio, but #golang should remain.",
			expected: []string{"golang"},
		},
		{
			name:     "HashtagBoundaryCheck",
			content:  "Email user@domain.com#golang should not extract golang, but #golang should.",
			expected: []string{"golang"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tags := scanner.ExtractTags(test.content)

			assert.Len(t, tags, len(test.expected))

			tagMap := make(map[string]bool)
			for _, tag := range tags {
				tagMap[tag] = true
			}

			for _, expected := range test.expected {
				assert.True(t, tagMap[expected])
			}
		})
	}
}

func TestFilesystemScannerScanDirectory(t *testing.T) {
	tempDir := t.TempDir()

	const (
		file1Content           = "# File 1\n#golang #programming"
		file2Content           = "# File 2\n#python #data-science"
		untaggedContent        = "# Untagged\nNo tags here"
		excalidrawContent      = "# Excalidraw\n#diagram"
		file3Content           = "# File 3\n#javascript"
		archivedContent        = "# Old\n#archived"
	)

	testFiles := map[string]string{
		"file1.md":           file1Content,
		"file2.md":           file2Content,
		"untagged.md":        untaggedContent,
		"file.excalidraw.md": excalidrawContent,
		"subdir/file3.md":    file3Content,
		"100 Archive/old.md": archivedContent,
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(tempDir, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0755))
		require.NoError(t, os.WriteFile(fullPath, []byte(content), tagmanager.DefaultFilePermissions))
	}

	config := tagmanager.DefaultConfig()
	scanner, err := tagmanager.NewFilesystemScanner(config)
	require.NoError(t, err)

	ctx := context.Background()
	var results []tagmanager.FileTagInfo

	for fileInfo, err := range scanner.ScanDirectory(ctx, tempDir, nil) {
		require.NoError(t, err)
		results = append(results, fileInfo)
	}

	expectedFiles := []string{"file1.md", "file2.md", "untagged.md", "subdir/file3.md"}

	assert.Len(t, results, len(expectedFiles))

	for _, result := range results {
		relPath, _ := filepath.Rel(tempDir, result.Path)

		assert.NotContains(t, relPath, "excalidraw")
		assert.NotContains(t, relPath, "100 Archive")
	}
}

func TestFilesystemScannerEdgeCases(t *testing.T) {
	config := tagmanager.DefaultConfig()
	scanner, err := tagmanager.NewFilesystemScanner(config)
	require.NoError(t, err)

	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "EmptyContent",
			content:  "",
			expected: []string{},
		},
		{
			name:     "OnlySpaces",
			content:  "   \n\n   ",
			expected: []string{},
		},
		{
			name:     "SpecialCharacters",
			content:  "Content with üñíçødé #valid-tag and émojis 😀",
			expected: []string{"valid-tag"},
		},
		{
			name:     "LongLines",
			content:  strings.Repeat("a", 10000) + " #long-content",
			expected: []string{"long-content"},
		},
		{
			name:     "MalformedYAML",
			content:  "---\ntags: [incomplete\n---\n#hashtag works though",
			expected: []string{"hashtag"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tags := scanner.ExtractTags(test.content)

			assert.Len(t, tags, len(test.expected))

			tagMap := make(map[string]bool)
			for _, tag := range tags {
				tagMap[tag] = true
			}

			for _, expected := range test.expected {
				assert.True(t, tagMap[expected])
			}
		})
	}
}
