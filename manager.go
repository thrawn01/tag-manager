package tagmanager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type TagManager interface {
	FindFilesByTags(ctx context.Context, tags []string, rootPath string) (map[string][]string, error)
	GetTagsInfo(ctx context.Context, tags []string, rootPath string) ([]TagInfo, error)
	ListAllTags(ctx context.Context, rootPath string, minCount int) ([]TagInfo, error)
	ReplaceTagsBatch(ctx context.Context, replacements []TagReplacement, rootPath string, dryRun bool) (*TagReplaceResult, error)
	GetUntaggedFiles(ctx context.Context, rootPath string) ([]FileTagInfo, error)
	GetFilesTags(ctx context.Context, filePaths []string) ([]FileTagInfo, error)
	ValidateTags(ctx context.Context, tags []string) map[string]*ValidationResult
}

type DefaultTagManager struct {
	scanner   Scanner
	validator Validator
	config    *Config
}

func NewDefaultTagManager(config *Config) (*DefaultTagManager, error) {
	scanner, err := NewFilesystemScanner(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create scanner: %w", err)
	}

	return &DefaultTagManager{
		scanner:   scanner,
		validator: NewDefaultValidator(config),
		config:    config,
	}, nil
}

func (m *DefaultTagManager) FindFilesByTags(ctx context.Context, tags []string, rootPath string) (map[string][]string, error) {
	if err := m.validator.ValidatePath(rootPath); err != nil {
		return nil, fmt.Errorf("invalid root path: %w", err)
	}

	normalizedTags := m.normalizeTags(tags)
	result := make(map[string][]string)
	for _, tag := range normalizedTags {
		result[tag] = []string{}
	}

	for fileInfo, err := range m.scanner.ScanDirectory(ctx, rootPath, nil) {
		if err != nil {
			continue
		}

		fileTags := make(map[string]bool)
		for _, tag := range fileInfo.Tags {
			fileTags[m.normalizeTag(tag)] = true
		}

		for _, searchTag := range normalizedTags {
			if fileTags[searchTag] {
				result[searchTag] = append(result[searchTag], fileInfo.Path)
			}
		}
	}

	return result, nil
}

func (m *DefaultTagManager) GetTagsInfo(ctx context.Context, tags []string, rootPath string) ([]TagInfo, error) {
	if err := m.validator.ValidatePath(rootPath); err != nil {
		return nil, fmt.Errorf("invalid root path: %w", err)
	}

	filesByTag, err := m.FindFilesByTags(ctx, tags, rootPath)
	if err != nil {
		return nil, err
	}

	var result []TagInfo
	for tag, files := range filesByTag {
		result = append(result, TagInfo{
			Name:  tag,
			Count: len(files),
			Files: files,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})

	return result, nil
}

func (m *DefaultTagManager) ListAllTags(ctx context.Context, rootPath string, minCount int) ([]TagInfo, error) {
	if err := m.validator.ValidatePath(rootPath); err != nil {
		return nil, fmt.Errorf("invalid root path: %w", err)
	}

	tagCounts := make(map[string]map[string]bool)

	for fileInfo, err := range m.scanner.ScanDirectory(ctx, rootPath, nil) {
		if err != nil {
			continue
		}

		for _, tag := range fileInfo.Tags {
			normalized := m.normalizeTag(tag)
			if tagCounts[normalized] == nil {
				tagCounts[normalized] = make(map[string]bool)
			}
			tagCounts[normalized][fileInfo.Path] = true
		}
	}

	var result []TagInfo
	for tag, files := range tagCounts {
		count := len(files)
		if count >= minCount {
			fileList := make([]string, 0, len(files))
			for file := range files {
				fileList = append(fileList, file)
			}
			sort.Strings(fileList)

			result = append(result, TagInfo{
				Name:  tag,
				Count: count,
				Files: fileList,
			})
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Count != result[j].Count {
			return result[i].Count > result[j].Count
		}
		return result[i].Name < result[j].Name
	})

	return result, nil
}

func (m *DefaultTagManager) ReplaceTagsBatch(ctx context.Context, replacements []TagReplacement, rootPath string, dryRun bool) (*TagReplaceResult, error) {
	if err := m.validator.ValidatePath(rootPath); err != nil {
		return nil, fmt.Errorf("invalid root path: %w", err)
	}

	result := &TagReplaceResult{
		ModifiedFiles: []string{},
		FailedFiles:   []string{},
		Errors:        []string{},
	}

	filesToProcess := make(map[string]bool)
	for _, replacement := range replacements {
		normalized := m.normalizeTag(replacement.OldTag)
		files, err := m.FindFilesByTags(ctx, []string{normalized}, rootPath)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Error finding files for tag %s: %v", replacement.OldTag, err))
			continue
		}
		for _, fileList := range files {
			for _, file := range fileList {
				filesToProcess[file] = true
			}
		}
	}

	for file := range filesToProcess {
		if ctx.Err() != nil {
			break
		}

		if err := m.replaceTagsInFile(file, replacements, dryRun); err != nil {
			result.FailedFiles = append(result.FailedFiles, file)
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", file, err))
			continue
		}

		result.ModifiedFiles = append(result.ModifiedFiles, file)
	}

	sort.Strings(result.ModifiedFiles)
	sort.Strings(result.FailedFiles)

	return result, nil
}

