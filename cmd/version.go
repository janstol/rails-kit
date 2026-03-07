package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/janstol/rails-kit/internal/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		v, commit, buildDate := version.Info()
		if jsonFlag {
			return printJSON(struct {
				Version   string `json:"version"`
				Commit    string `json:"commit"`
				BuildDate string `json:"build_date"`
			}{v, commit, buildDate})
		}
		fmt.Printf("rails-kit version %s (commit: %s, built: %s)\n", v, commit, buildDate)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
