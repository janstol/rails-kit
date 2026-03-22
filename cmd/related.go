package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/janstol/rails-kit/internal/pluralize"
	"github.com/janstol/rails-kit/internal/related"
)

type relatedJSON struct {
	Model      string           `json:"model"`
	Plural     string           `json:"plural"`
	Categories []relatedCatJSON `json:"categories"`
}

type relatedCatJSON struct {
	Label string   `json:"label"`
	Files []string `json:"files"`
}

var relatedCmd = &cobra.Command{
	Use:   "related <name>",
	Short: "List all files related to a model",
	Long: `Find all files related to a model: model, controller, views, decorator,
job, mailer, former, service, datagrid, tests, and fixtures.

The name can be a model name (user, order_item) or a related file path
(model, controller, view, decorator, job, mailer, former, service, datagrid, test/spec, fixture).
Matches stay within the exact namespace of the requested model.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root, cfg, err := loadConfig()
		if err != nil {
			return err
		}

		p := pluralize.New(cfg.Plurals)
		name, plural, err := related.ResolveLookup(root, cfg, args[0], p)
		if err != nil {
			return err
		}

		cats, err := related.Find(root, cfg, name, plural)
		if err != nil {
			return err
		}
		if len(cats) == 0 {
			return fmt.Errorf("no files found for '%s'", name)
		}

		if jsonFlag {
			out := relatedJSON{
				Model:  name,
				Plural: plural,
			}
			for _, cat := range cats {
				out.Categories = append(out.Categories, relatedCatJSON{
					Label: cat.Label,
					Files: cat.Files,
				})
			}
			return printJSON(out)
		}

		for _, cat := range cats {
			fmt.Printf("%s:\n", cat.Label)
			for _, f := range cat.Files {
				fmt.Printf("  %s\n", f)
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(relatedCmd)
}
