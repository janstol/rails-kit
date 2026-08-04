package mailers

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
	"github.com/janstol/rails-kit/internal/astutil"
	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/prism"
)

var skippedConcernPrefixes = []string{
	"ActionMailer", "ActionController", "AbstractController", "ActiveSupport",
}

// Parse reads a mailer file, parses it with Prism, and returns its structural
// summary. Prism is error-tolerant: recoverable syntax errors are attached to
// the summary while whatever structure Prism could recover is still returned.
func Parse(mailerPath, railsRoot, mailersPath string) (*Summary, error) {
	ctx := context.Background()
	p, err := prism.NewParser(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating prism parser: %w", err)
	}
	defer p.Close(ctx) //nolint:errcheck

	result, src, err := p.Parse(ctx, mailerPath)
	if err != nil {
		return nil, err
	}

	s := summaryForPath(mailerPath, railsRoot, mailersPath)
	for _, parseErr := range result.Errors {
		s.ParseErrors = append(s.ParseErrors, ParseDiagnostic{
			Line:    prism.LineAt(src, parseErr.Location.StartOffset),
			Message: parseErr.Message,
		})
	}
	if result.Value == nil {
		return s, nil
	}

	class := astutil.TopLevelClass(result.Value)
	if class == nil {
		return s, nil
	}
	if class.Superclass != nil {
		s.ParentClass = prism.Slice(src, class.Superclass.GetLocation())
	}

	w := mailerWalker{src: src, summary: s}
	w.walkClassBody(prism.BlockStatements(class.Body))
	return s, nil
}

func summaryForPath(mailerPath, railsRoot, mailersPath string) *Summary {
	s := &Summary{}
	rel, err := filepath.Rel(railsRoot, mailerPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		rel = mailerPath
	}
	s.RelPath = filepath.ToSlash(rel)

	mailersDir := config.ResolvePath(railsRoot, mailersPath)
	namePart, err := filepath.Rel(mailersDir, mailerPath)
	if err != nil || strings.HasPrefix(namePart, "..") {
		namePart = filepath.Base(mailerPath)
	}
	namePart = strings.TrimSuffix(namePart, ".rb")
	classSegments := make([]string, 0, 2)
	for _, seg := range strings.Split(namePart, string(filepath.Separator)) {
		var camel string
		for _, part := range strings.Split(seg, "_") {
			if part != "" {
				camel += strings.ToUpper(part[:1]) + part[1:]
			}
		}
		classSegments = append(classSegments, camel)
	}
	s.ClassName = strings.Join(classSegments, "::")
	return s
}

type mailerWalker struct {
	src     []byte
	summary *Summary
}

// walkClassBody scans the mailer class's top-level statements, tracking
// `private`/`protected`/`public` visibility switches (both the bare-call form
// and the `private def foo; end` single-method form) so only public methods
// are reported. Every def's body is still scanned for attachments regardless
// of visibility, since attachments are routinely added in private helpers.
func (w *mailerWalker) walkClassBody(nodes []parser.Node) {
	visibility := "public"
	for _, node := range nodes {
		switch n := node.(type) {
		case *parser.CallNode:
			if n.Receiver == nil {
				switch n.Name {
				case "private", "protected", "public":
					args := prism.ArgNodes(n)
					if len(args) == 0 && n.Block == nil {
						visibility = n.Name
						continue
					}
					if len(args) == 1 {
						if def, ok := args[0].(*parser.DefNode); ok {
							w.handleDef(def, n.Name)
							continue
						}
					}
				}
			}
			w.handleCall(n)
		case *parser.DefNode:
			w.handleDef(n, visibility)
		}
	}
}

func (w *mailerWalker) handleDef(def *parser.DefNode, visibility string) {
	if def.Receiver == nil && visibility == "public" {
		w.summary.Methods = append(w.summary.Methods, "  "+def.Name)
	}
	w.collectAttachments(def)
}

func (w *mailerWalker) handleCall(call *parser.CallNode) {
	if call.Receiver != nil {
		return
	}
	args := prism.ArgNodes(call)
	switch call.Name {
	case "include":
		w.handleInclude(args)
	case "default":
		w.handleDefault(args)
	case "layout":
		w.handleLayout(args)
	}
}

