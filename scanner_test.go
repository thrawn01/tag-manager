package tagmanager_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tagmanager "github.com/thrawn01/tag-manager"
)

func TestFilesystemScanner_ExtractTags(t *testing.T) {
	config := tagmanager.DefaultConfig()
	scanner, err := tagmanager.NewFilesystemScanner(config)
	if err != nil {
		t.Fatal(err)
	}

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
			
			if len(tags) != len(test.expected) {
				t.Errorf("Expected %d tags, got %d: %v", len(test.expected), len(tags), tags)
				return
			}

			tagMap := make(map[string]bool)
			for _, tag := range tags {
				tagMap[tag] = true
			}

			for _, expected := range test.expected {
				if !tagMap[expected] {
					t.Errorf("Expected tag %s not found in %v", expected, tags)
				}
			}
		})
	}
}

func TestFilesystemScanner_ScanDirectory(t *testing.T) {
	tempDir := t.TempDir()

	testFiles := map[string]string{
		"file1.md":         "# File 1\n#golang #programming",
		"file2.md":         "# File 2\n#python #data-science",
		"untagged.md":      "# Untagged\nNo tags here",
		"file.excalidraw.md": "# Excalidraw\n#diagram",
		"subdir/file3.md":  "# File 3\n#javascript",
		"100 Archive/old.md": "# Old\n#archived",
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(tempDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	config := tagmanager.DefaultConfig()
	scanner, err := tagmanager.NewFilesystemScanner(config)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	var results []tagmanager.FileTagInfo

	for fileInfo, err := range scanner.ScanDirectory(ctx, tempDir, nil) {
		if err != nil {
			t.Errorf("Scan error: %v", err)
			continue
		}
		results = append(results, fileInfo)
	}

	expectedFiles := []string{"file1.md", "file2.md", "untagged.md", "subdir/file3.md"}
	
	if len(results) != len(expectedFiles) {
		t.Errorf("Expected %d files, got %d", len(expectedFiles), len(results))
	}

	for _, result := range results {
		relPath, _ := filepath.Rel(tempDir, result.Path)
		
		if strings.Contains(relPath, "excalidraw") {
			t.Errorf("Excalidraw file should be excluded: %s", relPath)
		}
		
		if strings.Contains(relPath, "100 Archive") {
			t.Errorf("Archive file should be excluded: %s", relPath)
		}
	}
}

func TestFilesystemScanner_EdgeCases(t *testing.T) {
	config := tagmanager.DefaultConfig()
	scanner, err := tagmanager.NewFilesystemScanner(config)
	if err != nil {
		t.Fatal(err)
	}

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

			if len(tags) != len(test.expected) {
				t.Errorf("Expected %d tags, got %d: %v", len(test.expected), len(tags), tags)
				return
			}

			tagMap := make(map[string]bool)
			for _, tag := range tags {
				tagMap[tag] = true
			}

			for _, expected := range test.expected {
				if !tagMap[expected] {
					t.Errorf("Expected tag %s not found in %v", expected, tags)
				}
			}
		})
	}
}