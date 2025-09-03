package tagmanager

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

type Validator interface {
	ValidateTag(tag string) *ValidationResult
	ValidatePath(path string) error
	ValidateConfig(config *Config) error
}

type DefaultValidator struct {
	config *Config
}

func NewDefaultValidator(config *Config) *DefaultValidator {
	return &DefaultValidator{
		config: config,
	}
}

func (v *DefaultValidator) ValidateTag(tag string) *ValidationResult {
	result := &ValidationResult{
		IsValid:     true,
		Issues:      []string{},
		Suggestions: []string{},
	}

	cleanTag := strings.TrimSpace(tag)
	cleanTag = strings.TrimPrefix(cleanTag, "#")

	if cleanTag == "" {
		result.IsValid = false
		result.Issues = append(result.Issues, "Tag cannot be empty")
		return result
	}

	if len(cleanTag) < v.config.MinTagLength {
		result.IsValid = false
		result.Issues = append(result.Issues, fmt.Sprintf("Tag must be at least %d characters long", v.config.MinTagLength))
	}

	if !regexp.MustCompile(`^[a-zA-Z]`).MatchString(cleanTag) {
		result.IsValid = false
		result.Issues = append(result.Issues, "Tag must start with a letter")
		if regexp.MustCompile(`^[0-9]`).MatchString(cleanTag) {
			result.Suggestions = append(result.Suggestions, fmt.Sprintf("Consider: tag-%s", cleanTag))
		}
	}

	invalidChars := regexp.MustCompile(`[^a-zA-Z0-9\-_]`)
	if invalidChars.MatchString(cleanTag) {
		result.IsValid = false
		result.Issues = append(result.Issues, "Tag contains invalid characters (only letters, numbers, hyphens, and underscores allowed)")

		suggested := invalidChars.ReplaceAllString(cleanTag, "-")
		suggested = regexp.MustCompile(`-+`).ReplaceAllString(suggested, "-")
		suggested = strings.Trim(suggested, "-")
		if suggested != cleanTag {
			result.Suggestions = append(result.Suggestions, fmt.Sprintf("Suggested: %s", suggested))
		}
	}

	if strings.Contains(cleanTag, "--") {
		result.IsValid = false
		result.Issues = append(result.Issues, "Tag contains consecutive hyphens")
		suggested := regexp.MustCompile(`-+`).ReplaceAllString(cleanTag, "-")
		if suggested != cleanTag {
			result.Suggestions = append(result.Suggestions, fmt.Sprintf("Suggested: %s", suggested))
		}
	}

	scanner, err := NewFilesystemScanner(v.config)
	if err != nil {
		result.IsValid = false
		result.Issues = append(result.Issues, fmt.Sprintf("Invalid regex configuration: %v", err))
		return result
	}
	if scanner.isHexColor(cleanTag) {
		result.IsValid = false
		result.Issues = append(result.Issues, "Tag appears to be a hex color code")
		result.Suggestions = append(result.Suggestions, fmt.Sprintf("Consider: color-%s", cleanTag))
	}

	if scanner.looksLikeID(cleanTag) {
		result.IsValid = false
		result.Issues = append(result.Issues, "Tag appears to be an ID or hash")
		result.Suggestions = append(result.Suggestions, "Consider using a more descriptive tag name")
	}

	if scanner.isURLFragment(cleanTag) {
		result.IsValid = false
		result.Issues = append(result.Issues, "Tag appears to contain URL fragments")
		result.Suggestions = append(result.Suggestions, "Consider using a more descriptive tag name")
	}

	for _, keyword := range v.config.ExcludeKeywords {
		if strings.Contains(strings.ToLower(cleanTag), keyword) {
			result.IsValid = false
			result.Issues = append(result.Issues, fmt.Sprintf("Tag contains excluded keyword: %s", keyword))
			break
		}
	}

	digitCount := 0
	for _, ch := range cleanTag {
		if ch >= '0' && ch <= '9' {
			digitCount++
		}
	}
	digitRatio := float64(digitCount) / float64(len(cleanTag))
	if digitRatio > v.config.MaxDigitRatio {
		result.IsValid = false
		result.Issues = append(result.Issues, fmt.Sprintf("Tag contains too many digits (%.0f%% digits, max allowed: %.0f%%)",
			digitRatio*100, v.config.MaxDigitRatio*100))
		result.Suggestions = append(result.Suggestions, "Consider using more descriptive text instead of numbers")
	}

	return result
}

func (v *DefaultValidator) ValidatePath(path string) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	if !filepath.IsAbs(path) {
		return fmt.Errorf("path must be absolute")
	}

	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("path contains directory traversal")
	}

	info, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	if strings.Count(info, "..") > 0 {
		return fmt.Errorf("path contains directory traversal after resolution")
	}

	return nil
}

func (v *DefaultValidator) ValidateConfig(config *Config) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	if config.MinTagLength < 1 {
		return fmt.Errorf("min_tag_length must be at least 1")
	}

	if config.MaxDigitRatio < 0 || config.MaxDigitRatio > 1 {
		return fmt.Errorf("max_digit_ratio must be between 0 and 1")
	}

	if config.HashtagPattern == "" {
		return fmt.Errorf("hashtag_pattern cannot be empty")
	}

	if _, err := regexp.Compile(config.HashtagPattern); err != nil {
		return fmt.Errorf("invalid hashtag_pattern regex: %w", err)
	}

	if config.YAMLTagPattern != "" {
		if _, err := regexp.Compile(config.YAMLTagPattern); err != nil {
			return fmt.Errorf("invalid yaml_tag_pattern regex: %w", err)
		}
	}

	if config.YAMLListPattern != "" {
		if _, err := regexp.Compile(config.YAMLListPattern); err != nil {
			return fmt.Errorf("invalid yaml_list_pattern regex: %w", err)
		}
	}

	return nil
}
