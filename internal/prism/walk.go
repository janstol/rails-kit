package prism

import (
	"bytes"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
)

// railsMacros mirrors the Rails DSL macro allowlist previously maintained in
// skeleton.rb, so calls like has_many/validates/scope are tagged distinctly
// from arbitrary bare-receiver calls.
var railsMacros = map[string]bool{
	"belongs_to": true, "has_and_belongs_to_many": true, "has_many": true, "has_one": true,
	"validates": true, "validate": true, "validates_absence_of": true, "validates_acceptance_of": true,
	"validates_confirmation_of": true, "validates_exclusion_of": true, "validates_format_of": true,
	"validates_inclusion_of": true, "validates_length_of": true, "validates_numericality_of": true,
	"validates_presence_of": true, "validates_size_of": true, "validates_uniqueness_of": true,
	"scope": true, "before_validation": true, "after_validation": true, "before_create": true, "after_create": true, "after_create_commit": true,
	"before_update": true, "after_update": true, "after_update_commit": true, "before_save": true, "after_save": true, "after_save_commit": true,
	"before_destroy": true, "after_destroy": true, "after_destroy_commit": true,
	"after_initialize": true, "before_commit": true, "after_commit": true, "after_rollback": true, "after_touch": true,
	"after_find": true, "around_validation": true, "around_create": true, "around_update": true, "around_save": true,
	"around_destroy": true, "enum": true, "delegate": true,
}

func isVisibilityCall(name string) bool {
	return name == "public" || name == "protected" || name == "private"
}

// container accumulates the structural children found while walking a body
// scope (a file, class, or module), before being flattened into the
// exported File/Class/Module structs.
type container struct {
	classes   []Class
	modules   []Module
	constants []Constant
	calls     []Call
	methods   []Method
	includes  []Call
	extends   []Call
	prepends  []Call
}

type astWalker struct {
	src []byte
}

func buildFile(path string, src []byte, result *parser.ParseResult) File {
	file := File{Path: path}
	for _, e := range result.Errors {
		file.ParseErrors = append(file.ParseErrors, e.Message)
	}
	if result.Value != nil {
		w := &astWalker{src: src}
		var c container
		w.walkBody(w.bodyNodes(result.Value), &c)
		file.Classes = c.classes
		file.Modules = c.modules
		file.Constants = c.constants
		file.Calls = c.calls
		file.Methods = c.methods
	}
	return file
}

func (w *astWalker) walkBody(nodes []parser.Node, c *container) {
	visibility := "public"
	for _, node := range nodes {
		switch n := node.(type) {
		case *parser.ClassNode:
			c.classes = append(c.classes, w.buildClass(n))
		case *parser.ModuleNode:
			c.modules = append(c.modules, w.buildModule(n))
		case *parser.ConstantWriteNode:
			c.constants = append(c.constants, w.constantEntry(n))
		case *parser.DefNode:
			c.methods = append(c.methods, w.methodEntry(n, visibility))
		case *parser.CallNode:
			switch {
			case isVisibilityCall(n.Name) && n.Arguments == nil:
				visibility = n.Name
			case n.Name == "include":
				c.includes = append(c.includes, w.callEntry(n, "include"))
			case n.Name == "extend":
				c.extends = append(c.extends, w.callEntry(n, "extend"))
			case n.Name == "prepend":
				c.prepends = append(c.prepends, w.callEntry(n, "prepend"))
			case railsMacros[n.Name]:
				c.calls = append(c.calls, w.callEntry(n, "rails_macro"))
			case n.Receiver == nil && !strings.HasSuffix(n.Name, "="):
				c.calls = append(c.calls, w.callEntry(n, "macro"))
			}
		}
	}
}

func (w *astWalker) buildClass(n *parser.ClassNode) Class {
	start, end := w.lineRange(n.Location)
	class := Class{
		Name:      w.nameFor(n.ConstantPath),
		Parent:    w.nameFor(n.Superclass),
		StartLine: start,
		EndLine:   end,
	}
	var c container
	w.walkBody(w.bodyNodes(n), &c)
	class.Includes = c.includes
	class.Extends = c.extends
	class.Prepends = c.prepends
	class.Classes = c.classes
	class.Modules = c.modules
	class.Constants = c.constants
	class.Calls = c.calls
	class.Methods = c.methods
	return class
}

