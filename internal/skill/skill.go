package skill

import (
	_ "embed"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed SKILL.md
var Content string

//go:embed agents/openai.yaml
var OpenAIMetadata string

type codexFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// CodexContent removes Claude-specific frontmatter while preserving the shared
// skill instructions.
func CodexContent() (string, error) {
	const delimiter = "---\n"
	if !strings.HasPrefix(Content, delimiter) {
		return "", fmt.Errorf("skill content is missing YAML frontmatter")
	}
	rest := strings.TrimPrefix(Content, delimiter)
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return "", fmt.Errorf("skill content has unterminated YAML frontmatter")
	}

	var metadata codexFrontmatter
	if err := yaml.Unmarshal([]byte(rest[:end]), &metadata); err != nil {
		return "", fmt.Errorf("parsing skill frontmatter: %w", err)
	}
	if metadata.Name == "" || metadata.Description == "" {
		return "", fmt.Errorf("skill frontmatter requires name and description")
	}
	frontmatter, err := yaml.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("rendering Codex skill frontmatter: %w", err)
	}
	return delimiter + string(frontmatter) + "---\n" + rest[end+len("\n---\n"):], nil
}
