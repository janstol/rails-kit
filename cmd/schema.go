package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/schema"
)

var schemaCmd = &cobra.Command{
	Use:   "schema [table...]",
	Short: "Extract table definitions from schema.rb or structure.sql",
	Long: `Extract table definitions from db/schema.rb or db/structure.sql.

Both formats are supported. If schema.rb is not found, structure.sql is
tried automatically. The path can also be set via schema_path in .rails-kit.yml.

With no arguments, lists all table names.
With table names, prints the full table definition plus associated
indexes and foreign keys for each table.`,
	ValidArgsFunction: completeSchemaArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		root, cfg, err := loadConfig()
		if err != nil {
			return err
		}
		schemaPath := config.ResolveSchemaPath(root, cfg)

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
		fmt.Print(schema.Highlight(schemaPath, out, stdoutStyler()))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(schemaCmd)
}
