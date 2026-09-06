package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/helpers"
)

type helpersListJSON struct {
	Helpers []string `json:"helpers"`
}

type helpersJSON struct {
	ClassName   string   `json:"class_name"`
	Kind        string   `json:"kind"`
	ParentClass string   `json:"parent_class,omitempty"`
	RelPath     string   `json:"rel_path"`
	Concerns    []string `json:"concerns,omitempty"`
	Constants   []string `json:"constants,omitempty"`
	Methods     []string `json:"methods,omitempty"`
}

var helpersCmd = &cobra.Command{
	Use:   "helpers [name]",
	Short: "List or inspect Rails view helpers",
	Long: `Show a structural summary of a Rails view helper file.

With no arguments, lists available helpers.
With a helper name, extracts its included concerns, class-level constants,
and methods (public instance methods plus singleton def self.x class
methods). Unlike the other readers, each method is rendered as its full
signature -- a helper is an API surface consumed from views, so its
parameters are the useful part.

The name can be a resource name (users, admin/reports), the full file
basename (users_helper), a CamelCase module name (UsersHelper,
Admin::ReportsHelper), or a file path ending in .rb.`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeHelperNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, cfg, err := loadConfig()
		if err != nil {
			return err
		}

		if len(args) == 0 {
			return runHelpersList(cmd, root, cfg)
		}
		return runHelpersDetail(cmd, root, cfg, args[0])
	},
}

func runHelpersList(cmd *cobra.Command, root string, cfg config.Config) error {
	names, err := helpers.ListNames(root, cfg.HelpersPath)
	if err != nil {
		return fmt.Errorf("listing helpers: %w", err)
	}

	if jsonFlag {
		if names == nil {
			names = []string{}
		}
		return printJSON(cmd, helpersListJSON{Helpers: names})
	}

	if len(names) == 0 {
		fmt.Println("No helpers found.")
		return nil
	}
	for _, n := range names {
		fmt.Println(n)
	}
	return nil
}

func runHelpersDetail(cmd *cobra.Command, root string, cfg config.Config, input string) error {
	path, err := helpers.Resolve(root, cfg.HelpersPath, input)
	if err != nil {
		return coded(codeNotFound, err)
	}

	summary, err := helpers.Parse(path, root, cfg.HelpersPath)
	if err != nil {
		return fmt.Errorf("parsing helper: %w", err)
	}
	for _, diagnostic := range summary.ParseErrors {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %s:%d: %s\n", path, diagnostic.Line, diagnostic.Message)
	}

	if jsonFlag {
		out := helpersJSON{
			ClassName:   summary.ClassName,
			Kind:        summary.Kind,
			ParentClass: summary.ParentClass,
			RelPath:     summary.RelPath,
			Concerns:    trimEntries(summary.Concerns),
			Constants:   trimEntries(summary.Constants),
			Methods:     trimEntries(summary.Methods),
		}
		return printJSON(cmd, out)
	}

	fmt.Print(helpers.Format(summary, stdoutStyler()))
	return nil
}

func init() {
	rootCmd.AddCommand(helpersCmd)
}
