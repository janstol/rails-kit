package schema

import "strings"

// Styler is the minimal styling surface Highlight needs. term.Styler
// satisfies this structurally, so this package does not need to import
// internal/term.
type Styler interface {
	Enabled() bool
	Bold(string) string
	Cyan(string) string
}

// Highlight adds terminal styling to the text output of ExtractTables.
// It is a no-op for .sql schemas and for a disabled Styler.
func Highlight(schemaPath, out string, st Styler) string {
	if !st.Enabled() || strings.HasSuffix(schemaPath, ".sql") {
		return out
	}
	lines := strings.SplitAfter(out, "\n")
	for i, line := range lines {
		lines[i] = highlightLine(line, st)
	}
	return strings.Join(lines, "")
}

// highlightLine styles the DDL keyword and table name of a single line,
// reusing the package's existing schema.rb regexes. Lines that don't match
// any of them (index/fkey continuation lines, blank lines, column
// definitions) pass through unchanged.
func highlightLine(line string, st Styler) string {
	if loc := reCreateTable.FindStringSubmatchIndex(line); loc != nil {
		return styleSpan(line, loc[0], loc[1], "create_table", loc[2], loc[3], st)
	}
	if loc := reCreateJoinTable.FindStringIndex(line); loc != nil {
		return styleSpan(line, loc[0], loc[1], "create_join_table", -1, -1, st)
	}
	if loc := reAddIndex.FindStringSubmatchIndex(line); loc != nil {
		return styleSpan(line, loc[0], loc[1], "add_index", loc[2], loc[3], st)
	}
	if loc := reAddForeignKey.FindStringSubmatchIndex(line); loc != nil {
		return styleSpan(line, loc[0], loc[1], "add_foreign_key", loc[2], loc[3], st)
	}
	return line
}

// styleSpan colors keyword (found within line[wholeStart:wholeEnd]) cyan and,
// when nameStart >= 0, colors line[nameStart:nameEnd] bold, leaving every
// other byte of line untouched.
func styleSpan(line string, wholeStart, wholeEnd int, keyword string, nameStart, nameEnd int, st Styler) string {
	kwStart := wholeStart + strings.Index(line[wholeStart:wholeEnd], keyword)
	kwEnd := kwStart + len(keyword)

	var b strings.Builder
	b.WriteString(line[:kwStart])
	b.WriteString(st.Cyan(keyword))
	if nameStart >= 0 {
		b.WriteString(line[kwEnd:nameStart])
		b.WriteString(st.Bold(line[nameStart:nameEnd]))
		b.WriteString(line[nameEnd:])
	} else {
		b.WriteString(line[kwEnd:])
	}
	return b.String()
}
