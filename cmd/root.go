package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/railsroot"
	"github.com/janstol/rails-kit/internal/version"
)

var rootFlag string
var jsonFlag bool

var rootCmd = &cobra.Command{
	Use:   "rails-kit",
	Short: "CLI toolkit for Rails projects",
	Long:  "rails-kit provides fast, compiled tools for reading Rails project structure without loading Rails.",
	Version: func() string {
		v, commit, buildDate := version.Info()
		return fmt.Sprintf("%s (commit: %s, built: %s)", v, commit, buildDate)
	}(),
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&rootFlag, "root", "r", "", "Rails root directory (default: auto-detect from CWD)")
	rootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "Output as JSON")
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// resolveRailsRoot returns the Rails root either from --root flag or auto-detection.
func resolveRailsRoot() (string, error) {
	if rootFlag != "" {
		if _, err := os.Stat(filepath.Join(rootFlag, "config", "application.rb")); err != nil {
			return "", fmt.Errorf("--root %q is not a Rails root (config/application.rb not found)", rootFlag)
		}
		return rootFlag, nil
	}
	return railsroot.Find()
}

// loadConfig finds the Rails root and loads its config.
func loadConfig() (string, config.Config, error) {
	root, err := resolveRailsRoot()
	if err != nil {
		return "", config.Config{}, err
	}
	cfg, err := config.Load(root)
	if err != nil {
		return "", config.Config{}, fmt.Errorf("loading config: %w", err)
	}
	return root, cfg, nil
}