func (m *DefaultTagManager) GetUntaggedFiles(ctx context.Context, rootPath string) ([]FileTagInfo, error) {
	if err := m.validator.ValidatePath(rootPath); err != nil {
		return nil, fmt.Errorf("invalid root path: %w", err)
	}

	var untagged []FileTagInfo

	for fileInfo, err := range m.scanner.ScanDirectory(ctx, rootPath, nil) {
		if err != nil {
			continue
		}

		if len(fileInfo.Tags) == 0 {
			untagged = append(untagged, fileInfo)
		}
	}

	sort.Slice(untagged, func(i, j int) bool {
		return untagged[i].Path < untagged[j].Path
	})

	return untagged, nil
}

func (m *DefaultTagManager) GetFilesTags(ctx context.Context, filePaths []string) ([]FileTagInfo, error) {
	var result []FileTagInfo

	for _, path := range filePaths {
		if ctx.Err() != nil {
			break
		}

		absPath, err := filepath.Abs(path)
		if err != nil {
			result = append(result, FileTagInfo{
				Path: path,
				Tags: nil,
			})
			continue
		}

		fileInfo, err := m.scanner.ScanFile(ctx, absPath)
		if err != nil {
			result = append(result, FileTagInfo{
				Path: absPath,
				Tags: nil,
			})
			continue
		}

		result = append(result, fileInfo)
	}

	return result, nil
}

func (m *DefaultTagManager) ValidateTags(ctx context.Context, tags []string) map[string]*ValidationResult {
	results := make(map[string]*ValidationResult)

	for _, tag := range tags {
		if ctx.Err() != nil {
			break
		}

		normalized := m.normalizeTag(tag)
		results[tag] = m.validator.ValidateTag(normalized)
	}

	return results
}

func (m *DefaultTagManager) replaceTagsInFile(filePath string, replacements []TagReplacement, dryRun bool) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	originalContent := string(content)
	modifiedContent := originalContent

	for _, replacement := range replacements {
		oldTag := m.normalizeTag(replacement.OldTag)
		newTag := m.normalizeTag(replacement.NewTag)

		hashtagPattern := regexp.MustCompile(`#` + regexp.QuoteMeta(oldTag) + `\b`)
		modifiedContent = hashtagPattern.ReplaceAllString(modifiedContent, "#"+newTag)

		yamlArrayPattern := regexp.MustCompile(`(tags:\s*\[[^\]]*)"?` + regexp.QuoteMeta(oldTag) + `"?([^\]]*\])`)
		modifiedContent = yamlArrayPattern.ReplaceAllString(modifiedContent, `${1}"`+newTag+`"${2}`)

		yamlListPattern := regexp.MustCompile(`(?m)(^\s+-\s+)"?` + regexp.QuoteMeta(oldTag) + `"?\s*$`)
		modifiedContent = yamlListPattern.ReplaceAllString(modifiedContent, `${1}"`+newTag+`"`)
	}

	if modifiedContent != originalContent && !dryRun {
		return os.WriteFile(filePath, []byte(modifiedContent), 0644)
	}

	return nil
}

func (m *DefaultTagManager) normalizeTag(tag string) string {
	tag = strings.TrimSpace(tag)
	tag = strings.TrimPrefix(tag, "#")
	return tag
}

func (m *DefaultTagManager) normalizeTags(tags []string) []string {
	normalized := make([]string, len(tags))
	for i, tag := range tags {
		normalized[i] = m.normalizeTag(tag)
	}
	return normalized
}
