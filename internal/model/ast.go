package model

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/danielgatis/go-ruby-prism/parser"
	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/prism"
)

var associationNames = map[string]bool{
	"has_many": true, "has_one": true, "belongs_to": true, "has_and_belongs_to_many": true,
}

var callbackNames = map[string]bool{
	"before_validation": true, "after_validation": true,
	"before_create": true, "after_create_commit": true, "after_create": true,
	"before_update": true, "after_update_commit": true, "after_update": true,
	"before_save": true, "after_save_commit": true, "after_save": true,
	"before_destroy": true, "after_destroy_commit": true, "after_destroy": true,
	"after_initialize": true, "before_commit": true, "after_commit": true,
	"after_rollback": true, "after_touch": true, "after_find": true,
}

var skippedConcernPrefixes = []string{
	"ActiveModel", "ActiveRecord", "ActiveSupport", "Devise", "Comparable", "Enumerable",
}

// Parse reads a model file, parses it with Prism, and returns its structural summary.
// Prism is error-tolerant: recoverable syntax errors are attached to the summary while
// whatever structure Prism could recover is still returned.
func Parse(modelPath, railsRoot, modelsPath string) (*Summary, error) {
	ctx := context.Background()
	p, err := prism.NewParser(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating prism parser: %w", err)
	}
	defer p.Close(ctx) //nolint:errcheck

	result, src, err := p.Parse(ctx, modelPath)
	if err != nil {
		return nil, err
	}

	s := summaryForPath(modelPath, railsRoot, modelsPath)
	for _, parseErr := range result.Errors {
		s.ParseErrors = append(s.ParseErrors, ParseDiagnostic{
			Line:    prism.LineAt(src, parseErr.Location.StartOffset),
			Message: parseErr.Message,
		})
	}
	if result.Value == nil {
		return s, nil
	}

	w := modelWalker{src: src, summary: s, seenLines: make(map[int]bool)}
	w.walk(result.Value)
	return s, nil
}

