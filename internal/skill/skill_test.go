package skill

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCodexContentFrontmatter(t *testing.T) {
	content, err := CodexContent()
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.SplitN(content, "---\n", 3)
	if len(parts) != 3 {
		t.Fatalf("unexpected frontmatter: %q", content)
	}
	var metadata map[string]any
	if err := yaml.Unmarshal([]byte(parts[1]), &metadata); err != nil {
		t.Fatal(err)
	}
	if len(metadata) != 2 || metadata["name"] != "rails-kit" || metadata["description"] == "" {
		t.Fatalf("unexpected Codex frontmatter: %#v", metadata)
	}
	if !strings.Contains(parts[2], "`rails-kit` is a compiled Go binary") {
		t.Fatal("Codex skill body was not preserved")
	}
}

func TestOpenAIMetadata(t *testing.T) {
	var metadata map[string]any
	if err := yaml.Unmarshal([]byte(OpenAIMetadata), &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata["interface"] == nil {
		t.Fatal("OpenAI metadata is missing interface settings")
	}
}
