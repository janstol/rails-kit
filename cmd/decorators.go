package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/decorators"
)

type decoratorsListJSON struct {
	Decorators []string `json:"decorators"`
}

type decoratorsJSON struct {
	ClassName   string   `json:"class_name"`
	Kind        string   `json:"kind"`
	ParentClass string   `json:"parent_class,omitempty"`
	RelPath     string   `json:"rel_path"`
	Concerns    []string `json:"concerns,omitempty"`
	Constants   []string `json:"constants,omitempty"`
	Macros      []string `json:"macros,omitempty"`
	Methods     []string `json:"methods,omitempty"`
}

var decoratorsCmd = &cobra.Command{
	Use:   "decorators [name]",
	Short: "List or inspect Rails decorators",
	Long: `Show a structural summary of a Rails decorator file.

With no arguments, lists available decorators.
With a decorator name, extracts its parent class, included concerns,
class-level constants, other class-level DSL calls (surfaced as macros), and
methods (public instance methods plus singleton def self.x class methods).
Like helpers, each method is rendered as its full signature -- a decorator's
parameters are the useful part.

The reader targets the Draper gem convention (delegate_all on an
ApplicationDecorator/Draper::Decorator subclass) but degrades gracefully: a
custom decorator implementation still resolves and reports a useful summary,
just without the Draper-specific accent.

The name can be a resource name (users, admin/reports), the full file
basename (users_decorator), a CamelCase class name (UserDecorator,
Admin::ReportDecorator), or a file path ending in .rb.`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeDecoratorNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, cfg, err := loadConfig()
		if err != nil {
			return err
		}

		if len(args) == 0 {
			return runDecoratorsList(cmd, root, cfg)
		}
		return runDecoratorsDetail(cmd, root, cfg, args[0])
	},
}

func runDecoratorsList(cmd *cobra.Command, root string, cfg config.Config) error {
	names, err := decorators.ListNames(root, cfg.DecoratorsPath)
	if err != nil {
		return fmt.Errorf("listing decorators: %w", err)
	}

	if jsonFlag {
		if names == nil {
			names = []string{}
		}
		return printJSON(cmd, decoratorsListJSON{Decorators: names})
	}

	if len(names) == 0 {
		fmt.Println("No decorators found.")
		return nil
	}
	for _, n := range names {
		fmt.Println(n)
	}
	return nil
}

func runDecoratorsDetail(cmd *cobra.Command, root string, cfg config.Config, input string) error {
	path, err := decorators.Resolve(root, cfg.DecoratorsPath, input)
	if err != nil {
		return coded(codeNotFound, err)
	}

	summary, err := decorators.Parse(path, root, cfg.DecoratorsPath)
	if err != nil {
		return fmt.Errorf("parsing decorator: %w", err)
	}
	for _, diagnostic := range summary.ParseErrors {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %s:%d: %s\n", path, diagnostic.Line, diagnostic.Message)
	}

	if jsonFlag {
		out := decoratorsJSON{
			ClassName:   summary.ClassName,
			Kind:        summary.Kind,
			ParentClass: summary.ParentClass,
			RelPath:     summary.RelPath,
			Concerns:    trimEntries(summary.Concerns),
			Constants:   trimEntries(summary.Constants),
			Macros:      trimEntries(summary.Macros),
			Methods:     trimEntries(summary.Methods),
		}
		return printJSON(cmd, out)
	}

	fmt.Print(decorators.Format(summary, stdoutStyler()))
	return nil
}

func init() {
	rootCmd.AddCommand(decoratorsCmd)
}
