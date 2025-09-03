package tagmanager

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ExcludeDirs     []string `yaml:"exclude_dirs"`
	ExcludePatterns []string `yaml:"exclude_patterns"`
	HashtagPattern  string   `yaml:"hashtag_pattern"`
	YAMLTagPattern  string   `yaml:"yaml_tag_pattern"`
	YAMLListPattern string   `yaml:"yaml_list_pattern"`
	MinTagLength    int      `yaml:"min_tag_length"`
	MaxDigitRatio   float64  `yaml:"max_digit_ratio"`
	ExcludeKeywords []string `yaml:"exclude_keywords"`
}

func DefaultConfig() *Config {
	return &Config{
		HashtagPattern:  `#[a-zA-Z][\w\-]*`,
		YAMLTagPattern:  `(?m)^tags:\s*\[([^\]]+)\]`,
		YAMLListPattern: `(?m)^tags:\s*$\n((?:\s+-\s+.+\n?)+)`,
		ExcludeKeywords: []string{"bibr", "ftn", "issuecomment", "discussion", "diff-"},
		ExcludeDirs:     []string{"100 Archive", "Attachments", ".git"},
		ExcludePatterns: []string{"*.excalidraw.md"},
		MaxDigitRatio:   0.5,
		MinTagLength:    3,
	}
}

func LoadConfig(path string) (*Config, error) {
	if path == "" {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	config := DefaultConfig()
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, err
	}

	return config, nil
}