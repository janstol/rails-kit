package cmd

import "github.com/spf13/cobra"

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Install or remove the rails-kit Claude Code skill",
}

func init() {
	rootCmd.AddCommand(skillCmd)
}
