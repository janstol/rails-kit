package schema

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
)

var (
	reCreateTable     = regexp.MustCompile(`^\s*create_table\s+"([^"]+)"`)
	reCreateJoinTable = regexp.MustCompile(`^\s*create_join_table\s+:(\w+),\s*:(\w+)`)
	reJoinTableName   = regexp.MustCompile(`table_name:\s*(?::(\w+)|"([^"]+)"|'([^']+)')`)
	reAddIndex        = regexp.MustCompile(`^\s*add_index\s+"([^"]+)"`)
	reAddForeignKey   = regexp.MustCompile(`^\s*add_foreign_key\s+"([^"]+)"`)
)

// JoinTableName returns the canonical join table name for two tables (alphabetically sorted).
func JoinTableName(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + "_" + b
}

// Schema holds the parsed contents of a schema.rb file.
type Schema struct {
	tables  map[string]bool
	blocks  map[string][]string
	indexes map[string][]string
	fkeys   map[string][]string
}

// Parse reads and parses the schema file at schemaPath.
// Both db/schema.rb (Ruby DSL) and db/structure.sql (PostgreSQL DDL) are supported;
// the format is detected by file extension.
func Parse(schemaPath string) (*Schema, error) {
	lines, err := readLines(schemaPath)
	if err != nil {
		return nil, err
	}

	if strings.HasSuffix(schemaPath, ".sql") {
		return parseSQLFile(lines), nil
	}

	s := &Schema{
		tables:  make(map[string]bool),
		blocks:  parseBlocks(lines),
		indexes: collectLines(lines, reAddIndex),
		fkeys:   collectLines(lines, reAddForeignKey),
	}
	for _, line := range lines {
		if m := reCreateTable.FindStringSubmatch(line); m != nil {
			s.tables[m[1]] = true
		} else if tableName, ok := createJoinTableName(line); ok {
			s.tables[tableName] = true
		}
	}
	return s, nil
}

// ListTables returns all table names found in the schema, sorted.
func (s *Schema) ListTables() []string {
	tables := make([]string, 0, len(s.tables))
	for name := range s.tables {
		tables = append(tables, name)
	}
	sort.Strings(tables)
	return tables
}

// ExtractTables returns the create_table blocks plus associated indexes and
// foreign keys for the requested tables. Unknown table names are returned as errors.
func (s *Schema) ExtractTables(names []string) (string, error) {
	if err := s.checkUnknown(names); err != nil {
		return "", err
	}
	var sb strings.Builder
	for i, name := range names {
		if i > 0 {
			sb.WriteString("\n")
		}
		s.writeBlock(&sb, name)
	}
	return sb.String(), nil
}

// ExtractTableMap returns a map of table name to its schema block string
// (create_table block + associated indexes + foreign keys) for the requested tables.
// Unknown table names are returned as an error.
func (s *Schema) ExtractTableMap(names []string) (map[string]string, error) {
	if err := s.checkUnknown(names); err != nil {
		return nil, err
	}
	result := make(map[string]string, len(names))
	for _, name := range names {
		var sb strings.Builder
		s.writeBlock(&sb, name)
		result[name] = sb.String()
	}
	return result, nil
}

