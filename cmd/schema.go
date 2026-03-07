package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/schema"
)

var schemaCmd = &cobra.Command{
	Use:   "schema [table...]",
	Short: "Extract table definitions from db/schema.rb",
	Long: `Extract create_table blocks from db/schema.rb.

With no arguments, lists all table names.
With table names, prints the full create_table block plus associated
indexes and foreign keys for each table.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, cfg, err := loadConfig()
		if err != nil {
			return err
		}
		schemaPath := config.ResolvePath(root, cfg.SchemaPath)

		if len(args) == 0 {
			tables, err := schema.ListTables(schemaPath)
			if err != nil {
				return fmt.Errorf("listing tables: %w", err)
			}
			if jsonFlag {
				return printJSON(tables)
			}
			for _, t := range tables {
				fmt.Println(t)
			}
			return nil
		}

		if jsonFlag {
			result, err := schema.ExtractTableMap(schemaPath, args)
			if err != nil {
				return fmt.Errorf("extracting schema: %w", err)
			}
			return printJSON(result)
		}

		out, err := schema.ExtractTables(schemaPath, args)
		if err != nil {
			return fmt.Errorf("extracting schema: %w", err)
		}
		fmt.Print(out)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(schemaCmd)
}
