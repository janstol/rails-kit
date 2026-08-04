package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/datagrids"
)

type datagridsListJSON struct {
	Datagrids []string `json:"datagrids"`
}

type datagridsJSON struct {
	ClassName   string   `json:"class_name"`
	ParentClass string   `json:"parent_class,omitempty"`
	RelPath     string   `json:"rel_path"`
	Concerns    []string `json:"concerns,omitempty"`
	Decorate    string   `json:"decorate,omitempty"`
	Scope       string   `json:"scope,omitempty"`
	Filters     []string `json:"filters,omitempty"`
	Columns     []string `json:"columns,omitempty"`
	Macros      []string `json:"macros,omitempty"`
	Methods     []string `json:"methods,omitempty"`
}

var datagridsCmd = &cobra.Command{
	Use:   "datagrids [name]",
	Short: "List or inspect Rails datagrids",
	Long: `Show a structural summary of a Rails datagrid file.

With no arguments, lists available datagrids.
With a datagrid name, extracts its parent class, included concerns, decorator,
scope, filters, columns, other class-level DSL calls, and methods (public
instance methods plus singleton def self.x class methods).

The name can be a resource name (example, admin/report), the file's basename
(example_datagrid), a CamelCase class name (ExampleDatagrid,
Admin::ReportDatagrid), or a file path ending in .rb.

The reader targets the ` + "`datagrid`" + ` gem DSL (filter/column/scope/decorate on a
BaseDatagrid subclass, files named *_datagrid.rb) but degrades gracefully: a
custom grid implementation or a different grid library in app/datagrids/ still
resolves and reports a useful summary (parent class, concerns, methods, and the
class-level calls it does make), just without the datagrid-gem-specific
structure.

Only the datagrid's own file is parsed -- declarations inherited from a
superclass are not resolved. See parent_class for where to look next.`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeDatagridNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, cfg, err := loadConfig()
		if err != nil {
			return err
		}

		if len(args) == 0 {
			return runDatagridsList(cmd, root, cfg)
		}
		return runDatagridsDetail(cmd, root, cfg, args[0])
	},
}

func runDatagridsList(cmd *cobra.Command, root string, cfg config.Config) error {
	names, err := datagrids.ListNames(root, cfg.DatagridsPath)
	if err != nil {
		return fmt.Errorf("listing datagrids: %w", err)
	}

	if jsonFlag {
		if names == nil {
			names = []string{}
		}
		return printJSON(cmd, datagridsListJSON{Datagrids: names})
	}

	if len(names) == 0 {
		fmt.Println("No datagrids found.")
		return nil
	}
	for _, n := range names {
		fmt.Println(n)
	}
	return nil
}

func runDatagridsDetail(cmd *cobra.Command, root string, cfg config.Config, input string) error {
	path, err := datagrids.Resolve(root, cfg.DatagridsPath, input)
	if err != nil {
		return coded(codeNotFound, err)
	}

	summary, err := datagrids.Parse(path, root, cfg.DatagridsPath)
	if err != nil {
		return fmt.Errorf("parsing datagrid: %w", err)
	}
	for _, diagnostic := range summary.ParseErrors {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %s:%d: %s\n", path, diagnostic.Line, diagnostic.Message)
	}

	if jsonFlag {
		out := datagridsJSON{
			ClassName:   summary.ClassName,
			ParentClass: summary.ParentClass,
			RelPath:     summary.RelPath,
			Concerns:    trimEntries(summary.Concerns),
			Decorate:    summary.Decorate,
			Scope:       summary.Scope,
			Filters:     trimEntries(summary.Filters),
			Columns:     trimEntries(summary.Columns),
			Macros:      trimEntries(summary.Macros),
			Methods:     trimEntries(summary.Methods),
		}
		return printJSON(cmd, out)
	}

	fmt.Print(datagrids.Format(summary, stdoutStyler()))
	return nil
}

func init() {
	rootCmd.AddCommand(datagridsCmd)
}
