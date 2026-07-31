package cmd

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/fixtures"
	"github.com/janstol/rails-kit/internal/pluralize"
)

type fixturesListJSON struct {
	Files []string `json:"files"`
}

type fixturesJSON struct {
	File    string         `json:"file"`
	Entries map[string]any `json:"entries"`
}

var fixturesCmd = &cobra.Command{
	Use:   "fixtures [name]",
	Short: "Summarize test fixture entries",
	Long: `Show a compact summary of fixture entries.

With no arguments, lists available fixture files.
With a name (singular or plural), shows each fixture entry with key fields.

Scalar ERB values are normalized to __ERB__.
Structural ERB such as loops, conditionals, or ERB-generated fixture keys is rejected.

Errors if the configured fixtures path does not exist or is not a directory.`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeFixtureNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, cfg, err := loadConfig()
		if err != nil {
			return err
		}
		fixturesDir := config.ResolvePath(root, cfg.FixturesPath)

		if len(args) == 0 {
			names, err := fixtures.ListFiles(fixturesDir)
			if err != nil {
				return fmt.Errorf("listing fixtures: %w", err)
			}
			if jsonFlag {
				return printJSON(cmd, fixturesListJSON{Files: names})
			}
			for _, n := range names {
				fmt.Println(n)
			}
			return nil
		}

		name := strings.ToLower(args[0])
		p := pluralize.New(cfg.Plurals)
		plural := pluralizeFixtureLookup(name, p)

		filename, data, err := fixtures.Load(fixturesDir, name, plural)
		if err != nil {
			return coded(codeNotFound, err)
		}
		data = fixtures.VisibleEntries(data)
		if jsonFlag {
			return printJSON(cmd, fixturesJSON{
				File:    filename,
				Entries: data,
			})
		}

		fmt.Println(filename)
		fmt.Println(strings.Repeat("-", 40))
		keys := make([]string, 0, len(data))
		for k := range data {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, fixtureName := range keys {
			fmt.Printf("  %s: %s\n", fixtureName, fixtures.Summarize(data[fixtureName]))
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(fixturesCmd)
}

func pluralizeFixtureLookup(name string, p *pluralize.Pluralizer) string {
	dir := filepath.Dir(name)
	base := filepath.Base(name)
	pluralBase := p.Pluralize(base)
	if dir == "." {
		return pluralBase
	}
	return filepath.Join(dir, pluralBase)
}
