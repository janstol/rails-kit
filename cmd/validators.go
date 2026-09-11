package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/validators"
)

type validatorsListJSON struct {
	Validators []string `json:"validators"`
}

type validatorsJSON struct {
	ClassName   string   `json:"class_name"`
	Kind        string   `json:"kind"`
	ParentClass string   `json:"parent_class,omitempty"`
	RelPath     string   `json:"rel_path"`
	Concerns    []string `json:"concerns,omitempty"`
	Constants   []string `json:"constants,omitempty"`
	Macros      []string `json:"macros,omitempty"`
	Methods     []string `json:"methods,omitempty"`
}

var validatorsCmd = &cobra.Command{
	Use:   "validators [name]",
	Short: "List or inspect Rails validators",
	Long: `Show a structural summary of a Rails validator file.

With no arguments, lists available validators.
With a validator name, extracts its parent class, included concerns,
class-level constants, other class-level DSL calls (surfaced as macros), and
methods (public instance methods plus singleton def self.x class methods).
Each method is rendered as its full signature -- for a validator that
signature is what tells an ActiveModel::EachValidator (validate_each) apart
from an ActiveModel::Validator (validate) at a glance.

Constants get their own section: a format regexp or an allowed-value list is
usually where a validator's actual rule lives, and that is exactly what a
structural summary surfaces and grep doesn't.

The name can be a resource name (phone, admin/access), the full file
basename (phone_validator), a CamelCase class name (PhoneValidator,
Admin::AccessValidator), or a file path ending in .rb.`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeValidatorNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, cfg, err := loadConfig()
		if err != nil {
			return err
		}

		if len(args) == 0 {
			return runValidatorsList(cmd, root, cfg)
		}
		return runValidatorsDetail(cmd, root, cfg, args[0])
	},
}

func runValidatorsList(cmd *cobra.Command, root string, cfg config.Config) error {
	names, err := validators.ListNames(root, cfg.ValidatorsPath)
	if err != nil {
		return fmt.Errorf("listing validators: %w", err)
	}

	if jsonFlag {
		if names == nil {
			names = []string{}
		}
		return printJSON(cmd, validatorsListJSON{Validators: names})
	}

	if len(names) == 0 {
		fmt.Println("No validators found.")
		return nil
	}
	for _, n := range names {
		fmt.Println(n)
	}
	return nil
}

func runValidatorsDetail(cmd *cobra.Command, root string, cfg config.Config, input string) error {
	path, err := validators.Resolve(root, cfg.ValidatorsPath, input)
	if err != nil {
		return coded(codeNotFound, err)
	}

	summary, err := validators.Parse(path, root, cfg.ValidatorsPath)
	if err != nil {
		return fmt.Errorf("parsing validator: %w", err)
	}
	for _, diagnostic := range summary.ParseErrors {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %s:%d: %s\n", path, diagnostic.Line, diagnostic.Message)
	}

	if jsonFlag {
		out := validatorsJSON{
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

	fmt.Print(validators.Format(summary, stdoutStyler()))
	return nil
}

func init() {
	rootCmd.AddCommand(validatorsCmd)
}
