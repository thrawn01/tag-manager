package tagmanager

type TagInfo struct {
	Name  string   `json:"name"`
	Count int      `json:"count"`
	Files []string `json:"files"`
}

type FileTagInfo struct {
	Path string   `json:"path"`
	Tags []string `json:"tags"`
}

type TagReplaceResult struct {
	ModifiedFiles []string `json:"modified_files"`
	FailedFiles   []string `json:"failed_files,omitempty"`
	Errors        []string `json:"errors,omitempty"`
}

type ScanStats struct {
	TotalFiles     int
	ProcessedFiles int
	ErrorCount     int
	LastError      error
}

type TagReplacement struct {
	OldTag string `json:"old_tag"`
	NewTag string `json:"new_tag"`
}

type ValidationResult struct {
	IsValid     bool     `json:"is_valid"`
	Issues      []string `json:"issues,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
}