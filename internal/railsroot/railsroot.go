package railsroot

import (
	"fmt"
	"os"
	"path/filepath"
)

// Find walks up from the current working directory looking for config/application.rb.
func Find() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("could not determine working directory: %w", err)
	}
	return FindFrom(cwd)
}

// FindFrom walks up from dir looking for config/application.rb.
func FindFrom(dir string) (string, error) {
	current := filepath.Clean(dir)
	for {
		if _, err := os.Stat(filepath.Join(current, "config", "application.rb")); err == nil {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return "", fmt.Errorf("could not find Rails root (no config/application.rb found)")
}
