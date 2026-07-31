package cmd

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/schema"
)

// schemaTableJSON is one entry of the schema command's data.tables array.
// Definition is the raw DDL block text (create_table + indexes + foreign
// keys), not structured columns — internal/schema.ExtractTableMap returns
// map[string]string, not a parsed column list. It is omitted in list mode.
type schemaTableJSON struct {
	Name       string `json:"name"`
	Definition string `json:"definition,omitempty"`
}

type schemaJSON struct {
	Tables []schemaTableJSON `json:"tables"`
}

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
				return coded(codeParseError, fmt.Errorf("listing tables: %w", err))
			}
			if jsonFlag {
				out := schemaJSON{Tables: make([]schemaTableJSON, len(tables))}
				for i, t := range tables {
					out.Tables[i] = schemaTableJSON{Name: t}
				}
				return printJSON(cmd, out)
			}
			for _, t := range tables {
				fmt.Println(t)
			}
			return nil
		}

		if jsonFlag {
			result, err := schema.ExtractTableMap(schemaPath, args)
			if err != nil {
				return coded(codeParseError, fmt.Errorf("extracting schema: %w", err))
			}
			names := make([]string, 0, len(result))
			for name := range result {
				names = append(names, name)
			}
			sort.Strings(names)
			out := schemaJSON{Tables: make([]schemaTableJSON, len(names))}
			for i, name := range names {
				out.Tables[i] = schemaTableJSON{Name: name, Definition: result[name]}
			}
			return printJSON(cmd, out)
		}

		out, err := schema.ExtractTables(schemaPath, args)
		if err != nil {
			return coded(codeParseError, fmt.Errorf("extracting schema: %w", err))
		}
		fmt.Print(schema.Highlight(schemaPath, out, stdoutStyler()))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(schemaCmd)
}