func (s *Schema) checkUnknown(names []string) error {
	var unknown []string
	for _, name := range names {
		if !s.tables[name] {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) > 0 {
		return fmt.Errorf("table(s) not found in schema: %s", strings.Join(unknown, ", "))
	}
	return nil
}

func (s *Schema) writeBlock(sb *strings.Builder, name string) {
	if block, ok := s.blocks[name]; ok {
		sb.WriteString(strings.Join(block, ""))
	}
	if idxLines := s.indexes[name]; len(idxLines) > 0 {
		sb.WriteString("\n")
		sb.WriteString(strings.Join(idxLines, ""))
	}
	if fkLines := s.fkeys[name]; len(fkLines) > 0 {
		sb.WriteString("\n")
		sb.WriteString(strings.Join(fkLines, ""))
	}
}

// ListTables returns all table names found in the schema file, sorted.
func ListTables(schemaPath string) ([]string, error) {
	s, err := Parse(schemaPath)
	if err != nil {
		return nil, err
	}
	return s.ListTables(), nil
}

// ExtractTables returns the create_table blocks plus associated indexes and
// foreign keys for the requested tables. Unknown table names are returned as errors.
func ExtractTables(schemaPath string, names []string) (string, error) {
	s, err := Parse(schemaPath)
	if err != nil {
		return "", err
	}
	return s.ExtractTables(names)
}

// ExtractTableMap returns a map of table name to its schema block string
// (create_table block + associated indexes + foreign keys) for the requested tables.
// Unknown table names are returned as an error.
func ExtractTableMap(schemaPath string, names []string) (map[string]string, error) {
	s, err := Parse(schemaPath)
	if err != nil {
		return nil, err
	}
	return s.ExtractTableMap(names)
}

// parseBlocks extracts create_table...end blocks keyed by table name.
// NOTE: This function uses a simple depth counter based on 'do' and 'end'
// keywords. It may fail to correctly parse blocks that contain nested 'do'/'end'
// structures (e.g., case statements) that are not part of the create_table
// block's own structure. This is a known trade-off for parsing speed and
// simplicity.
func parseBlocks(lines []string) map[string][]string {
	blocks := make(map[string][]string)
	var currentTable string
	depth := 0

	for _, line := range lines {
		if currentTable == "" {
			if m := reCreateTable.FindStringSubmatch(line); m != nil {
				currentTable = m[1]
				depth = 1
				blocks[currentTable] = []string{line}
			} else if tableName, ok := createJoinTableName(line); ok {
				currentTable = tableName
				depth = 1
				blocks[currentTable] = []string{line}
			}
			continue
		}
		blocks[currentTable] = append(blocks[currentTable], line)
		depth += blockDepthDelta(line)
		if depth <= 0 {
			currentTable = ""
			depth = 0
		}
	}
	return blocks
}

// collectLines groups lines by table name using the given regexp (must have table name in group 1).
func collectLines(lines []string, re *regexp.Regexp) map[string][]string {
	result := make(map[string][]string)
	for _, line := range lines {
		if m := re.FindStringSubmatch(line); m != nil {
			result[m[1]] = append(result[m[1]], line)
		}
	}
	return result
}

func readLines(path string) (lines []string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open schema: %w", err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	reader := bufio.NewReader(f)
	for {
		line, readErr := reader.ReadString('\n')
		if len(line) > 0 {
			lines = append(lines, line)
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return lines, readErr
		}
	}
	return lines, nil
}

func blockDepthDelta(line string) int {
	sanitized := stripRubyStringsAndComments(line)
	fields := bytes.Fields([]byte(sanitized))
	delta := 0
	for _, field := range fields {
		switch string(field) {
		case "do":
			delta++
		case "end":
			delta--
		}
	}
	return delta
}

func stripRubyStringsAndComments(line string) string {
	var b strings.Builder
	b.Grow(len(line))

	inSingle := false
	inDouble := false
	escaped := false

	for i := 0; i < len(line); i++ {
		ch := line[i]

		switch {
		case inSingle:
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '\'' {
				inSingle = false
			}
		case inDouble:
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inDouble = false
			}
		default:
			switch ch {
			case '#':
				return b.String()
			case '\'':
				inSingle = true
			case '"':
				inDouble = true
			default:
				if isRubyWordByte(ch) {
					b.WriteByte(ch)
				} else {
					b.WriteByte(' ')
				}
			}
		}
	}

	return b.String()
}

func isRubyWordByte(ch byte) bool {
	return ch == '_' ||
		(ch >= 'a' && ch <= 'z') ||
		(ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9')
}

func createJoinTableName(line string) (string, bool) {
	m := reCreateJoinTable.FindStringSubmatch(line)
	if m == nil {
		return "", false
	}
	if tm := reJoinTableName.FindStringSubmatch(line); tm != nil {
		for i := 1; i < len(tm); i++ {
			if tm[i] != "" {
				return tm[i], true
			}
		}
	}
	return JoinTableName(m[1], m[2]), true
}
