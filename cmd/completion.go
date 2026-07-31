package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for rails-kit.

Source the output to enable tab completion in your shell:

  # Bash
  rails-kit completion bash > /etc/bash_completion.d/rails-kit

  # Zsh
  rails-kit completion zsh > "${fpath[1]}/_rails-kit"

  # Fish
  rails-kit completion fish > ~/.config/fish/completions/rails-kit.fish

Completions are dynamic: commands that take a model, table, locale scope,
concern, fixture, or gem name read the current Rails project (honoring
--root and .rails-kit.yml) to offer real values, not just flag names.
Run from outside a Rails root, or if the project can't be read, completion
falls back to no suggestions rather than an error.`,
}

var completionBashCmd = &cobra.Command{
	Use:   "bash",
	Short: "Generate bash completion script",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return rootCmd.GenBashCompletionV2(os.Stdout, true)
	},
}

var completionZshCmd = &cobra.Command{
	Use:   "zsh",
	Short: "Generate zsh completion script",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return rootCmd.GenZshCompletion(os.Stdout)
	},
}

var completionFishCmd = &cobra.Command{
	Use:   "fish",
	Short: "Generate fish completion script",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return rootCmd.GenFishCompletion(os.Stdout, true)
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
	completionCmd.AddCommand(completionBashCmd)
	completionCmd.AddCommand(completionZshCmd)
	completionCmd.AddCommand(completionFishCmd)
}
