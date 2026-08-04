package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/mailers"
)

type mailersListJSON struct {
	Mailers []string `json:"mailers"`
}

type mailersJSON struct {
	ClassName   string   `json:"class_name"`
	ParentClass string   `json:"parent_class,omitempty"`
	RelPath     string   `json:"rel_path"`
	Concerns    []string `json:"concerns,omitempty"`
	Default     []string `json:"default,omitempty"`
	Layout      string   `json:"layout,omitempty"`
	Attachments []string `json:"attachments,omitempty"`
	Methods     []string `json:"methods,omitempty"`
}

var mailersCmd = &cobra.Command{
	Use:   "mailers [name]",
	Short: "List or inspect Rails mailers",
	Long: `Show a structural summary of a Rails mailer file.

With no arguments, lists available mailers.
With a mailer name, extracts default headers, layout, included concerns,
attachments (regular and inline), and public action methods.

The name can be a resource name (users, admin/notification), the mailer
file's basename (user_mailer), a CamelCase class name (UserMailer,
Admin::NotificationMailer), or a file path ending in .rb.

Only the mailer's own file is parsed -- defaults and other declarations
inherited from a superclass (ApplicationMailer, etc.) are not resolved.
See parent_class for where to look next.`,
	Args:              cobra.MaximumNArgs(1),
	ValidArgsFunction: completeMailerNames,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, cfg, err := loadConfig()
		if err != nil {
			return err
		}

		if len(args) == 0 {
			return runMailersList(cmd, root, cfg)
		}
		return runMailersDetail(cmd, root, cfg, args[0])
	},
}

func runMailersList(cmd *cobra.Command, root string, cfg config.Config) error {
	names, err := mailers.ListNames(root, cfg.MailersPath)
	if err != nil {
		return fmt.Errorf("listing mailers: %w", err)
	}

	if jsonFlag {
		if names == nil {
			names = []string{}
		}
		return printJSON(cmd, mailersListJSON{Mailers: names})
	}

	if len(names) == 0 {
		fmt.Println("No mailers found.")
		return nil
	}
	for _, n := range names {
		fmt.Println(n)
	}
	return nil
}

func runMailersDetail(cmd *cobra.Command, root string, cfg config.Config, input string) error {
	path, err := mailers.Resolve(root, cfg.MailersPath, input)
	if err != nil {
		return coded(codeNotFound, err)
	}

	summary, err := mailers.Parse(path, root, cfg.MailersPath)
	if err != nil {
		return fmt.Errorf("parsing mailer: %w", err)
	}
	for _, diagnostic := range summary.ParseErrors {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %s:%d: %s\n", path, diagnostic.Line, diagnostic.Message)
	}

	if jsonFlag {
		out := mailersJSON{
			ClassName:   summary.ClassName,
			ParentClass: summary.ParentClass,
			RelPath:     summary.RelPath,
			Concerns:    trimEntries(summary.Concerns),
			Default:     trimEntries(summary.Default),
			Layout:      summary.Layout,
			Attachments: trimEntries(summary.Attachments),
			Methods:     trimEntries(summary.Methods),
		}
		return printJSON(cmd, out)
	}

	fmt.Print(mailers.Format(summary, stdoutStyler()))
	return nil
}

func init() {
	rootCmd.AddCommand(mailersCmd)
}
