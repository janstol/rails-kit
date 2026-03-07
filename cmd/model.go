package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/janstol/rails-kit/internal/model"
)

type modelJSON struct {
	ClassName    string   `json:"class_name"`
	ParentClass  string   `json:"parent_class,omitempty"`
	RelPath      string   `json:"rel_path"`
	TableName    string   `json:"table_name,omitempty"`
	Concerns     []string `json:"concerns,omitempty"`
	Associations []string `json:"associations,omitempty"`
	Validations  []string `json:"validations,omitempty"`
	Scopes       []string `json:"scopes,omitempty"`
	Callbacks    []string `json:"callbacks,omitempty"`
	Enums        []string `json:"enums,omitempty"`
	Delegates    []string `json:"delegates,omitempty"`
}

func trimEntries(entries []string) []string {
	if len(entries) == 0 {
		return nil
	}
	result := make([]string, len(entries))
	for i, e := range entries {
		result[i] = strings.TrimSpace(e)
	}
	return result
}

var modelCmd = &cobra.Command{
	Use:   "model <name>",
	Short: "Compact model structure summary",
	Long: `Show a compact structural summary of a Rails model file.

Extracts: concerns, associations, validations, scopes, callbacks, enums, delegates.

The name can be a model name (user, order_item) or a file path ending in .rb.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, cfg, err := loadConfig()
		if err != nil {
			return err
		}

		modelPath, err := model.Resolve(root, cfg.ModelsPath, args[0])
		if err != nil {
			return fmt.Errorf("resolving model: %w", err)
		}

		summary, err := model.Parse(modelPath, root, cfg.ModelsPath)
		if err != nil {
			return fmt.Errorf("parsing model: %w", err)
		}

		if jsonFlag {
			out := modelJSON{
				ClassName:    summary.ClassName,
				ParentClass:  summary.ParentClass,
				RelPath:      summary.RelPath,
				TableName:    summary.TableName,
				Concerns:     trimEntries(summary.Concerns),
				Associations: trimEntries(summary.Assocs),
				Validations:  trimEntries(summary.Valids),
				Scopes:       trimEntries(summary.Scopes),
				Callbacks:    trimEntries(summary.Callbacks),
				Enums:        trimEntries(summary.Enums),
				Delegates:    trimEntries(summary.Delegates),
			}
			return printJSON(out)
		}

		fmt.Print(model.Format(summary))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(modelCmd)
}
