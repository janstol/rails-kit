package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/controllers"
)

type controllersListJSON struct {
	Controllers []string `json:"controllers"`
}

type controllersJSON struct {
	ClassName     string   `json:"class_name"`
	ParentClass   string   `json:"parent_class,omitempty"`
	RelPath       string   `json:"rel_path"`
	Concerns      []string `json:"concerns,omitempty"`
	Filters       []string `json:"filters,omitempty"`
	RescueFrom    []string `json:"rescue_from,omitempty"`
	HelperMethods []string `json:"helper_methods,omitempty"`
	Layout        string   `json:"layout,omitempty"`
	RespondTo     []string `json:"respond_to,omitempty"`
	StrongParams  []string `json:"strong_params,omitempty"`
	Actions       []string `json:"actions,omitempty"`
}

var controllersCmd = &cobra.Command{
	Use:   "controllers [name]",
	Short: "List or inspect Rails controllers",
	Long: `Show a structural summary of a Rails controller file.

With no arguments, lists available controllers.
With a controller name, extracts filters, rescue_from handlers, helper
methods, layout, respond_to formats, strong params, and action methods.

The name can be a resource name (users, admin/reports), the controller file's
basename (users_controller), a CamelCase class name (UsersController,
Admin::ReportsController), or a file path ending in .rb.

Only the controller's own file is parsed -- filters and other declarations
inherited from a superclass (ApplicationController, etc.) are not resolved.
See parent_class for where to look next.`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeControllerNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, cfg, err := loadConfig()
		if err != nil {
			return err
		}

		if len(args) == 0 {
			return runControllersList(cmd, root, cfg)
		}
		return runControllersDetail(cmd, root, cfg, args[0])
	},
}

func runControllersList(cmd *cobra.Command, root string, cfg config.Config) error {
	names, err := controllers.ListNames(root, cfg.ControllersPath, cfg.ControllerConcernsPath)
	if err != nil {
		return fmt.Errorf("listing controllers: %w", err)
	}

	if jsonFlag {
		if names == nil {
			names = []string{}
		}
		return printJSON(cmd, controllersListJSON{Controllers: names})
	}

	if len(names) == 0 {
		fmt.Println("No controllers found.")
		return nil
	}
	for _, n := range names {
		fmt.Println(n)
	}
	return nil
}

func runControllersDetail(cmd *cobra.Command, root string, cfg config.Config, input string) error {
	path, err := controllers.Resolve(root, cfg.ControllersPath, input)
	if err != nil {
		return coded(codeNotFound, err)
	}

	summary, err := controllers.Parse(path, root, cfg.ControllersPath)
	if err != nil {
		return fmt.Errorf("parsing controller: %w", err)
	}
	for _, diagnostic := range summary.ParseErrors {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %s:%d: %s\n", path, diagnostic.Line, diagnostic.Message)
	}

	if jsonFlag {
		out := controllersJSON{
			ClassName:     summary.ClassName,
			ParentClass:   summary.ParentClass,
			RelPath:       summary.RelPath,
			Concerns:      trimEntries(summary.Concerns),
			Filters:       trimEntries(summary.Filters),
			RescueFrom:    trimEntries(summary.RescueFrom),
			HelperMethods: trimEntries(summary.HelperMethods),
			Layout:        summary.Layout,
			RespondTo:     trimEntries(summary.RespondTo),
			StrongParams:  trimEntries(summary.StrongParams),
			Actions:       trimEntries(summary.Actions),
		}
		return printJSON(cmd, out)
	}

	fmt.Print(controllers.Format(summary, stdoutStyler()))
	return nil
}

func init() {
	rootCmd.AddCommand(controllersCmd)
}
