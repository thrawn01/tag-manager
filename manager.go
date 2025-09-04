package tagmanager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const DefaultFilePermissions = 0644

type TagManager interface {
	FindFilesByTags(ctx context.Context, tags []string, rootPath string) (map[string][]string, error)
	GetTagsInfo(ctx context.Context, tags []string, rootPath string) ([]TagInfo, error)
	ListAllTags(ctx context.Context, rootPath string, minCount int) ([]TagInfo, error)
	ReplaceTagsBatch(ctx context.Context, replacements []TagReplacement, rootPath string, dryRun bool) (*TagReplaceResult, error)
	GetUntaggedFiles(ctx context.Context, rootPath string) ([]FileTagInfo, error)
	GetFilesTags(ctx context.Context, filePaths []string) ([]FileTagInfo, error)
	ValidateTags(ctx context.Context, tags []string) map[string]*ValidationResult
	UpdateTags(ctx context.Context, addTags []string, removeTags []string, rootPath string, filePaths []string, dryRun bool) (*TagUpdateResult, error)
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
		return os.WriteFile(filePath, []byte(modifiedContent), DefaultFilePermissions)
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

func (m *DefaultTagManager) UpdateTags(ctx context.Context, addTags []string, removeTags []string, rootPath string, filePaths []string, dryRun bool) (*TagUpdateResult, error) {
	if err := m.validator.ValidatePath(rootPath); err != nil {
		return nil, fmt.Errorf("invalid root path: %w", err)
	}

	var resolvedAddTags, resolvedRemoveTags []string
	var err error
	
	if len(addTags) > 0 || len(removeTags) > 0 {
		resolvedAddTags, resolvedRemoveTags, err = m.resolveTagConflicts(addTags, removeTags)
		if err != nil {
			return nil, fmt.Errorf("tag conflict resolution failed: %w", err)
		}
	} else {
		resolvedAddTags = []string{}
		resolvedRemoveTags = []string{}
	}

	result := &TagUpdateResult{
		FilesMigrated: make([]string, 0),
		ModifiedFiles: make([]string, 0),
		TagsRemoved:   make(map[string]int),
		TagsAdded:     make(map[string]int),
		Errors:        make([]string, 0),
	}

	for _, filePath := range filePaths {
		cleanPath := filepath.Clean(filePath)
		if filepath.IsAbs(cleanPath) || strings.Contains(cleanPath, "..") {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: path must be relative to root and cannot contain '..'", filePath))
			continue
		}

		absolutePath := filepath.Join(rootPath, cleanPath)
		if err := m.validator.ValidatePath(absolutePath); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: invalid path: %v", filePath, err))
			continue
		}

		content, err := os.ReadFile(absolutePath)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", filePath, err))
			continue
		}

		originalContent := string(content)
		modified := false

		frontmatterData, bodyContent, err := m.parseFrontmatter(originalContent)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: malformed YAML frontmatter: %v", filePath, err))
			continue
		}

		topHashtags := m.DetectTopOfFileHashtags(bodyContent)
		migrationOccurred := len(topHashtags) > 0
		if migrationOccurred {
			result.FilesMigrated = append(result.FilesMigrated, filePath)
			for _, tag := range topHashtags {
				result.TagsAdded[tag]++
			}
			bodyContent = m.removeTopHashtags(bodyContent, topHashtags)
			modified = true
		}

		allAddTags := append(m.normalizeTags(resolvedAddTags), topHashtags...)
		addedTags, removedTags := m.updateFrontmatterTags(frontmatterData, allAddTags, m.normalizeTags(resolvedRemoveTags))
		if len(addedTags) > 0 || len(removedTags) > 0 {
			modified = true
			for _, tag := range addedTags {
				if !migrationOccurred || !containsTag(topHashtags, tag) {
					result.TagsAdded[tag]++
				}
			}
			for _, tag := range removedTags {
				result.TagsRemoved[tag]++
			}
		}

		var newContent string
		if modified {
			frontmatterString, err := m.serializeFrontmatter(frontmatterData)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: error serializing frontmatter: %v", filePath, err))
				continue
			}

			modifiedBodyContent := m.removeHashtagsFromBody(bodyContent, resolvedRemoveTags)
			newContent = frontmatterString + modifiedBodyContent
		} else {
			newContent = originalContent
		}

		if modified && !dryRun {
			if err := os.WriteFile(absolutePath, []byte(newContent), DefaultFilePermissions); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", filePath, err))
				continue
			}
		}

		if modified {
			result.ModifiedFiles = append(result.ModifiedFiles, filePath)
		}
	}

	return result, nil
}

func (m *DefaultTagManager) parseFrontmatter(content string) (map[string]interface{}, string, error) {
	lines := strings.Split(content, "\n")
	if len(lines) < 3 || lines[0] != "---" {
		return make(map[string]interface{}), content, nil
	}

	endIdx := -1
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			endIdx = i
			break
		}
	}

	if endIdx == -1 {
		return make(map[string]interface{}), content, nil
	}

	var frontmatterData map[string]interface{}
	if frontmatterContent := strings.Join(lines[1:endIdx], "\n"); frontmatterContent != "" {
		if err := yaml.Unmarshal([]byte(frontmatterContent), &frontmatterData); err != nil {
			return nil, "", fmt.Errorf("YAML parse error: %w", err)
		}
	}

	if frontmatterData == nil {
		frontmatterData = make(map[string]interface{})
	}

	return frontmatterData, strings.Join(lines[endIdx+1:], "\n"), nil
}

