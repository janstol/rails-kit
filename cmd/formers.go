package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/formers"
)

type formersListJSON struct {
	Formers []string `json:"formers"`
}

type formersJSON struct {
	ClassName   string   `json:"class_name"`
	Kind        string   `json:"kind"`
	ParentClass string   `json:"parent_class,omitempty"`
	RelPath     string   `json:"rel_path"`
	Concerns    []string `json:"concerns,omitempty"`
	Constants   []string `json:"constants,omitempty"`
	Attributes  []string `json:"attributes,omitempty"`
	Validations []string `json:"validations,omitempty"`
	Macros      []string `json:"macros,omitempty"`
	Methods     []string `json:"methods,omitempty"`
}

var formersCmd = &cobra.Command{
	Use:   "formers [name]",
	Short: "List or inspect Rails form objects",
	Long: `Show a structural summary of a Rails form object (former) file.

With no arguments, lists available formers.
With a former name, extracts its parent class, included concerns,
class-level constants, attributes, validations, other class-level DSL calls
(surfaced as macros), and methods (public instance methods plus singleton
def self.x class methods). Each method is rendered as its full signature.

Formers are only lightly parameterized, so validations and attributes get
their own sections rather than being folded into a catch-all -- a
with_options block wrapping validations is expanded inline under
Validations, indented under its own entry.

The name can be a resource name (users, admin/reports), the full file
basename (users_former, session_form), a CamelCase class name (UserFormer,
Admin::ReportFormer), or a file path ending in .rb.`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeFormerNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, cfg, err := loadConfig()
		if err != nil {
			return err
		}

		if len(args) == 0 {
			return runFormersList(cmd, root, cfg)
		}
		return runFormersDetail(cmd, root, cfg, args[0])
	},
}

func runFormersList(cmd *cobra.Command, root string, cfg config.Config) error {
	names, err := formers.ListNames(root, cfg.FormersPath)
	if err != nil {
		return fmt.Errorf("listing formers: %w", err)
	}

	if jsonFlag {
		if names == nil {
			names = []string{}
		}
		return printJSON(cmd, formersListJSON{Formers: names})
	}

	if len(names) == 0 {
		fmt.Println("No formers found.")
		return nil
	}
	for _, n := range names {
		fmt.Println(n)
	}
	return nil
}

func runFormersDetail(cmd *cobra.Command, root string, cfg config.Config, input string) error {
	path, err := formers.Resolve(root, cfg.FormersPath, input)
	if err != nil {
		return coded(codeNotFound, err)
	}

	summary, err := formers.Parse(path, root, cfg.FormersPath)
	if err != nil {
		return fmt.Errorf("parsing former: %w", err)
	}
	for _, diagnostic := range summary.ParseErrors {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %s:%d: %s\n", path, diagnostic.Line, diagnostic.Message)
	}

	if jsonFlag {
		out := formersJSON{
			ClassName:   summary.ClassName,
			Kind:        summary.Kind,
			ParentClass: summary.ParentClass,
			RelPath:     summary.RelPath,
			Concerns:    trimEntries(summary.Concerns),
			Constants:   trimEntries(summary.Constants),
			Attributes:  trimEntries(summary.Attributes),
			Validations: trimEntries(summary.Validations),
			Macros:      trimEntries(summary.Macros),
			Methods:     trimEntries(summary.Methods),
		}
		return printJSON(cmd, out)
	}

	fmt.Print(formers.Format(summary, stdoutStyler()))
	return nil
}

func init() {
	rootCmd.AddCommand(formersCmd)
}
