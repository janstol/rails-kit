package fixtures

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reERBOutput = regexp.MustCompile(`(?s)["']<%=.*?%>["']|<%=.*?%>`)
	reERBCode   = regexp.MustCompile(`(?s)<%.*?%>`)
)

// ValidateERBUsage rejects ERB that can change fixture structure without being
// evaluated, since rails-kit intentionally does not execute templates.
func ValidateERBUsage(content string) error {
	lines := strings.Split(content, "\n")
	blockScalarIndent := -1

	for i, rawLine := range lines {
		lineNo := i + 1
		indent := leadingIndent(rawLine)
		trimmed := strings.TrimSpace(rawLine)

		if blockScalarIndent >= 0 {
			if trimmed == "" {
				continue
			}
			if indent > blockScalarIndent {
				if err := validateERBTags(rawLine, lineNo); err != nil {
					return err
				}
				continue
			}
			blockScalarIndent = -1
		}

		if trimmed == "" {
			continue
		}

		if err := validateERBTags(rawLine, lineNo); err != nil {
			return err
		}
		if !strings.Contains(rawLine, "<%=") {
			if beginsBlockScalar(trimmed) {
				blockScalarIndent = indent
			}
			continue
		}
		if outputERBInKeyPosition(trimmed) {
			return fmt.Errorf("unsupported structural ERB on line %d", lineNo)
		}
		if beginsBlockScalar(trimmed) {
			blockScalarIndent = indent
		}
	}

	return nil
}

func validateERBTags(line string, lineNo int) error {
	for _, tag := range reERBCode.FindAllString(line, -1) {
		if !strings.HasPrefix(tag, "<%=") {
			return fmt.Errorf("unsupported structural ERB on line %d", lineNo)
		}
	}
	return nil
}

func outputERBInKeyPosition(trimmed string) bool {
	segment := trimmed
	if strings.HasPrefix(segment, "- ") {
		segment = strings.TrimSpace(segment[2:])
	}

	erb := strings.Index(segment, "<%=")
	if erb < 0 {
		return false
	}
	colon := strings.Index(segment, ":")
	return colon >= 0 && erb < colon
}

func beginsBlockScalar(trimmed string) bool {
	if strings.HasPrefix(trimmed, "- |") || strings.HasPrefix(trimmed, "- >") {
		return true
	}
	colon := strings.Index(trimmed, ":")
	if colon < 0 {
		return false
	}
	rest := strings.TrimSpace(trimmed[colon+1:])
	return strings.HasPrefix(rest, "|") || strings.HasPrefix(rest, ">")
}

func leadingIndent(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

// StripERB replaces ERB output tags (<%= ... %>) with a placeholder string
// and removes code-only tags (<% ... %>) so the remaining content is valid YAML.
func StripERB(content string) string {
	s := reERBOutput.ReplaceAllString(content, `"__ERB__"`)
	s = reERBCode.ReplaceAllString(s, "")
	return s
}
