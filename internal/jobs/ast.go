package jobs

import (
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/janstol/rails-kit/internal/astutil"
	"github.com/janstol/rails-kit/internal/prism"
)

var skippedConcernPrefixes = []string{
	"ActiveJob", "ActiveSupport",
}

// Parse reads a job file, parses it with Prism, and returns its structural
// summary. Prism is error-tolerant: recoverable syntax errors are attached to
// the summary while whatever structure Prism could recover is still returned.
func Parse(jobPath, railsRoot, jobsPath string) (*Summary, error) {
	p, err := astutil.ParseFile(jobPath)
	if err != nil {
		return nil, err
	}

	s := &Summary{}
	s.RelPath, s.ClassName = astutil.SummaryPath(jobPath, railsRoot, jobsPath)
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

	w := jobWalker{src: p.Src, summary: s}
	astutil.WalkClassBody(prism.BlockStatements(class.Body), astutil.ClassBody{
		Def:  w.handleDef,
		Call: w.handleCall,
	})
	return s, nil
}

type jobWalker struct {
	src     []byte
	summary *Summary
}

// handleDef collects an instance method (`def foo`, Receiver nil) when it is
// public. A class method -- `def self.foo`, or a def inside a
// `class << self` block -- is excluded here just as `def self.foo` always
// was: jobs have no notion of a class-level job method.
func (w *jobWalker) handleDef(def *parser.DefNode, visibility string, singleton bool) {
	if !singleton && def.Receiver == nil && visibility == "public" {
		w.summary.Methods = append(w.summary.Methods, "  "+def.Name)
	}
}

func (w *jobWalker) handleCall(call *parser.CallNode) {
	if call.Receiver != nil {
		return
	}
	args := prism.ArgNodes(call)
	switch call.Name {
	case "include":
		w.handleInclude(args)
	case "queue_as":
		w.handleQueueAs(call, args)
	case "retry_on":
		w.handleRetryOn(call, args)
	case "discard_on":
		w.handleDiscardOn(call, args)
	}
}

func (w *jobWalker) handleInclude(args []parser.Node) {
	if name := astutil.IncludedConcern(w.src, args, skippedConcernPrefixes); name != "" {
		w.summary.Concerns = append(w.summary.Concerns, "  "+name)
	}
}

// handleQueueAs renders the queue argument: a symbol as `:name`, a string as
// `"name"`, an explicit nil as `nil`, a bare block (no args) as `(block)`, and
// anything else as its joined source.
func (w *jobWalker) handleQueueAs(call *parser.CallNode, args []parser.Node) {
	if len(args) == 0 {
		if call.Block != nil {
			w.summary.Queue = "(block)"
		}
		return
	}
	arg := args[0]
	if _, ok := arg.(*parser.NilNode); ok {
		w.summary.Queue = "nil"
		return
	}
	if name, ok := prism.SymbolValue(arg); ok {
		w.summary.Queue = ":" + name
		return
	}
	if name, ok := prism.StringValue(arg); ok {
		w.summary.Queue = "\"" + name + "\""
		return
	}
	w.summary.Queue = astutil.JoinedSource(w.src, arg.GetLocation())
}

func (w *jobWalker) handleRetryOn(call *parser.CallNode, args []parser.Node) {
	w.handleRetryOrDiscard(call, args, "retry_on", &w.summary.RetryOn)
}

func (w *jobWalker) handleDiscardOn(call *parser.CallNode, args []parser.Node) {
	w.handleRetryOrDiscard(call, args, "discard_on", &w.summary.DiscardOn)
}

// handleRetryOrDiscard mirrors controllers.handleRescueFrom: it collects the
// constant exception class names from the non-keyword args, then appends the
// keyword options in a fixed deterministic order. Unlike rescue_from's
// `with:` (which is exclusive with a block), retry_on/discard_on options
// configure retry timing while a block supplies the handler, so the two
// coexist -- a trailing block is noted with ` (block)` regardless of opts.
func (w *jobWalker) handleRetryOrDiscard(call *parser.CallNode, args []parser.Node, macro string, dest *[]string) {
	if len(args) == 0 {
		return
	}
	var classes []string
	var opts []string
	for _, arg := range args {
		if assocs := prism.KeywordAssocs(arg); len(assocs) > 0 {
			opts = append(opts, retryOptions(w.src, assocs)...)
			continue
		}
		if name := astutil.ConstantName(w.src, arg); name != "" {
			classes = append(classes, name)
		}
	}
	if len(classes) == 0 {
		return
	}
	entry := "  " + macro + " " + strings.Join(classes, ", ")
	if len(opts) > 0 {
		entry += ", " + strings.Join(opts, ", ")
	}
	if call.Block != nil {
		entry += " (block)"
	}
	*dest = append(*dest, entry)
}

// retryOptions extracts wait/attempts/wait_jitter/queue/priority from a
// retry_on/discard_on trailing keyword hash, always in that order regardless
// of source order, so entries are deterministic. Values are rendered as their
// joined source (e.g. `5.seconds`, `3`).
func retryOptions(src []byte, assocs []*parser.AssocNode) []string {
	byKey := astutil.AssocsBySymbolKey(assocs)
	var opts []string
	for _, key := range []string{"wait", "attempts", "wait_jitter", "queue", "priority"} {
		if value, ok := byKey[key]; ok {
			opts = append(opts, key+": "+astutil.JoinedSource(src, value.GetLocation()))
		}
	}
	return opts
}
