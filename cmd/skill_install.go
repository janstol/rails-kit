package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/janstol/rails-kit/internal/skill"
)

var skillInstallGlobal bool
var skillInstallLocal bool
var skillInstallTarget string

var skillInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the rails-kit skill for Claude Code or Codex",
	Long: `Install the rails-kit skill for Claude Code, Codex, or both.

Installation is global by default. Use --local to install into the detected
Rails root, or the directory supplied with --root. The default target is
Claude Code; use --target codex or --target all for Codex support.

When --root is set, install writes to that directory even if it is not a Rails app.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateSkillScope(skillInstallLocal, skillInstallGlobal); err != nil {
			return err
		}
		targets, err := skillTargets(skillInstallTarget)
		if err != nil {
			return err
		}
		for _, target := range targets {
			targetDir, err := skillDir(target, skillInstallLocal, false)
			if err != nil {
				return err
			}
			if err := installSkill(target, targetDir); err != nil {
				return err
			}
		}
		return nil
	},
}

func installSkill(target, targetDir string) error {
	skillFile := filepath.Join(targetDir, "SKILL.md")
	_, statErr := os.Stat(skillFile)
	existed := statErr == nil

	content := skill.Content
	if target == skillTargetCodex {
		var err error
		content, err = skill.CodexContent()
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("creating skill directory: %w", err)
	}
	if err := os.WriteFile(skillFile, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing skill file: %w", err)
	}
	if target == skillTargetCodex {
		metadataFile := filepath.Join(targetDir, "agents", "openai.yaml")
		if err := os.MkdirAll(filepath.Dir(metadataFile), 0o755); err != nil {
			return fmt.Errorf("creating Codex metadata directory: %w", err)
		}
		if err := os.WriteFile(metadataFile, []byte(skill.OpenAIMetadata), 0o644); err != nil {
			return fmt.Errorf("writing Codex metadata: %w", err)
		}
	}

	if existed {
		fmt.Printf("Updated %s\n", skillFile)
	} else {
		fmt.Printf("Installed %s\n", skillFile)
	}
	return nil
}

func init() {
	skillInstallCmd.Flags().BoolVar(&skillInstallLocal, "local", false, "Install into the Rails root instead of the user skill directory")
	skillInstallCmd.Flags().BoolVar(&skillInstallGlobal, "global", false, "Install into the user skill directory (default)")
	skillInstallCmd.Flags().StringVar(&skillInstallTarget, "target", skillTargetClaude, "Skill target: claude, codex, or all")
	_ = skillInstallCmd.Flags().MarkDeprecated("global", "global installation is now the default")
	skillCmd.AddCommand(skillInstallCmd)
}
