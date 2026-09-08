package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/presenters"
)

type presentersListJSON struct {
	Presenters []string `json:"presenters"`
}

type presentersJSON struct {
	ClassName   string   `json:"class_name"`
	Kind        string   `json:"kind"`
	ParentClass string   `json:"parent_class,omitempty"`
	RelPath     string   `json:"rel_path"`
	Concerns    []string `json:"concerns,omitempty"`
	Constants   []string `json:"constants,omitempty"`
	Attributes  []string `json:"attributes,omitempty"`
	Macros      []string `json:"macros,omitempty"`
	Methods     []string `json:"methods,omitempty"`
}

var presentersCmd = &cobra.Command{
	Use:   "presenters [name]",
	Short: "List or inspect Rails presenters",
	Long: `Show a structural summary of a Rails presenter file.

With no arguments, lists available presenters.
With a presenter name, extracts its parent class, included concerns,
class-level constants, attributes, other class-level DSL calls (surfaced as
macros), and methods (public instance methods plus singleton def self.x
class methods). Each method is rendered as its full signature -- a
presenter's parameters are the useful part.

attr_reader/attr_accessor/attr_writer get their own Attributes section
rather than being folded into a catch-all -- a presenter's attr_reader line
says what it wraps, which is the thing you actually want when you open one.

The name can be a resource name (users, admin/reports), the full file
basename (users_presenter), a CamelCase class name (UserPresenter,
Admin::ReportPresenter), or a file path ending in .rb.`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completePresenterNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, cfg, err := loadConfig()
		if err != nil {
			return err
		}

		if len(args) == 0 {
			return runPresentersList(cmd, root, cfg)
		}
		return runPresentersDetail(cmd, root, cfg, args[0])
	},
}

func runPresentersList(cmd *cobra.Command, root string, cfg config.Config) error {
	names, err := presenters.ListNames(root, cfg.PresentersPath)
	if err != nil {
		return fmt.Errorf("listing presenters: %w", err)
	}

	if jsonFlag {
		if names == nil {
			names = []string{}
		}
		return printJSON(cmd, presentersListJSON{Presenters: names})
	}

	if len(names) == 0 {
		fmt.Println("No presenters found.")
		return nil
	}
	for _, n := range names {
		fmt.Println(n)
	}
	return nil
}

func runPresentersDetail(cmd *cobra.Command, root string, cfg config.Config, input string) error {
	path, err := presenters.Resolve(root, cfg.PresentersPath, input)
	if err != nil {
		return coded(codeNotFound, err)
	}

	summary, err := presenters.Parse(path, root, cfg.PresentersPath)
	if err != nil {
		return fmt.Errorf("parsing presenter: %w", err)
	}
	for _, diagnostic := range summary.ParseErrors {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %s:%d: %s\n", path, diagnostic.Line, diagnostic.Message)
	}

	if jsonFlag {
		out := presentersJSON{
			ClassName:   summary.ClassName,
			Kind:        summary.Kind,
			ParentClass: summary.ParentClass,
			RelPath:     summary.RelPath,
			Concerns:    trimEntries(summary.Concerns),
			Constants:   trimEntries(summary.Constants),
			Attributes:  trimEntries(summary.Attributes),
			Macros:      trimEntries(summary.Macros),
			Methods:     trimEntries(summary.Methods),
		}
		return printJSON(cmd, out)
	}

	fmt.Print(presenters.Format(summary, stdoutStyler()))
	return nil
}

func init() {
	rootCmd.AddCommand(presentersCmd)
}
