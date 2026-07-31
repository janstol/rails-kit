package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/janstol/rails-kit/internal/about"
)

var aboutRuntime bool
var aboutRunner = about.Runner{}

var aboutCmd = &cobra.Command{
	Use:   "about",
	Short: "Summarize the Rails project environment",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, cfg, err := loadConfig()
		if err != nil {
			return err
		}
		report := about.Inspect(root, cfg)
		if aboutRuntime {
			info, runtimeErr := aboutRunner.Inspect(cmd.Context(), root)
			if runtimeErr != nil {
				report.Warnings = append(report.Warnings, "runtime inspection failed: "+runtimeErr.Error())
			} else {
				report = about.Enrich(report, info)
			}
		}
		if jsonFlag {
			return printJSON(cmd, report)
		}
		_, err = fmt.Fprint(cmd.OutOrStdout(), about.Format(report))
		return err
	},
}

func init() {
	rootCmd.AddCommand(aboutCmd)
	aboutCmd.Flags().BoolVar(&aboutRuntime, "runtime", false, "Boot Rails to report active runtime values")
}
