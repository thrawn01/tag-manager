package tagmanager

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/fs"
	"iter"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Scanner interface {
	ScanDirectory(ctx context.Context, rootPath string, excludePaths []string) iter.Seq2[FileTagInfo, error]
	ScanFile(ctx context.Context, filePath string) (FileTagInfo, error)
	ExtractTags(content string) []string
	ExtractTagsFromReader(ctx context.Context, reader io.Reader) []string
}

type FilesystemScanner struct {
	config             *Config
	hashtagPattern     *regexp.Regexp
	yamlTagPattern     *regexp.Regexp
	yamlTagListPattern *regexp.Regexp
}

func NewFilesystemScanner(config *Config) (*FilesystemScanner, error) {
	hashtagPattern, err := regexp.Compile(config.HashtagPattern)
	if err != nil {
		return nil, fmt.Errorf("invalid hashtag pattern: %w", err)
	}

	yamlTagPattern, err := regexp.Compile(config.YAMLTagPattern)
	if err != nil {
		return nil, fmt.Errorf("invalid YAML tag pattern: %w", err)
	}

	yamlTagListPattern, err := regexp.Compile(config.YAMLListPattern)
	if err != nil {
		return nil, fmt.Errorf("invalid YAML list pattern: %w", err)
	}

	return &FilesystemScanner{
		config:             config,
		hashtagPattern:     hashtagPattern,
		yamlTagPattern:     yamlTagPattern,
		yamlTagListPattern: yamlTagListPattern,
	}, nil
}

func (s *FilesystemScanner) ScanDirectory(ctx context.Context, rootPath string, excludePaths []string) iter.Seq2[FileTagInfo, error] {
	return func(yield func(FileTagInfo, error) bool) {
		allExcludes := append(s.config.ExcludeDirs, excludePaths...)

		if err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			if err != nil {
				yield(FileTagInfo{}, err)
				return nil
			}

			relPath, _ := filepath.Rel(rootPath, path)

			for _, exclude := range allExcludes {
				if strings.Contains(relPath, exclude) {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}

			if d.IsDir() {
				return nil
			}

			if !strings.HasSuffix(path, ".md") {
				return nil
			}

			for _, pattern := range s.config.ExcludePatterns {
				if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
					return nil
				}
			}

			fileInfo, err := s.ScanFile(ctx, path)
			if !yield(fileInfo, err) {
				return fmt.Errorf("scan terminated by consumer")
			}
			return nil
		}); err != nil {
			yield(FileTagInfo{}, err)
		}
	}
}

func (s *FilesystemScanner) ScanFile(ctx context.Context, filePath string) (FileTagInfo, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return FileTagInfo{Path: filePath}, err
	}

	tags := s.ExtractTags(string(content))
	return FileTagInfo{
		Path: filePath,
		Tags: tags,
	}, nil
}

func (s *FilesystemScanner) ExtractTags(content string) []string {
	tagMap := make(map[string]bool)

	hashtagMatches := s.hashtagPattern.FindAllString(content, -1)
	for _, match := range hashtagMatches {
		tag := strings.TrimPrefix(match, "#")
		if s.isValidTag(tag) && s.checkHashtagBoundary(content, match) {
			tagMap[tag] = true
		}
	}

	if yamlMatch := s.yamlTagPattern.FindStringSubmatch(content); len(yamlMatch) > 1 {
		tags := strings.Split(yamlMatch[1], ",")
		for _, tag := range tags {
			tag = strings.TrimSpace(tag)
			tag = strings.Trim(tag, `"'`)
			if s.isValidTag(tag) {
				tagMap[tag] = true
			}
		}
	}

	if yamlListMatch := s.yamlTagListPattern.FindStringSubmatch(content); len(yamlListMatch) > 1 {
		lines := strings.Split(yamlListMatch[1], "\n")
		for _, line := range lines {
			tag := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
			tag = strings.Trim(tag, `"'`)
			if s.isValidTag(tag) {
				tagMap[tag] = true
			}
		}
	}

	var tags []string
	for tag := range tagMap {
		tags = append(tags, tag)
	}
	return tags
}