func summaryForPath(modelPath, railsRoot, modelsPath string) *Summary {
	s := &Summary{}
	rel, err := filepath.Rel(railsRoot, modelPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		rel = modelPath
	}
	s.RelPath = filepath.ToSlash(rel)

	modelsDir := config.ResolvePath(railsRoot, modelsPath)
	namePart, err := filepath.Rel(modelsDir, modelPath)
	if err != nil || strings.HasPrefix(namePart, "..") {
		namePart = filepath.Base(modelPath)
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

type modelWalker struct {
	src       []byte
	summary   *Summary
	seenLines map[int]bool
}

func (w *modelWalker) walk(root parser.Node) {
	var nodes []parser.Node
	var collect func(parser.Node)
	collect = func(node parser.Node) {
		if node == nil {
			return
		}
		switch node.(type) {
		case *parser.ClassNode, *parser.CallNode:
			nodes = append(nodes, node)
		}
		for _, child := range node.CompactChildNodes() {
			collect(child)
		}
	}
	collect(root)
	sort.SliceStable(nodes, func(i, j int) bool {
		return nodes[i].GetLocation().StartOffset < nodes[j].GetLocation().StartOffset
	})
	for _, node := range nodes {
		line := prism.LineAt(w.src, node.GetLocation().StartOffset)
		if w.seenLines[line] {
			continue
		}
		handled := false
		switch n := node.(type) {
		case *parser.ClassNode:
			if legacyConstantPath(prism.Slice(w.src, n.ConstantPath.GetLocation())) {
				superclass := ""
				if n.Superclass != nil {
					superclass = prism.Slice(w.src, n.Superclass.GetLocation())
				}
				if legacyConstantPath(superclass) {
					w.summary.ParentClass = superclass
				} else {
					w.summary.ParentClass = ""
				}
			}
			handled = true
		case *parser.CallNode:
			handled = w.handleCall(n)
		}
		if handled {
			w.seenLines[line] = true
		}
	}
}

func (w *modelWalker) handleCall(call *parser.CallNode) bool {
	if call.Name == "table_name=" {
		return w.handleTableName(call)
	}
	if call.Receiver != nil {
		return false
	}
	if !w.legacyBareCall(call) {
		return false
	}
	args := prism.ArgNodes(call)
	switch {
	case call.Name == "include":
		return w.handleInclude(args)
	case associationNames[call.Name]:
		return w.handleAssociation(call.Name, args)
	case call.Name == "validate":
		return w.handleCustomValidation(args)
	case call.Name == "validates" || strings.HasPrefix(call.Name, "validates_"):
		return w.handleValidation(call.Name, args)
	case call.Name == "scope":
		return w.handleScope(call, args)
	case callbackNames[call.Name] || strings.HasPrefix(call.Name, "around_"):
		return w.handleCallback(call)
	case call.Name == "enum":
		return w.handleEnum(args)
	case call.Name == "delegate":
		return w.handleDelegate(call)
	}
	return false
}

func (w *modelWalker) legacyBareCall(call *parser.CallNode) bool {
	source, _ := legacyDeclaration(w.src, call.Location.StartOffset)
	source = strings.TrimLeft(source, " \t")
	if !strings.HasPrefix(source, call.Name) || len(source) == len(call.Name) {
		return false
	}
	next := source[len(call.Name)]
	return next == ' ' || next == '\t'
}

func (w *modelWalker) handleTableName(call *parser.CallNode) bool {
	if _, ok := call.Receiver.(*parser.SelfNode); !ok {
		return false
	}
	declaration, end := legacyDeclaration(w.src, call.Location.StartOffset)
	if !strings.HasPrefix(strings.TrimLeft(declaration, " \t"), "self.table_name") {
		return false
	}
	args := prism.ArgNodes(call)
	if len(args) != 1 {
		return false
	}
	if args[0].GetLocation().StartOffset >= end {
		return false
	}
	name, ok := prism.StringValue(args[0])
	if !ok {
		return false
	}
	w.summary.TableName = name
	return true
}

func (w *modelWalker) handleInclude(args []parser.Node) bool {
	if len(args) == 0 {
		return false
	}
	name := constantName(w.src, args[0])
	if name == "" {
		name = firstSourceToken(prism.Slice(w.src, args[0].GetLocation()))
	}
	if name == "" {
		return false
	}
	for _, prefix := range skippedConcernPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	w.summary.Concerns = append(w.summary.Concerns, "  "+name)
	return true
}

func (w *modelWalker) handleAssociation(name string, args []parser.Node) bool {
	if len(args) == 0 {
		return false
	}
	assocName, ok := prism.SymbolValue(args[0])
	if !ok {
		return false
	}
	assocName = legacyWord(assocName)
	source, _ := legacyDeclaration(w.src, args[0].GetLocation().StartOffset)
	var opts []string
	if value := sourceLooseWordOption(source, "through"); value != "" {
		opts = append(opts, "through: "+value)
	}
	if value := sourceQuotedOption(source, "class_name"); value != "" {
		opts = append(opts, "class_name: "+value)
	}
	if sourceOptionTrue(source, "polymorphic") {
		opts = append(opts, "polymorphic: true")
	}
	if value := sourceRequiredSymbolOption(source, "dependent"); value != "" {
		opts = append(opts, "dependent: "+value)
	}
	if sourceOptionTrue(source, "optional") {
		opts = append(opts, "optional: true")
	}
	if value := sourceLooseWordOption(source, "inverse_of"); value != "" {
		opts = append(opts, "inverse_of: "+value)
	}
	if value := sourceLooseWordOption(source, "source"); value != "" {
		opts = append(opts, "source: "+value)
	}
	entry := "  " + name + " :" + assocName
	if len(opts) > 0 {
		entry += ", " + strings.Join(opts, ", ")
	}
	w.summary.Assocs = append(w.summary.Assocs, entry)
	return true
}

func (w *modelWalker) handleCustomValidation(args []parser.Node) bool {
	if len(args) == 0 {
		return false
	}
	name, ok := prism.SymbolValue(args[0])
	if !ok {
		return false
	}
	w.summary.Valids = append(w.summary.Valids, "  validate :"+legacyWord(name)+" (custom)")
	return true
}

func (w *modelWalker) handleValidation(macro string, args []parser.Node) bool {
	if len(args) == 0 {
		return false
	}
	first, ok := prism.SymbolValue(args[0])
	if !ok {
		return false
	}
	first = legacyWord(first)
	source, _ := legacyDeclaration(w.src, args[0].GetLocation().StartOffset)
	fields := legacyValidationFields(source, first)
	details := validationDetails(source)
	if macro != "validates" {
		short := strings.TrimPrefix(macro, "validates_")
		if short != "" && !contains(details, short) {
			details = append(details, short)
		}
	}
	entry := "  validates :" + strings.Join(fields, ", :")
	if len(details) > 0 {
		entry += ", " + strings.Join(details, ", ")
	}
	w.summary.Valids = append(w.summary.Valids, entry)
	return true
}

func validationDetails(source string) []string {
	var details []string
	checks := []struct {
		name   string
		needle string
	}{
		{"presence", "presence:"}, {"uniqueness", "uniqueness"}, {"length", "length:"},
		{"format", "format:"}, {"numericality", "numericality"}, {"inclusion", "inclusion:"},
		{"exclusion", "exclusion:"}, {"confirmation", "confirmation:"}, {"email_format", "email_format:"},
	}
	for _, check := range checks {
		if strings.Contains(source, check.needle) &&
			(check.name != "presence" && check.name != "confirmation" || sourceOptionTrue(source, check.name)) {
			details = append(details, check.name)
		}
	}
	if sourceOptionTrue(source, "allow_nil") {
		details = append(details, "allow_nil")
	}
	if sourceOptionTrue(source, "allow_blank") {
		details = append(details, "allow_blank")
	}
	if value := sourceSymbolOption(source, "on"); value != "" {
		details = append(details, "on: "+value)
	}
	return details
}

func (w *modelWalker) handleScope(call *parser.CallNode, args []parser.Node) bool {
	if len(args) == 0 {
		return false
	}
	name, ok := prism.SymbolValue(args[0])
	if !ok {
		return false
	}
	entry := "  " + name
	if len(args) > 1 {
		source, _ := legacyDeclaration(w.src, call.Location.StartOffset)
		if params, hasParams := scopeParams(source, args[1]); hasParams {
			entry += "(" + params + ")"
		}
	}
	// A block can belong to lambda() itself rather than the scope call.
	if len(args) > 1 {
		if lambda, ok := args[1].(*parser.CallNode); ok && lambda.Name == "lambda" && lambda.Block != nil && blockHasParams(lambda.Block) {
			entry = "  " + name + "(...)"
		}
	}
	w.summary.Scopes = append(w.summary.Scopes, entry)
	return true
}

func scopeParams(declaration string, node parser.Node) (string, bool) {
	lambda, ok := node.(*parser.LambdaNode)
	if !ok || lambda.Parameters == nil {
		return "", false
	}
	if arrow := strings.Index(declaration, "->"); arrow >= 0 {
		rest := strings.TrimLeft(declaration[arrow+2:], " \t")
		if strings.HasPrefix(rest, "(") {
			if end := strings.IndexByte(rest[1:], ')'); end >= 0 {
				return strings.TrimSpace(rest[1 : 1+end]), true
			}
		}
	}
	return "...", true
}

func blockHasParams(node parser.Node) bool {
	block, ok := node.(*parser.BlockNode)
	return ok && block.Parameters != nil
}

func (w *modelWalker) handleCallback(call *parser.CallNode) bool {
	entry := "  " + call.Name
	source, _ := legacyDeclaration(w.src, call.Location.StartOffset)
	if target := legacyCallbackTarget(source, call.Name); target != "" {
		entry += " :" + target
	}
	w.summary.Callbacks = append(w.summary.Callbacks, entry)
	return true
}

func (w *modelWalker) handleEnum(args []parser.Node) bool {
	if len(args) == 0 {
		return false
	}
	if name, ok := prism.SymbolValue(args[0]); ok {
		w.summary.Enums = append(w.summary.Enums, "  "+legacyWord(name))
		return true
	}
	assocs := prism.KeywordAssocs(args[0])
	if len(assocs) > 0 {
		if name, ok := prism.SymbolValue(assocs[0].Key); ok {
			w.summary.Enums = append(w.summary.Enums, "  "+legacyWord(name))
			return true
		}
	}
	return false
}

func (w *modelWalker) handleDelegate(call *parser.CallNode) bool {
	source, _ := legacyDeclaration(w.src, call.Location.StartOffset)
	source = strings.TrimSpace(source)
	if !strings.HasPrefix(source, call.Name) {
		return false
	}
	rest := flattenCallRest(strings.TrimSpace(source[len(call.Name):]))
	if rest == "" {
		return false
	}
	if runes := []rune(rest); len(runes) > 80 {
		rest = string(runes[:80]) + "..."
	}
	w.summary.Delegates = append(w.summary.Delegates, "  delegate "+rest)
	return true
}

func flattenCallRest(source string) string {
	lines := strings.Split(source, "\n")
	if len(lines) == 1 {
		return source
	}
	var b strings.Builder
	b.WriteString(strings.TrimSpace(lines[0]))
	for _, line := range lines[1:] {
		b.WriteByte(' ')
		b.WriteString(strings.TrimSpace(line))
	}
	return b.String()
}

func constantName(src []byte, node parser.Node) string {
	switch node.(type) {
	case *parser.ConstantReadNode, *parser.ConstantPathNode:
		return strings.TrimSpace(prism.Slice(src, node.GetLocation()))
	default:
		return ""
	}
}

func legacyConstantPath(source string) bool {
	if source == "" || strings.HasPrefix(source, "::") {
		return false
	}
	for _, segment := range strings.Split(source, "::") {
		if segment == "" || segment[0] < 'A' || segment[0] > 'Z' {
			return false
		}
		for _, r := range segment[1:] {
			if !isWordRune(r) {
				return false
			}
		}
	}
	return true
}

func firstSourceToken(source string) string {
	if index := strings.IndexAny(source, " \t\r\n"); index >= 0 {
		return source[:index]
	}
	return source
}

func legacyWord(value string) string {
	for index, r := range value {
		if !isWordRune(r) {
			return value[:index]
		}
	}
	return value
}

func isWordRune(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_'
}

func legacyDeclaration(src []byte, offset int) (string, int) {
	start := offset
	for start > 0 && src[start-1] != '\n' {
		start--
	}
	end := start
	for end < len(src) && src[end] != '\n' {
		end++
	}
	line := string(src[start:end])
	indent := leadingIndent(line)
	joined := line
	for continuationLine(joined) && end < len(src) {
		nextStart := end + 1
		nextEnd := nextStart
		for nextEnd < len(src) && src[nextEnd] != '\n' {
			nextEnd++
		}
		next := string(src[nextStart:nextEnd])
		if leadingIndent(next) <= indent {
			break
		}
		joined += " " + strings.TrimSpace(next)
		end = nextEnd
	}
	return joined, end
}

func continuationLine(line string) bool {
	if hash := strings.LastIndexByte(line, '#'); hash >= 0 && !strings.ContainsAny(line[hash:], "\"'") {
		line = line[:hash]
	}
	return strings.HasSuffix(strings.TrimRight(line, " \t"), ",")
}

func leadingIndent(line string) int {
	return len(line) - len(strings.TrimLeft(line, " \t"))
}

func sourceOptionTrue(source, key string) bool {
	rest := sourceAfterKey(source, key)
	return strings.HasPrefix(strings.TrimLeft(rest, " \t"), "true")
}

func sourceSymbolOption(source, key string) string {
	needle := key + ":"
	for search := source; ; {
		index := strings.Index(search, needle)
		if index < 0 {
			return ""
		}
		rest := strings.TrimLeft(search[index+len(needle):], " \t")
		if strings.HasPrefix(rest, ":") {
			return legacyWord(rest[1:])
		}
		search = search[index+len(needle):]
	}
}

func sourceAfterKey(source, key string) string {
	needle := key + ":"
	index := strings.Index(source, needle)
	if index < 0 {
		return ""
	}
	return source[index+len(needle):]
}

func sourceLooseWordOption(source, key string) string {
	rest := strings.TrimLeft(sourceAfterKey(source, key), " \t")
	if strings.HasPrefix(rest, ":") {
		rest = strings.TrimLeft(rest[1:], " \t")
	}
	rest = strings.TrimPrefix(rest, "\"")
	return legacyWord(rest)
}

func sourceRequiredSymbolOption(source, key string) string {
	rest := strings.TrimLeft(sourceAfterKey(source, key), " \t")
	if !strings.HasPrefix(rest, ":") {
		return ""
	}
	return legacyWord(strings.TrimLeft(rest[1:], " \t"))
}

func sourceQuotedOption(source, key string) string {
	rest := strings.TrimLeft(sourceAfterKey(source, key), " \t")
	if len(rest) < 3 || rest[0] != '\'' && rest[0] != '"' || !isWordRune(rune(rest[1])) {
		return ""
	}
	if end := strings.IndexByte(rest[1:], rest[0]); end >= 0 {
		return rest[1 : 1+end]
	}
	return ""
}

func legacyValidationFields(source, first string) []string {
	fields := []string{first}
	marker := ":" + first
	index := strings.Index(source, marker)
	if index < 0 {
		return fields
	}
	section := source[index+len(marker):]
	if option := legacyFieldOptionStart(section); option >= 0 {
		section = section[:option]
	}
	for index := 0; index < len(section); index++ {
		if section[index] != ':' || index+1 >= len(section) || !isLowerFieldStart(section[index+1]) {
			continue
		}
		field := legacyWord(section[index+1:])
		if field != "" {
			fields = append(fields, field)
			index += len(field)
		}
	}
	return fields
}

func legacyFieldOptionStart(source string) int {
	for index := 0; index < len(source); index++ {
		if source[index] != ' ' && source[index] != '\t' {
			continue
		}
		wordStart := index + 1
		if wordStart >= len(source) || !isLowerFieldStart(source[wordStart]) {
			continue
		}
		wordEnd := wordStart
		for wordEnd < len(source) && isWordRune(rune(source[wordEnd])) {
			wordEnd++
		}
		if wordEnd < len(source) && source[wordEnd] == ':' {
			return index
		}
	}
	return -1
}

func isLowerFieldStart(value byte) bool {
	return value >= 'a' && value <= 'z' || value == '_'
}

func legacyCallbackTarget(source, callback string) string {
	index := strings.Index(source, callback)
	if index < 0 {
		return ""
	}
	rest := source[index+len(callback):]
	colon := strings.IndexByte(rest, ':')
	if colon < 0 {
		return ""
	}
	rest = strings.TrimLeft(rest[colon+1:], " \t")
	target := legacyWord(rest)
	if target == "" {
		return ""
	}
	tail := strings.TrimSpace(rest[len(target):])
	if tail == "" || strings.HasPrefix(tail, ",") {
		return target
	}
	return ""
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