func (m *DefaultTagManager) serializeFrontmatter(data map[string]interface{}) (string, error) {
	if len(data) == 0 {
		return "", nil
	}

	yamlBytes, err := yaml.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("YAML marshal error: %w", err)
	}

	return "---\n" + string(yamlBytes) + "---\n", nil
}

func (m *DefaultTagManager) updateFrontmatterTags(data map[string]interface{}, addTags, removeTags []string) ([]string, []string) {
	var currentTags []string
	var addedTags []string
	var removedTagsList []string

	if tagsInterface, exists := data["tags"]; exists {
		switch v := tagsInterface.(type) {
		case []interface{}:
			for _, tag := range v {
				if tagStr, ok := tag.(string); ok {
					currentTags = append(currentTags, strings.TrimSpace(tagStr))
				}
			}
		case []string:
			for _, tag := range v {
				currentTags = append(currentTags, strings.TrimSpace(tag))
			}
		}
	}

	tagSet := make(map[string]bool)
	for _, tag := range currentTags {
		tagSet[strings.ToLower(tag)] = true
	}

	for _, tag := range addTags {
		if !tagSet[strings.ToLower(tag)] {
			currentTags = append(currentTags, tag)
			tagSet[strings.ToLower(tag)] = true
			addedTags = append(addedTags, tag)
		}
	}

	var filteredTags []string
	for _, tag := range currentTags {
		shouldRemove := false
		for _, removeTag := range removeTags {
			if strings.EqualFold(tag, removeTag) {
				shouldRemove = true
				removedTagsList = append(removedTagsList, tag)
				break
			}
		}
		if !shouldRemove {
			filteredTags = append(filteredTags, tag)
		}
	}

	if len(filteredTags) > 0 || len(addedTags) > 0 {
		sort.Strings(filteredTags)
		data["tags"] = filteredTags
	} else {
		delete(data, "tags")
	}

	return addedTags, removedTagsList
}

func (m *DefaultTagManager) resolveTagConflicts(addTags, removeTags []string) ([]string, []string, error) {
	var filteredAddTags []string
	var filteredRemoveTags []string

	normalizedAddTags := m.normalizeTags(addTags)
	normalizedRemoveTags := m.normalizeTags(removeTags)

	for _, addTag := range normalizedAddTags {
		hasConflict := false
		for _, removeTag := range normalizedRemoveTags {
			if strings.EqualFold(addTag, removeTag) {
				hasConflict = true
				break
			}
		}
		if !hasConflict {
			filteredAddTags = append(filteredAddTags, addTag)
		}
	}

	for _, removeTag := range normalizedRemoveTags {
		hasConflict := false
		for _, addTag := range normalizedAddTags {
			if strings.EqualFold(removeTag, addTag) {
				hasConflict = true
				break
			}
		}
		if !hasConflict {
			filteredRemoveTags = append(filteredRemoveTags, removeTag)
		}
	}

	if len(filteredAddTags) == 0 && len(filteredRemoveTags) == 0 {
		return nil, nil, fmt.Errorf("no operations remain after conflict resolution")
	}

	sort.Strings(filteredAddTags)
	sort.Strings(filteredRemoveTags)

	return filteredAddTags, filteredRemoveTags, nil
}

func (m *DefaultTagManager) removeHashtagsFromBody(content string, tags []string) string {
	modifiedContent := content
	for _, tag := range tags {
		normalizedTag := m.normalizeTag(tag)

		if len(normalizedTag) == 0 {
			continue
		}

		quotedTag := regexp.QuoteMeta(normalizedTag)
		pattern := `#` + quotedTag + `\b`
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}
		modifiedContent = re.ReplaceAllString(modifiedContent, "")
	}
	return modifiedContent
}

func (m *DefaultTagManager) DetectTopOfFileHashtags(content string) []string {
	boundary := m.FindFirstNonHashtagContent(content)
	return m.extractTopHashtags(content, boundary)
}

func (m *DefaultTagManager) FindFirstNonHashtagContent(content string) int {
	lines := strings.Split(content, "\n")
	
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		
		if !m.isHashtagOnlyLine(trimmed) {
			return i
		}
	}
	
	return len(lines)
}

func (m *DefaultTagManager) isHashtagOnlyLine(line string) bool {
	words := strings.Fields(line)
	if len(words) == 0 {
		return false
	}
	
	for _, word := range words {
		if !strings.HasPrefix(word, "#") {
			return false
		}
	}
	return true
}

func (m *DefaultTagManager) extractTopHashtags(content string, boundary int) []string {
	if boundary == 0 {
		return nil
	}
	
	lines := strings.Split(content, "\n")
	var hashtags []string
	
	for i := 0; i < boundary && i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		
		words := strings.Fields(line)
		for _, word := range words {
			if strings.HasPrefix(word, "#") {
				tag := m.normalizeTag(word)
				if tag != "" {
					hashtags = append(hashtags, tag)
				}
			}
		}
	}
	
	return hashtags
}

func (m *DefaultTagManager) removeTopHashtags(content string, hashtags []string) string {
	if len(hashtags) == 0 {
		return content
	}
	
	lines := strings.Split(content, "\n")
	boundary := m.FindFirstNonHashtagContent(content)
	
	for i := 0; i < boundary && i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		
		if m.isHashtagOnlyLine(line) {
			lines[i] = ""
		}
	}
	
	result := strings.Join(lines, "\n")
	result = strings.TrimLeft(result, "\n")
	return result
}

func containsTag(tags []string, target string) bool {
	for _, tag := range tags {
		if strings.EqualFold(tag, target) {
			return true
		}
	}
	return false
}
