package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/janstol/rails-kit/internal/pluralize"
	"github.com/janstol/rails-kit/internal/routes"
)

var (
	routesRefresh bool
	routesNoCache bool
	routesStatic  bool
)

var routesCmd = &cobra.Command{
	Use:   "routes [pattern...]",
	Short: "Cached, filtered rails routes output",
	Long: `Show rails routes output with optional filtering.

With no arguments, prints all routes (using cache if available).
With patterns, prints only routes matching any pattern (case-insensitive).

The cache is stored in tmp/routes_cache.txt and is invalidated when
	config/routes.rb or any file in config/routes/ is modified.

--static parses config/routes.rb directly in pure Go, without booting
Rails or shelling out to bundler. It's fast and works even when the app
can't boot, but it's an approximation: it understands resources/resource,
resource path/controller/helper/parameter options, namespaces, static
path/module/helper/controller scopes, standard or parenthesized namespaces,
static controller blocks, root, recursively drawn route files,
member/collection blocks, single-declaration inline namespace/member/collection
blocks, action filters, static block-defined route concerns, single-line or
comma-continued verb and match routes, and literal string redirects with
optional numeric status codes. Static engine, Rack, and constant-qualified
receiver mount points are represented, but their internal routes are not
expanded. Generated resource helpers follow Rails inspector output: only the
first enabled route sharing a helper displays its prefix. Conditional mounts are
included without evaluating their conditions and produce warnings. Callable or
parameterized concerns, executable inline blocks, dynamic redirects, and routes
drawn by gems are not modeled. Inline regex constraints for named path
parameters are included in endpoint text; other constraints remain approximate
and produce warnings. Use it for a quick answer, not as a replacement for
"rails routes".`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if routesRefresh && routesNoCache {
			return fmt.Errorf("--refresh and --no-cache are mutually exclusive")
		}
		if routesStatic && (routesRefresh || routesNoCache) {
			return fmt.Errorf("--static cannot be combined with --refresh or --no-cache")
		}

		root, err := resolveRailsRoot()
		if err != nil {
			return err
		}

		if routesStatic {
			routesPath := filepath.Join(root, "config", "routes.rb")
			result, err := routes.ParseStaticDetailed(routesPath, pluralize.Default())
			if err != nil {
				return fmt.Errorf("parsing %s: %w", routesPath, err)
			}
			for _, warning := range result.Warnings {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %s:%d: %s\n", warning.Path, warning.Line, warning.Message)
			}
			entries := result.Entries
			if len(args) > 0 {
				entries, err = routes.FilterEntries(entries, args)
				if err != nil {
					return fmt.Errorf("filtering routes: %w", err)
				}
			}
			if jsonFlag {
				return printJSON(entries)
			}
			fmt.Print(routes.FormatTable(entries))
			return nil
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
			return fmt.Errorf("fetching routes: %w (hint: try --static for an offline, pure-Go approximation)", err)
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
	routesCmd.Flags().BoolVar(&routesStatic, "static", false, "Parse config/routes.rb directly (offline, pure Go, approximate)")
}