func (s *FilesystemScanner) ExtractTagsFromReader(ctx context.Context, reader io.Reader) []string {
	scanner := bufio.NewScanner(reader)
	var content strings.Builder

	for scanner.Scan() {
		if ctx.Err() != nil {
			return nil
		}
		content.WriteString(scanner.Text())
		content.WriteString("\n")
	}

	return s.ExtractTags(content.String())
}

func (s *FilesystemScanner) isValidTag(tag string) bool {
	if len(tag) < s.config.MinTagLength {
		return false
	}

	for _, keyword := range s.config.ExcludeKeywords {
		if strings.Contains(strings.ToLower(tag), keyword) {
			return false
		}
	}

	if s.isHexColor(tag) {
		return false
	}

	if s.looksLikeID(tag) {
		return false
	}

	if s.isURLFragment(tag) {
		return false
	}

	digitCount := 0
	for _, ch := range tag {
		if ch >= '0' && ch <= '9' {
			digitCount++
		}
	}
	digitRatio := float64(digitCount) / float64(len(tag))
	return digitRatio <= s.config.MaxDigitRatio
}

func (s *FilesystemScanner) isHexColor(tag string) bool {
	if len(tag) != 3 && len(tag) != 6 {
		return false
	}
	for _, ch := range tag {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') && (ch < 'A' || ch > 'F') {
			return false
		}
	}
	return true
}

func (s *FilesystemScanner) looksLikeID(tag string) bool {
	if len(tag) < 8 {
		return false
	}

	hasUpperCase := false
	hasLowerCase := false
	hasDigit := false
	consecutiveDigits := 0
	maxConsecutive := 0

	for _, ch := range tag {
		if ch >= 'A' && ch <= 'Z' {
			hasUpperCase = true
			consecutiveDigits = 0
		} else if ch >= 'a' && ch <= 'z' {
			hasLowerCase = true
			consecutiveDigits = 0
		} else if ch >= '0' && ch <= '9' {
			hasDigit = true
			consecutiveDigits++
			if consecutiveDigits > maxConsecutive {
				maxConsecutive = consecutiveDigits
			}
		} else {
			consecutiveDigits = 0
		}
	}

	if maxConsecutive > 4 {
		return true
	}

	if hasUpperCase && hasLowerCase && hasDigit && len(tag) > 12 {
		return true
	}

	return false
}

func (s *FilesystemScanner) isURLFragment(tag string) bool {
	urlPatterns := []string{
		"http", "https", "ftp", "www", ".com", ".org", ".net",
		"localhost", "127.0.0.1", "::1",
	}

	tagLower := strings.ToLower(tag)
	for _, pattern := range urlPatterns {
		if strings.Contains(tagLower, pattern) {
			return true
		}
	}

	return false
}

func (s *FilesystemScanner) checkHashtagBoundary(content string, hashtag string) bool {
	// Find all occurrences of this hashtag
	start := 0
	for {
		index := strings.Index(content[start:], hashtag)
		if index == -1 {
			break
		}

		absoluteIndex := start + index

		// Check character before hashtag
		validBefore := true
		if absoluteIndex > 0 {
			prevChar := content[absoluteIndex-1]
			// Don't allow @ before # (email case) or alphanumeric characters
			if prevChar == '@' ||
				(prevChar >= 'a' && prevChar <= 'z') || (prevChar >= 'A' && prevChar <= 'Z') ||
				(prevChar >= '0' && prevChar <= '9') || prevChar == '-' || prevChar == '_' {
				validBefore = false
			}
		}

		// Check character after hashtag
		validAfter := true
		endIndex := absoluteIndex + len(hashtag)
		if endIndex < len(content) {
			nextChar := content[endIndex]
			if (nextChar >= 'a' && nextChar <= 'z') || (nextChar >= 'A' && nextChar <= 'Z') ||
				(nextChar >= '0' && nextChar <= '9') || nextChar == '-' || nextChar == '_' {
				validAfter = false
			}
		}

		// If this occurrence is valid, return true
		if validBefore && validAfter {
			return true
		}

		// Move to next possible occurrence
		start = absoluteIndex + 1
	}

	// No valid occurrence found
	return false
}
