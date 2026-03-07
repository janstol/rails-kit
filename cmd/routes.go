package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/janstol/rails-kit/internal/routes"
)

var (
	routesRefresh bool
	routesNoCache bool
)

var routesCmd = &cobra.Command{
	Use:   "routes [pattern...]",
	Short: "Cached, filtered rails routes output",
	Long: `Show rails routes output with optional filtering.

With no arguments, prints all routes (using cache if available).
With patterns, prints only routes matching any pattern (case-insensitive).

The cache is stored in tmp/routes_cache.txt and is invalidated when
	config/routes.rb or any file in config/routes/ is modified.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if routesRefresh && routesNoCache {
			return fmt.Errorf("--refresh and --no-cache are mutually exclusive")
		}

		root, err := resolveRailsRoot()
		if err != nil {
			return err
		}

		var output string
		switch {
		case routesNoCache:
			output, err = routes.Run(cmd.Context(), root, os.Stderr)
		case routesRefresh:
			output, err = routes.Refresh(cmd.Context(), root, os.Stderr)
		default:
			output, err = routes.Cache(cmd.Context(), root, os.Stderr)
		}
		if err != nil {
			return fmt.Errorf("fetching routes: %w", err)
		}

		if len(args) == 0 {
			if jsonFlag {
				entries, err := routes.ParseTable(output)
				if err != nil {
					return err
				}
				return printJSON(entries)
			}
			fmt.Print(output)
			return nil
		}

		filtered, err := routes.Filter(output, args)
		if err != nil {
			return fmt.Errorf("filtering routes: %w", err)
		}
		if jsonFlag {
			entries, err := routes.ParseTable(filtered)
			if err != nil {
				return err
			}
			return printJSON(entries)
		}
		fmt.Print(filtered)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(routesCmd)
	routesCmd.Flags().BoolVar(&routesRefresh, "refresh", false, "Force cache regeneration")
	routesCmd.Flags().BoolVar(&routesNoCache, "no-cache", false, "Skip cache entirely (don't read or write)")
}
