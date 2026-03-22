package schema

import (
	"regexp"
	"strings"
)

var (
	reSQLCreateTable = regexp.MustCompile(`(?i)^\s*CREATE\s+TABLE\s+(?:\w+\.)?("?\w+"?)\s*\(`)
	reSQLCreateIndex = regexp.MustCompile(`(?i)^\s*CREATE\s+(?:UNIQUE\s+)?INDEX\s+\S+\s+ON\s+(?:ONLY\s+)?(?:\w+\.)?("?\w+"?)[\s(]`)
	reSQLAlterTable  = regexp.MustCompile(`(?i)^\s*ALTER\s+TABLE\s+(?:ONLY\s+)?(?:\w+\.)?("?\w+"?)\s*$`)
	reSQLForeignKey  = regexp.MustCompile(`(?i)FOREIGN\s+KEY`)
	reSQLSemicolon   = regexp.MustCompile(`;\s*$`)
)

var sqlInternalTables = map[string]bool{
	"schema_migrations":    true,
	"ar_internal_metadata": true,
}

func isInternalTable(name string) bool {
	return sqlInternalTables[name]
}

func cleanSQLTableName(raw string) string {
	return strings.Trim(raw, `"`)
}

// parseSQLFile parses a PostgreSQL structure.sql file and returns a populated Schema.
func parseSQLFile(lines []string) *Schema {
	s := &Schema{
		tables:  make(map[string]bool),
		blocks:  parseSQLBlocks(lines),
		indexes: collectSQLIndexes(lines),
		fkeys:   collectSQLForeignKeys(lines),
	}
	for name := range s.blocks {
		s.tables[name] = true
	}
	return s
}

// parseSQLBlocks extracts CREATE TABLE ... ); blocks keyed by table name.
func parseSQLBlocks(lines []string) map[string][]string {
	blocks := make(map[string][]string)
	var current string
	skip := false

	for _, line := range lines {
		if current == "" && !skip {
			m := reSQLCreateTable.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			name := cleanSQLTableName(m[1])
			if isInternalTable(name) {
				skip = true
			} else {
				current = name
				blocks[current] = []string{line}
			}
			continue
		}

		trimmed := strings.TrimSpace(line)
		isEnd := strings.HasPrefix(trimmed, ")") && reSQLSemicolon.MatchString(trimmed)

		if skip {
			if isEnd {
				skip = false
			}
			continue
		}

		blocks[current] = append(blocks[current], line)
		if isEnd {
			current = ""
		}
	}
	return blocks
}

// collectSQLIndexes groups CREATE [UNIQUE] INDEX lines by table name.
func collectSQLIndexes(lines []string) map[string][]string {
	result := make(map[string][]string)
	for _, line := range lines {
		m := reSQLCreateIndex.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		name := cleanSQLTableName(m[1])
		if isInternalTable(name) {
			continue
		}
		result[name] = append(result[name], line)
	}
	return result
}

// collectSQLForeignKeys groups multi-line ALTER TABLE ... FOREIGN KEY statements by table name.
// In pg_dump output these are typically split across two lines:
//
//	ALTER TABLE ONLY tablename
//	    ADD CONSTRAINT name FOREIGN KEY (col) REFERENCES other(id);
func collectSQLForeignKeys(lines []string) map[string][]string {
	result := make(map[string][]string)
	var bufTable string
	var bufLines []string

	for _, line := range lines {
		if bufTable == "" {
			m := reSQLAlterTable.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			name := cleanSQLTableName(m[1])
			if isInternalTable(name) {
				// skip — but the statement continues on the next line; consume until ;
				bufTable = "\x00"
				bufLines = []string{line}
				continue
			}
			bufTable = name
			bufLines = []string{line}
			continue
		}

		bufLines = append(bufLines, line)

		if reSQLSemicolon.MatchString(line) {
			if bufTable != "\x00" {
				combined := strings.Join(bufLines, "")
				if reSQLForeignKey.MatchString(combined) {
					result[bufTable] = append(result[bufTable], combined)
				}
			}
			bufTable = ""
			bufLines = nil
		}
	}
	return result
}
