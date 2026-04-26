package prism

import (
	"fmt"
	"strings"
)

// Format renders a compact text skeleton.
func Format(file File) string {
	var sb strings.Builder
	title := file.RelPath
	if title == "" {
		title = file.Path
	}
	sb.WriteString(title + "\n")
	sb.WriteString(strings.Repeat("=", 40) + "\n")

	writeParseErrors(&sb, file.ParseErrors)
	writeConstants(&sb, "", file.Constants)
	writeCalls(&sb, "", "Macros", file.Calls)
	writeMethods(&sb, "", file.Methods)
	for _, module := range file.Modules {
		writeModule(&sb, "", module)
	}
	for _, class := range file.Classes {
		writeClass(&sb, "", class)
	}
	sb.WriteString("\n")
	return sb.String()
}

func writeClass(sb *strings.Builder, indent string, class Class) {
	header := fmt.Sprintf("%sClass %s", indent, class.Name)
	if class.Parent != "" {
		header += " < " + class.Parent
	}
	header += lineSuffix(class.StartLine, class.EndLine)
	writeBlockHeader(sb, header)
	writeCallGroups(sb, indent+"  ", class.Includes, class.Extends, class.Prepends)
	writeConstants(sb, indent+"  ", class.Constants)
	writeCalls(sb, indent+"  ", "Macros", class.Calls)
	writeMethods(sb, indent+"  ", class.Methods)
	for _, module := range class.Modules {
		writeModule(sb, indent+"  ", module)
	}
	for _, child := range class.Classes {
		writeClass(sb, indent+"  ", child)
	}
}

func writeModule(sb *strings.Builder, indent string, module Module) {
	writeBlockHeader(sb, fmt.Sprintf("%sModule %s%s", indent, module.Name, lineSuffix(module.StartLine, module.EndLine)))
	writeCallGroups(sb, indent+"  ", module.Includes, module.Extends, module.Prepends)
	writeConstants(sb, indent+"  ", module.Constants)
	writeCalls(sb, indent+"  ", "Macros", module.Calls)
	writeMethods(sb, indent+"  ", module.Methods)
	for _, child := range module.Modules {
		writeModule(sb, indent+"  ", child)
	}
	for _, class := range module.Classes {
		writeClass(sb, indent+"  ", class)
	}
}

func writeParseErrors(sb *strings.Builder, errors []string) {
	if len(errors) == 0 {
		return
	}
	writeBlockHeader(sb, "Parse errors")
	for _, err := range errors {
		sb.WriteString("  " + err + "\n")
	}
}

func writeCallGroups(sb *strings.Builder, indent string, includes, extends, prepends []Call) {
	writeCalls(sb, indent, "Includes", includes)
	writeCalls(sb, indent, "Extends", extends)
	writeCalls(sb, indent, "Prepends", prepends)
}

func writeConstants(sb *strings.Builder, indent string, constants []Constant) {
	if len(constants) == 0 {
		return
	}
	writeBlockHeader(sb, indent+"Constants")
	for _, c := range constants {
		fmt.Fprintf(sb, "%s  %s%s\n", indent, c.Source, lineSuffix(c.StartLine, c.EndLine))
	}
}

func writeCalls(sb *strings.Builder, indent, label string, calls []Call) {
	if len(calls) == 0 {
		return
	}
	writeBlockHeader(sb, indent+label)
	for _, call := range calls {
		fmt.Fprintf(sb, "%s  %s%s\n", indent, call.Source, lineSuffix(call.StartLine, call.EndLine))
	}
}

func writeMethods(sb *strings.Builder, indent string, methods []Method) {
	if len(methods) == 0 {
		return
	}
	writeBlockHeader(sb, indent+"Methods")
	for _, method := range methods {
		params := method.Params
		if params != "" {
			params = "(" + params + ")"
		}
		fmt.Fprintf(sb, "%s  %s def %s%s%s\n", indent, method.Visibility, method.Name, params, lineSuffix(method.StartLine, method.EndLine))
	}
}

func writeBlockHeader(sb *strings.Builder, header string) {
	if sb.Len() > 0 && !strings.HasSuffix(sb.String(), "\n\n") {
		sb.WriteString("\n")
	}
	sb.WriteString(header + ":\n")
}

func lineSuffix(start, end int) string {
	if start == 0 {
		return ""
	}
	if end == 0 || end == start {
		return fmt.Sprintf(" (line %d)", start)
	}
	return fmt.Sprintf(" (lines %d-%d)", start, end)
}
