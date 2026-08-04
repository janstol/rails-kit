package jobs

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/janstol/rails-kit/internal/astutil"
	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/prism"
)

var skippedConcernPrefixes = []string{
	"ActiveJob", "ActiveSupport",
}

// Parse reads a job file, parses it with Prism, and returns its structural
// summary. Prism is error-tolerant: recoverable syntax errors are attached to
// the summary while whatever structure Prism could recover is still returned.
func Parse(jobPath, railsRoot, jobsPath string) (*Summary, error) {
	ctx := context.Background()
	p, err := prism.NewParser(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating prism parser: %w", err)
	}
	defer p.Close(ctx) //nolint:errcheck

	result, src, err := p.Parse(ctx, jobPath)
	if err != nil {
		return nil, err
	}

	s := summaryForPath(jobPath, railsRoot, jobsPath)
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

	w := jobWalker{src: src, summary: s}
	w.walkClassBody(prism.BlockStatements(class.Body))
	return s, nil
}

func summaryForPath(jobPath, railsRoot, jobsPath string) *Summary {
	s := &Summary{}
	rel, err := filepath.Rel(railsRoot, jobPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		rel = jobPath
	}
	s.RelPath = filepath.ToSlash(rel)

	jobsDir := config.ResolvePath(railsRoot, jobsPath)
	namePart, err := filepath.Rel(jobsDir, jobPath)
	if err != nil || strings.HasPrefix(namePart, "..") {
		namePart = filepath.Base(jobPath)
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

type jobWalker struct {
	src     []byte
	summary *Summary
}

// walkClassBody scans the job class's top-level statements, tracking
// `private`/`protected`/`public` visibility switches (both the bare-call form
// and the `private def foo; end` single-method form) so only public methods
// are reported.
func (w *jobWalker) walkClassBody(nodes []parser.Node) {
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

func (w *jobWalker) handleDef(def *parser.DefNode, visibility string) {
	if def.Receiver == nil && visibility == "public" {
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
	byKey := make(map[string]parser.Node, len(assocs))
	for _, assoc := range assocs {
		if key, ok := prism.SymbolValue(assoc.Key); ok {
			byKey[key] = assoc.Value
		}
	}
	var opts []string
	for _, key := range []string{"wait", "attempts", "wait_jitter", "queue", "priority"} {
		if value, ok := byKey[key]; ok {
			opts = append(opts, key+": "+astutil.JoinedSource(src, value.GetLocation()))
		}
	}
	return opts
}
