package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/services"
)

type servicesListJSON struct {
	Services []string `json:"services"`
}

type servicesJSON struct {
	ClassName   string   `json:"class_name"`
	Kind        string   `json:"kind"`
	ParentClass string   `json:"parent_class,omitempty"`
	RelPath     string   `json:"rel_path"`
	Concerns    []string `json:"concerns,omitempty"`
	Constants   []string `json:"constants,omitempty"`
	Methods     []string `json:"methods,omitempty"`
}

var servicesCmd = &cobra.Command{
	Use:   "services [name]",
	Short: "List or inspect Rails services",
	Long: `Show a structural summary of a Rails service file.

With no arguments, lists available services.
With a service name, extracts its parent class, included concerns, class-level
constants, and methods (public instance methods plus singleton def self.x
class methods).

The name can be a resource name (user_export_service, admin/billing_service),
a CamelCase class name (UserExportService, Admin::BillingService), or a file
path ending in .rb. Services have no universal naming convention, so no suffix
is appended or stripped -- the name is matched as given.

Only the service's own file is parsed -- declarations inherited from a
superclass are not resolved. See parent_class for where to look next.`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeServiceNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, cfg, err := loadConfig()
		if err != nil {
			return err
		}

		if len(args) == 0 {
			return runServicesList(cmd, root, cfg)
		}
		return runServicesDetail(cmd, root, cfg, args[0])
	},
}

func runServicesList(cmd *cobra.Command, root string, cfg config.Config) error {
	names, err := services.ListNames(root, cfg.ServicesPath)
	if err != nil {
		return fmt.Errorf("listing services: %w", err)
	}

	if jsonFlag {
		if names == nil {
			names = []string{}
		}
		return printJSON(cmd, servicesListJSON{Services: names})
	}

	if len(names) == 0 {
		fmt.Println("No services found.")
		return nil
	}
	for _, n := range names {
		fmt.Println(n)
	}
	return nil
}

func runServicesDetail(cmd *cobra.Command, root string, cfg config.Config, input string) error {
	path, err := services.Resolve(root, cfg.ServicesPath, input)
	if err != nil {
		return coded(codeNotFound, err)
	}

	summary, err := services.Parse(path, root, cfg.ServicesPath)
	if err != nil {
		return fmt.Errorf("parsing service: %w", err)
	}
	for _, diagnostic := range summary.ParseErrors {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %s:%d: %s\n", path, diagnostic.Line, diagnostic.Message)
	}

	if jsonFlag {
		out := servicesJSON{
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

	fmt.Print(services.Format(summary, stdoutStyler()))
	return nil
}

func init() {
	rootCmd.AddCommand(servicesCmd)
}
