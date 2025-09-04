package tagmanager_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	tagmanager "github.com/thrawn01/tag-manager"
)

func TestDefaultValidator_ValidateTag(t *testing.T) {
	config := tagmanager.DefaultConfig()
	validator := tagmanager.NewDefaultValidator(config)

	tests := []struct {
		name           string
		tag            string
		expectValid    bool
		expectedIssues []string
	}{
		{
			name:        "ValidTag",
			tag:         "golang",
			expectValid: true,
		},
		{
			name:        "ValidTagWithHyphen",
			tag:         "web-development",
			expectValid: true,
		},
		{
			name:        "ValidTagWithUnderscore",
			tag:         "data_science",
			expectValid: true,
		},
		{
			name:        "ValidTagWithHashPrefix",
			tag:         "#programming",
			expectValid: true,
		},
		{
			name:           "TooShort",
			tag:            "go",
			expectValid:    false,
			expectedIssues: []string{"must be at least 3 characters long"},
		},
		{
			name:           "EmptyTag",
			tag:            "",
			expectValid:    false,
			expectedIssues: []string{"cannot be empty"},
		},
		{
			name:           "OnlySpaces",
			tag:            "   ",
			expectValid:    false,
			expectedIssues: []string{"cannot be empty"},
		},
		{
			name:           "StartsWithNumber",
			tag:            "123golang",
			expectValid:    false,
			expectedIssues: []string{"must start with a letter"},
		},
		{
			name:           "InvalidCharacters",
			tag:            "invalid!",
			expectValid:    false,
			expectedIssues: []string{"invalid characters"},
		},
		{
			name:           "InvalidCharactersWithSpaces",
			tag:            "tag with spaces",
			expectValid:    false,
			expectedIssues: []string{"invalid characters"},
		},
		{
			name:           "ConsecutiveHyphens",
			tag:            "double--hyphen",
			expectValid:    false,
			expectedIssues: []string{"consecutive hyphens"},
		},
		{
			name:           "HexColor",
			tag:            "ff0000",
			expectValid:    false,
			expectedIssues: []string{"hex color code"},
		},
		{
			name:           "HexColorShort",
			tag:            "abc",
			expectValid:    false,
			expectedIssues: []string{"hex color code"},
		},
		{
			name:           "ExcludedKeyword",
			tag:            "bibr123",
			expectValid:    false,
			expectedIssues: []string{"excluded keyword"},
		},
		{
			name:           "HighDigitRatio",
			tag:            "abc12345678",
			expectValid:    false,
			expectedIssues: []string{"too many digits"},
		},
		{
			name:           "URLFragment",
			tag:            "httptest",
			expectValid:    false,
			expectedIssues: []string{"URL fragments"},
		},
		{
			name:           "LooksLikeID",
			tag:            "aB3dEf9H2jK4lM6nP8qR",
			expectValid:    false,
			expectedIssues: []string{"ID or hash"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := validator.ValidateTag(test.tag)

			assert.Equal(t, test.expectValid, result.IsValid)

			if !test.expectValid {
				assert.NotEmpty(t, result.Issues)

				for _, expectedIssue := range test.expectedIssues {
					found := false
					for _, issue := range result.Issues {
						if strings.Contains(strings.ToLower(issue), strings.ToLower(expectedIssue)) {
							found = true
							break
						}
					}
					assert.True(t, found)
				}
			}

		})
	}
}

func TestDefaultValidator_ValidatePath(t *testing.T) {
	config := tagmanager.DefaultConfig()
	validator := tagmanager.NewDefaultValidator(config)

	tests := []struct {
		name        string
		path        string
		expectError bool
	}{
		{
			name:        "ValidAbsolutePath",
			path:        "/valid/absolute/path",
			expectError: false,
		},
		{
			name:        "EmptyPath",
			path:        "",
			expectError: true,
		},
		{
			name:        "RelativePath",
			path:        "relative/path",
			expectError: true,
		},
		{
			name:        "PathWithTraversal",
			path:        "/path/../traversal",
			expectError: false, // filepath.Clean resolves this to "/traversal" which is valid
		},
		{
			name:        "PathWithMultipleTraversal",
			path:        "/path/../../traversal",
			expectError: false, // filepath.Clean resolves this to "/traversal" which is valid
		},
		{
			name:        "ValidPathWithDots",
			path:        "/path/to/.hidden/file",
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validator.ValidatePath(test.path)

			if test.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDefaultValidator_ValidateConfig(t *testing.T) {
	config := tagmanager.DefaultConfig()
	validator := tagmanager.NewDefaultValidator(config)

	tests := []struct {
		name        string
		config      *tagmanager.Config
		expectError bool
	}{
		{
			name:        "ValidConfig",
			config:      tagmanager.DefaultConfig(),
			expectError: false,
		},
		{
			name:        "NilConfig",
			config:      nil,
			expectError: true,
		},
		{
			name: "InvalidMinTagLength",
			config: &tagmanager.Config{
				MinTagLength: 0,
			},
			expectError: true,
		},
		{
			name: "InvalidMaxDigitRatio",
			config: &tagmanager.Config{
				MinTagLength:  3,
				MaxDigitRatio: 1.5,
			},
			expectError: true,
		},
		{
			name: "EmptyHashtagPattern",
			config: &tagmanager.Config{
				MinTagLength:   3,
				MaxDigitRatio:  0.5,
				HashtagPattern: "",
			},
			expectError: true,
		},
		{
			name: "InvalidHashtagPattern",
			config: &tagmanager.Config{
				MinTagLength:   3,
				MaxDigitRatio:  0.5,
				HashtagPattern: "[invalid",
			},
			expectError: true,
		},
		{
			name: "InvalidYAMLTagPattern",
			config: &tagmanager.Config{
				MinTagLength:   3,
				MaxDigitRatio:  0.5,
				HashtagPattern: `#[a-zA-Z][\w\-]*`,
				YAMLTagPattern: "[invalid",
			},
			expectError: true,
		},
		{
			name: "InvalidYAMLListPattern",
			config: &tagmanager.Config{
				MinTagLength:    3,
				MaxDigitRatio:   0.5,
				HashtagPattern:  `#[a-zA-Z][\w\-]*`,
				YAMLTagPattern:  `(?m)^tags:\s*\[([^\]]+)\]`,
				YAMLListPattern: "[invalid",
			},
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validator.ValidateConfig(test.config)

			if test.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDefaultValidator_WithInvalidRegexConfig(t *testing.T) {
	// Test that validator handles invalid regex patterns gracefully
	invalidConfig := &tagmanager.Config{
		HashtagPattern:  "[invalid",
		YAMLTagPattern:  `(?m)^tags:\s*\[([^\]]+)\]`,
		YAMLListPattern: `(?m)^tags:\s*$\n((?:\s+-\s+.+\n?)+)`,
		MinTagLength:    3,
		MaxDigitRatio:   0.5,
		ExcludeKeywords: []string{"test"},
	}

	validator := tagmanager.NewDefaultValidator(invalidConfig)

	// ValidateTag should handle the invalid regex gracefully
	result := validator.ValidateTag("test-tag")

	// Should be invalid due to regex configuration error
	assert.False(t, result.IsValid)

	// Should have an issue about invalid regex
	found := false
	for _, issue := range result.Issues {
		if strings.Contains(strings.ToLower(issue), "regex") ||
			strings.Contains(strings.ToLower(issue), "configuration") {
			found = true
			break
		}
	}

	assert.True(t, found)
}
