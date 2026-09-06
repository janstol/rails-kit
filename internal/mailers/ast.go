package mailers

import (
	"fmt"

	"github.com/danielgatis/go-ruby-prism/parser"
	"github.com/janstol/rails-kit/internal/astutil"
	"github.com/janstol/rails-kit/internal/prism"
)

var skippedConcernPrefixes = []string{
	"ActionMailer", "ActionController", "AbstractController", "ActiveSupport",
}

// Parse reads a mailer file, parses it with Prism, and returns its structural
// summary. Prism is error-tolerant: recoverable syntax errors are attached to
// the summary while whatever structure Prism could recover is still returned.
func Parse(mailerPath, railsRoot, mailersPath string) (*Summary, error) {
	p, err := astutil.ParseFile(mailerPath)
	if err != nil {
		return nil, err
	}

	s := &Summary{}
	s.RelPath, s.ClassName = astutil.SummaryPath(mailerPath, railsRoot, mailersPath)
	s.ParseErrors = p.Diagnostics
	if p.Program == nil {
		return s, nil
	}

	class := astutil.TopLevelClass(p.Program)
	if class == nil {
		return s, nil
	}
	if class.Superclass != nil {
		s.ParentClass = prism.Slice(p.Src, class.Superclass.GetLocation())
	}

	w := mailerWalker{src: p.Src, summary: s}
	astutil.WalkClassBody(prism.BlockStatements(class.Body), astutil.ClassBody{
		Def:  w.handleDef,
		Call: w.handleCall,
	})
	return s, nil
}

type mailerWalker struct {
	src     []byte
	summary *Summary
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
	if name := astutil.IncludedConcern(w.src, args, skippedConcernPrefixes); name != "" {
		w.summary.Concerns = append(w.summary.Concerns, "  "+name)
	}
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
	if value := astutil.LayoutValue(w.src, args); value != "" {
		w.summary.Layout = value
	}
}

// collectAttachments finds every `attachments["name"] = ...` or
// `attachments.inline["name"] = ...` assignment anywhere in def's body --
// not just as its sole statement -- and reports it in source order.
func (w *mailerWalker) collectAttachments(def *parser.DefNode) {
	calls := astutil.CollectCalls(def.Body, func(call *parser.CallNode) bool {
		_, _, ok := attachmentKey(call)
		return ok
	})
	for _, call := range calls {
		key, inline, _ := attachmentKey(call)
		if inline {
			w.summary.Attachments = append(w.summary.Attachments, fmt.Sprintf("  attachments.inline[%q]", key))
		} else {
			w.summary.Attachments = append(w.summary.Attachments, fmt.Sprintf("  attachments[%q]", key))
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
