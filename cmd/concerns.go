package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/janstol/rails-kit/internal/concerns"
	"github.com/janstol/rails-kit/internal/config"
)

type concernsListJSON struct {
	ModelConcerns      []string `json:"model_concerns"`
	ControllerConcerns []string `json:"controller_concerns"`
}

var concernsCmd = &cobra.Command{
	Use:   "concerns [name]",
	Short: "List or inspect Rails concerns",
	Long: `Show Rails concerns from model and controller concern directories.

With no arguments, lists all concerns grouped by type (model/controller).
With a concern name, shows parsed details: module name, methods, included block, class methods.

Concern names can be qualified to disambiguate:
  rails-kit concerns model/searchable
  rails-kit concerns controller/authenticatable`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeConcernNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, cfg, err := loadConfig()
		if err != nil {
			return err
		}
		modelDir := config.ResolvePath(root, cfg.ModelConcernsPath)
		ctrlDir := config.ResolvePath(root, cfg.ControllerConcernsPath)

		if len(args) == 0 {
			return runConcernsList(modelDir, ctrlDir)
		}
		return runConcernsDetail(modelDir, ctrlDir, root, args[0])
	},
}

func runConcernsList(modelDir, ctrlDir string) error {
	modelNames, err := concerns.ListFiles(modelDir)
	if err != nil {
		return fmt.Errorf("listing model concerns: %w", err)
	}
	ctrlNames, err := concerns.ListFiles(ctrlDir)
	if err != nil {
		return fmt.Errorf("listing controller concerns: %w", err)
	}

	if modelNames == nil {
		modelNames = []string{}
	}
	if ctrlNames == nil {
		ctrlNames = []string{}
	}

	if jsonFlag {
		return printJSON(concernsListJSON{
			ModelConcerns:      modelNames,
			ControllerConcerns: ctrlNames,
		})
	}

	if len(modelNames) == 0 && len(ctrlNames) == 0 {
		fmt.Println("No concerns found.")
		return nil
	}

	if len(modelNames) > 0 {
		fmt.Println("Model concerns:")
		for _, n := range modelNames {
			fmt.Printf("  %s\n", n)
		}
	}
	if len(ctrlNames) > 0 {
		if len(modelNames) > 0 {
			fmt.Println()
		}
		fmt.Println("Controller concerns:")
		for _, n := range ctrlNames {
			fmt.Printf("  %s\n", n)
		}
	}
	return nil
}

func runConcernsDetail(modelDir, ctrlDir, root, name string) error {
	fullPath, relPath, cType, err := concerns.FindConcern(modelDir, ctrlDir, root, name)
	if err != nil {
		return err
	}

	d, err := concerns.Parse(fullPath, relPath, cType)
	if err != nil {
		return err
	}

	if jsonFlag {
		return printJSON(d)
	}

	fmt.Print(concerns.Format(d))
	return nil
}

func init() {
	rootCmd.AddCommand(concernsCmd)
}
