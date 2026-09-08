package formers

import (
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"

	"github.com/janstol/rails-kit/internal/astutil"
	"github.com/janstol/rails-kit/internal/prism"
)

// Parse reads a former file, parses it with Prism, and returns its
// structural summary. Prism is error-tolerant: recoverable syntax errors are
// attached to the summary while whatever structure Prism could recover is
// still returned.
func Parse(formerPath, railsRoot, formersPath string) (*Summary, error) {
	p, err := astutil.ParseFile(formerPath)
	if err != nil {
		return nil, err
	}

	s := &Summary{}
	s.RelPath, s.ClassName = astutil.SummaryPath(formerPath, railsRoot, formersPath)
	s.ParseErrors = p.Diagnostics
	if p.Program == nil {
		return s, nil
	}

	class, module := astutil.TopLevelClassOrModule(p.Program)
	if class == nil && module == nil {
		return s, nil
	}

	var body parser.Node
	if class != nil {
		s.Kind = "class"
		if class.Superclass != nil {
			s.ParentClass = prism.Slice(p.Src, class.Superclass.GetLocation())
		}
		body = class.Body
	} else {
		s.Kind = "module"
		body = module.Body
	}

	w := formerWalker{src: p.Src, summary: s}
	astutil.WalkClassBody(prism.BlockStatements(body), astutil.ClassBody{
		Def:                 w.handleDef,
		Call:                w.handleCall,
		Const:               w.handleConstant,
		SkipVisibilityCalls: true,
	})
	return s, nil
}

type formerWalker struct {
	src     []byte
	summary *Summary
}

// isValidationCall reports whether name is `validate` or one of the
// `validates`/`validates_*` family (validates_each, validates_presence_of,
// and the rest of ActiveModel's validation helpers).
func isValidationCall(name string) bool {
	return name == "validate" || strings.HasPrefix(name, "validates")
}

// handleDef collects an instance method (`def foo`, Receiver nil) only when
// it is public, and a singleton method (`def self.foo`, Receiver is
// *SelfNode) regardless of visibility. Every collected method is rendered as
// a signature -- a former's parameters are part of its useful summary, same
// reasoning as helpers and decorators, even though most former methods take
// none.
func (w *formerWalker) handleDef(def *parser.DefNode, visibility string) {
	if _, ok := def.Receiver.(*parser.SelfNode); ok {
		w.summary.Methods = append(w.summary.Methods, "  "+astutil.Signature(w.src, def))
		return
	}
	if def.Receiver == nil && visibility == "public" {
		w.summary.Methods = append(w.summary.Methods, "  "+astutil.Signature(w.src, def))
	}
}

// handleCall reports `include`d concerns separately, attr_accessor/reader/
// writer as Attributes, validate/validates/validates_* as Validations, a
// with_options block wrapping validations as a Validations group (see
// handleWithOptions), and everything else (delegate, store_accessor, and any
// app-specific macro) as a Macros catch-all. SkipVisibilityCalls keeps bare
// private/protected/public switches and the `private :a, :b` symbol-list form
// from leaking into Macros as noise.
func (w *formerWalker) handleCall(call *parser.CallNode) {
	if call.Receiver != nil {
		return
	}
	switch call.Name {
	case "include":
		w.handleInclude(prism.ArgNodes(call))
	case "attr_accessor", "attr_reader", "attr_writer":
		w.summary.Attributes = append(w.summary.Attributes, w.renderCall(call, "  "))
	case "with_options":
		w.handleWithOptions(call)
	default:
		if isValidationCall(call.Name) {
			w.summary.Validations = append(w.summary.Validations, w.renderCall(call, "  "))
			return
		}
		w.summary.Macros = append(w.summary.Macros, w.renderCall(call, "  "))
	}
}

// handleWithOptions descends one level into a `with_options do ... end`
// block -- astutil.WalkClassBody only walks a class body's top-level
// statements, so validations nested in a with_options block would otherwise
// be silently dropped. When the block holds at least one validation call, the
// with_options call itself becomes a Validations group header (without its
// usual " (block)" suffix, since the children are rendered right below it,
// four spaces deep) followed by each nested validation. A with_options block
// holding no validations is treated like any other class-level call and
// falls through to the Macros catch-all with its normal " (block)" suffix.
func (w *formerWalker) handleWithOptions(call *parser.CallNode) {
	block, ok := call.Block.(*parser.BlockNode)
	if !ok {
		w.summary.Macros = append(w.summary.Macros, w.renderCall(call, "  "))
		return
	}

	var nested []string
	for _, node := range prism.BlockStatements(block.Body) {
		child, ok := node.(*parser.CallNode)
		if !ok || child.Receiver != nil || !isValidationCall(child.Name) {
			continue
		}
		nested = append(nested, w.renderCall(child, "    "))
	}
	if len(nested) == 0 {
		w.summary.Macros = append(w.summary.Macros, w.renderCall(call, "  "))
		return
	}

	w.summary.Validations = append(w.summary.Validations, w.renderCallNoBlockSuffix(call, "  "))
	w.summary.Validations = append(w.summary.Validations, nested...)
}

// handleInclude reports every included module. No framework prefix to
// filter, same reasoning as services, helpers, and decorators -- so
// ActiveModel::Model shows like any other concern.
func (w *formerWalker) handleInclude(args []parser.Node) {
	if name := astutil.IncludedConcern(w.src, args, nil); name != "" {
		w.summary.Concerns = append(w.summary.Concerns, "  "+name)
	}
}

// handleConstant renders a class-level constant assignment as
// `  NAME = value`, collapsing any whitespace in the value expression.
func (w *formerWalker) handleConstant(n *parser.ConstantWriteNode) {
	if n.Value == nil {
		return
	}
	w.summary.Constants = append(w.summary.Constants, "  "+n.Name+" = "+astutil.JoinedSource(w.src, n.Value.GetLocation()))
}

// renderCall renders a class-level call as `<indent>name args` with
// whitespace collapsed, appending ` (block)` for a real block literal
// (`do…end` / `{…}`). A block-pass (`&:sym` / `&expr`) is carried on
// call.Block as a BlockArgumentNode, separate from the ArgumentsNode
// location, so it is folded into the rendered argument list rather than
// noted as a block.
func (w *formerWalker) renderCall(call *parser.CallNode, indent string) string {
	return w.render(call, indent, true)
}

// renderCallNoBlockSuffix is renderCall without the trailing " (block)" --
// used for a with_options group header whose children are rendered
// immediately below it, making the suffix redundant.
func (w *formerWalker) renderCallNoBlockSuffix(call *parser.CallNode, indent string) string {
	return w.render(call, indent, false)
}

func (w *formerWalker) render(call *parser.CallNode, indent string, blockSuffix bool) string {
	entry := indent + call.Name
	argSrc := ""
	if call.Arguments != nil {
		argSrc = astutil.JoinedSource(w.src, call.Arguments.GetLocation())
	}
	if bp, ok := call.Block.(*parser.BlockArgumentNode); ok {
		passSrc := astutil.JoinedSource(w.src, bp.GetLocation())
		if argSrc == "" {
			argSrc = passSrc
		} else {
			argSrc += ", " + passSrc
		}
	}
	if argSrc != "" {
		entry += " " + argSrc
	}
	if blockSuffix {
		if _, ok := call.Block.(*parser.BlockNode); ok {
			entry += " (block)"
		}
	}
	return entry
}
