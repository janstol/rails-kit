package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var skillUninstallGlobal bool
var skillUninstallLocal bool
var skillUninstallTarget string

var skillUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the rails-kit skill for Claude Code or Codex",
	Long: `Remove the rails-kit skill for Claude Code, Codex, or both.

Uninstallation is global by default. Use --local to remove the skill from the
detected Rails root. Local uninstall validates that --root is a Rails app.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateSkillScope(skillUninstallLocal, skillUninstallGlobal); err != nil {
			return err
		}
		targets, err := skillTargets(skillUninstallTarget)
		if err != nil {
			return err
		}
		for _, target := range targets {
			targetDir, err := skillDir(target, skillUninstallLocal, skillUninstallLocal)
			if err != nil {
				return err
			}
			if err := uninstallSkill(targetDir); err != nil {
				return err
			}
		}
		return nil
	},
}

func uninstallSkill(targetDir string) error {
	if _, err := os.Stat(targetDir); err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("Skill not installed at %s\n", targetDir)
			return nil
		}
		return fmt.Errorf("checking skill directory: %w", err)
	}
	if err := os.RemoveAll(targetDir); err != nil {
		return fmt.Errorf("removing skill directory: %w", err)
	}
	fmt.Printf("Uninstalled %s\n", targetDir)
	return nil
}

func init() {
	skillUninstallCmd.Flags().BoolVar(&skillUninstallLocal, "local", false, "Uninstall from the Rails root instead of the user skill directory")
	skillUninstallCmd.Flags().BoolVar(&skillUninstallGlobal, "global", false, "Uninstall from the user skill directory (default)")
	skillUninstallCmd.Flags().StringVar(&skillUninstallTarget, "target", skillTargetClaude, "Skill target: claude, codex, or all")
	_ = skillUninstallCmd.Flags().MarkDeprecated("global", "global uninstallation is now the default")
	skillCmd.AddCommand(skillUninstallCmd)
}