func (w *mailerWalker) handleInclude(args []parser.Node) {
	if len(args) == 0 {
		return
	}
	name := astutil.ConstantName(w.src, args[0])
	if name == "" {
		return
	}
	for _, prefix := range skippedConcernPrefixes {
		if strings.HasPrefix(name, prefix) {
			return
		}
	}
	w.summary.Concerns = append(w.summary.Concerns, "  "+name)
}

func (w *mailerWalker) handleDefault(args []parser.Node) {
	for _, arg := range args {
		for _, assoc := range prism.KeywordAssocs(arg) {
			key, ok := prism.SymbolValue(assoc.Key)
			if !ok {
				continue
			}
			w.summary.Default = append(w.summary.Default, "  "+key+": "+renderDefault(w.src, assoc.Value))
		}
	}
}

func renderDefault(src []byte, node parser.Node) string {
	if s, ok := prism.StringValue(node); ok {
		return "\"" + s + "\""
	}
	if name, ok := prism.SymbolValue(node); ok {
		return ":" + name
	}
	return astutil.JoinedSource(src, node.GetLocation())
}

func (w *mailerWalker) handleLayout(args []parser.Node) {
	if len(args) == 0 {
		return
	}
	switch {
	case astutil.IsFalseNode(args[0]):
		w.summary.Layout = "false"
	default:
		if name, ok := prism.StringValue(args[0]); ok {
			w.summary.Layout = "\"" + name + "\""
			return
		}
		if name, ok := prism.SymbolValue(args[0]); ok {
			w.summary.Layout = ":" + name
			return
		}
		w.summary.Layout = astutil.JoinedSource(w.src, args[0].GetLocation())
	}
}

type attachmentRef struct {
	key    string
	inline bool
	loc    int
}

// collectAttachments finds every `attachments["name"] = ...` or
// `attachments.inline["name"] = ...` assignment anywhere in def's body --
// not just as its sole statement -- and reports it in source order.
func (w *mailerWalker) collectAttachments(def *parser.DefNode) {
	var refs []attachmentRef
	var walk func(parser.Node)
	walk = func(node parser.Node) {
		if node == nil {
			return
		}
		if call, ok := node.(*parser.CallNode); ok {
			if key, inline, ok := attachmentKey(call); ok {
				refs = append(refs, attachmentRef{key: key, inline: inline, loc: call.GetLocation().StartOffset})
			}
		}
		for _, child := range node.CompactChildNodes() {
			walk(child)
		}
	}
	walk(def.Body)
	sort.SliceStable(refs, func(i, j int) bool { return refs[i].loc < refs[j].loc })
	for _, r := range refs {
		if r.inline {
			w.summary.Attachments = append(w.summary.Attachments, fmt.Sprintf("  attachments.inline[%q]", r.key))
		} else {
			w.summary.Attachments = append(w.summary.Attachments, fmt.Sprintf("  attachments[%q]", r.key))
		}
	}
}

// attachmentKey reports whether call is an `attachments["key"] = ...` or
// `attachments.inline["key"] = ...` assignment, and if so, the string key and
// whether it is the inline variant.
func attachmentKey(call *parser.CallNode) (key string, inline bool, ok bool) {
	if call.Name != "[]=" {
		return "", false, false
	}
	receiver, isCall := call.Receiver.(*parser.CallNode)
	if !isCall {
		return "", false, false
	}
	args := prism.ArgNodes(call)
	if len(args) < 1 {
		return "", false, false
	}
	k, isStr := prism.StringValue(args[0])
	if !isStr {
		return "", false, false
	}
	// Regular: attachments["key"] = ... -- receiver is `attachments`.
	if receiver.Name == "attachments" && receiver.Receiver == nil {
		return k, false, true
	}
	// Inline: attachments.inline["key"] = ... -- receiver is `inline`
	// called on `attachments`.
	if receiver.Name == "inline" {
		inner, isCall := receiver.Receiver.(*parser.CallNode)
		if isCall && inner.Name == "attachments" && inner.Receiver == nil {
			return k, true, true
		}
	}
	return "", false, false
}
