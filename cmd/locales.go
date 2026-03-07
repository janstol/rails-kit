package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/locales"
)

type localesJSON struct {
	Scope string `json:"scope"`
	Value any    `json:"value"`
}

var localesCmd = &cobra.Command{
	Use:   "locales [scope]",
	Short: "Extract locale keys by scope",
	Long: `Browse and extract locale keys from config/locales/*.yml.

With no arguments, lists nested scopes (e.g., en.views.users, en.time.formats).
With a scope like en.views.users, prints all keys under that scope.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, cfg, err := loadConfig()
		if err != nil {
			return err
		}
		localesDir := config.ResolvePath(root, cfg.LocalesPath)

		merged, err := locales.Load(localesDir)
		if err != nil {
			return fmt.Errorf("loading locales: %w", err)
		}

		if len(args) == 0 {
			scopes := locales.ListScopes(merged)
			if jsonFlag {
				return printJSON(scopes)
			}
			for _, scope := range scopes {
				fmt.Println(scope)
			}
			return nil
		}

		scope := args[0]
		node, err := locales.Navigate(merged, scope)
		if err != nil {
			return err
		}
		if jsonFlag {
			return printJSON(localesJSON{
				Scope: scope,
				Value: node,
			})
		}

		fmt.Println("# " + scope)
		var sb strings.Builder
		locales.PrintTree(&sb, node, 0)
		fmt.Print(sb.String())
		return nil
	},
}

func init() {
	rootCmd.AddCommand(localesCmd)
}
