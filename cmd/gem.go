package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/gem"
)

type gemListEntry struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type gemListJSON struct {
	Gems []gemListEntry `json:"gems"`
}

var gemCmd = &cobra.Command{
	Use:   "gem [name]",
	Short: "Inspect gems from Gemfile.lock",
	Long: `Parse Gemfile.lock and display gem information.

With no arguments, lists all gem names with their versions.
With a gem name, shows detailed information including source,
source URL, git metadata, and dependencies.`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeGemNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, cfg, err := loadConfig()
		if err != nil {
			return err
		}
		lockPath := config.ResolvePath(root, cfg.GemfileLockPath)

		lockfile, err := gem.Parse(lockPath)
		if err != nil {
			return coded(codeParseError, fmt.Errorf("parsing Gemfile.lock: %w", err))
		}

		if len(args) == 0 {
			gems := lockfile.List()
			if jsonFlag {
				entries := make([]gemListEntry, len(gems))
				for i, g := range gems {
					entries[i] = gemListEntry{Name: g.Name, Version: g.Version}
				}
				return printJSON(cmd, gemListJSON{Gems: entries})
			}
			for _, g := range gems {
				fmt.Printf("%s (%s)\n", g.Name, g.Version)
			}
			return nil
		}

		name := args[0]
		g := lockfile.Find(name)
		if g == nil {
			return coded(codeNotFound, fmt.Errorf("gem %q not found in Gemfile.lock", name))
		}
		if jsonFlag {
			return printJSON(cmd, g)
		}
		fmt.Printf("%s (%s)\n", g.Name, g.Version)
		fmt.Printf("  source: %s\n", g.Source)
		fmt.Printf("  url: %s\n", g.SourceURL)
		if g.Revision != "" {
			fmt.Printf("  revision: %s\n", g.Revision)
		}
		if g.Branch != "" {
			fmt.Printf("  branch: %s\n", g.Branch)
		}
		if g.Tag != "" {
			fmt.Printf("  tag: %s\n", g.Tag)
		}
		if g.Ref != "" {
			fmt.Printf("  ref: %s\n", g.Ref)
		}
		if len(g.Dependencies) > 0 {
			fmt.Println("  dependencies:")
			for _, d := range g.Dependencies {
				if d.Constraint != "" {
					fmt.Printf("    %s (%s)\n", d.Name, d.Constraint)
				} else {
					fmt.Printf("    %s\n", d.Name)
				}
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(gemCmd)
}