func (w *astWalker) buildModule(n *parser.ModuleNode) Module {
	start, end := w.lineRange(n.Location)
	module := Module{
		Name:      w.nameFor(n.ConstantPath),
		StartLine: start,
		EndLine:   end,
	}
	var c container
	w.walkBody(w.bodyNodes(n), &c)
	module.Includes = c.includes
	module.Extends = c.extends
	module.Prepends = c.prepends
	module.Classes = c.classes
	module.Modules = c.modules
	module.Constants = c.constants
	module.Calls = c.calls
	module.Methods = c.methods
	return module
}

func (w *astWalker) constantEntry(n *parser.ConstantWriteNode) Constant {
	start, end := w.lineRange(n.Location)
	return Constant{
		Name:      n.Name,
		Source:    w.slice(n.Location),
		StartLine: start,
		EndLine:   end,
	}
}

func (w *astWalker) methodEntry(n *parser.DefNode, visibility string) Method {
	start, end := w.lineRange(n.Location)
	params := ""
	if n.Parameters != nil {
		params = w.slice(n.Parameters.Location)
	}
	return Method{
		Name:       n.Name,
		Params:     params,
		Visibility: visibility,
		StartLine:  start,
		EndLine:    end,
	}
}

func (w *astWalker) callEntry(n *parser.CallNode, kind string) Call {
	start, end := w.lineRange(n.Location)
	return Call{
		Name:      n.Name,
		Kind:      kind,
		Args:      w.argSlices(n),
		Source:    w.callSource(n),
		StartLine: start,
		EndLine:   end,
	}
}

func (w *astWalker) argSlices(n *parser.CallNode) []string {
	if n.Arguments == nil {
		return nil
	}
	args := make([]string, len(n.Arguments.Arguments))
	for i, a := range n.Arguments.Arguments {
		args[i] = w.slice(a.GetLocation())
	}
	return args
}

// callSource returns the full call source, unless it has a block and spans
// multiple lines, in which case only the first line is kept so a call's
// block body doesn't bloat the compact "source" field.
func (w *astWalker) callSource(n *parser.CallNode) string {
	source := w.slice(n.Location)
	if n.Block == nil {
		return source
	}
	if idx := strings.IndexByte(source, '\n'); idx >= 0 {
		return strings.TrimRight(source[:idx], "\r")
	}
	return source
}

// bodyNodes returns the top-level statements of a scope, matching
// skeleton.rb's body_nodes: only a plain StatementsNode body is walked, so a
// class/module body wrapped in a BeginNode (e.g. one with a top-level
// rescue) yields no children, same as before.
func (w *astWalker) bodyNodes(n parser.Node) []parser.Node {
	switch v := n.(type) {
	case *parser.ProgramNode:
		if v.Statements == nil {
			return nil
		}
		return v.Statements.Body
	case *parser.ClassNode:
		if st, ok := v.Body.(*parser.StatementsNode); ok {
			return st.Body
		}
	case *parser.ModuleNode:
		if st, ok := v.Body.(*parser.StatementsNode); ok {
			return st.Body
		}
	}
	return nil
}

func (w *astWalker) nameFor(n parser.Node) string {
	if n == nil {
		return ""
	}
	return w.slice(n.GetLocation())
}

func (w *astWalker) slice(loc parser.Location) string {
	start, end := loc.StartOffset, loc.StartOffset+loc.Length
	if end > len(w.src) {
		end = len(w.src)
	}
	if start < 0 || start > end {
		return ""
	}
	return string(w.src[start:end])
}

func (w *astWalker) lineRange(loc parser.Location) (int, int) {
	return w.lineAt(loc.StartOffset), w.lineAt(loc.StartOffset + loc.Length)
}

func (w *astWalker) lineAt(offset int) int {
	if offset > len(w.src) {
		offset = len(w.src)
	}
	if offset < 0 {
		offset = 0
	}
	return bytes.Count(w.src[:offset], []byte("\n")) + 1
}
