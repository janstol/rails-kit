package related

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/model"
	"github.com/janstol/rails-kit/internal/pluralize"
)

// ResolveLookup maps an arbitrary user input (model name, file path) to the
// canonical model name and its plural form used for category discovery.
func ResolveLookup(root string, cfg config.Config, input string, p *pluralize.Pluralizer) (string, string, error) {
	input = NormalizeInput(input)
	if strings.HasSuffix(input, ".rb") {
		if name, plural, err := resolveModel(root, cfg.ModelsPath, input, p); err == nil {
			return name, plural, nil
		}
	}

	if IsPathInput(cfg, input) {
		return ResolvePath(root, cfg, input, p)
	}

	name := NormalizeNameWithPrefixes(input, PathPrefixes(cfg))
	resolved, err := model.Resolve(root, cfg.ModelsPath, name)
	if err != nil {
		return "", "", err
	}
	rel, relErr := filepath.Rel(config.ResolvePath(root, cfg.ModelsPath), resolved)
	if relErr == nil {
		name = strings.TrimSuffix(rel, ".rb")
	}

	return name, p.Pluralize(filepath.Base(name)), nil
}

// IsPathInput reports whether the input looks like a file path rather than a
// bare model name (e.g. it starts with a known configured directory prefix,
// ends with a known file extension, or is absolute).
func IsPathInput(cfg config.Config, input string) bool {
	if strings.HasSuffix(input, ".rb") || strings.HasSuffix(input, ".yml") || strings.HasSuffix(input, ".yaml") {
		return true
	}
	normalized := filepath.ToSlash(input)
	for _, prefix := range PathPrefixes(cfg) {
		if strings.HasPrefix(normalized, prefix) {
			return true
		}
	}
	return filepath.IsAbs(input)
}

// ResolvePath maps a related-file path (controller, view, fixture, etc.) back
// to its canonical model name before category discovery runs.
func ResolvePath(root string, cfg config.Config, input string, p *pluralize.Pluralizer) (string, string, error) {
	name, err := resolvePathModelName(root, cfg, input, p)
	if err != nil {
		return "", "", err
	}
	resolved, err := model.Resolve(root, cfg.ModelsPath, name)
	if err != nil {
		return "", "", err
	}
	rel, relErr := filepath.Rel(config.ResolvePath(root, cfg.ModelsPath), resolved)
	if relErr != nil {
		return "", "", fmt.Errorf("model not under models path: %s", input)
	}
	name = strings.TrimSuffix(rel, ".rb")
	return name, p.Pluralize(filepath.Base(name)), nil
}

// resolvePathModelName maps a supported related-file path back to its
// canonical model name before category discovery runs.
func resolvePathModelName(root string, cfg config.Config, input string, p *pluralize.Pluralizer) (string, error) {
	if rel, ok := trimInputPrefix(root, input, cfg.ModelsPath); ok && strings.HasSuffix(rel, ".rb") {
		return strings.TrimSuffix(filepath.ToSlash(rel), ".rb"), nil
	}
	if rel, ok := trimInputPrefix(root, input, cfg.FixturesPath); ok && (strings.HasSuffix(rel, ".yml") || strings.HasSuffix(rel, ".yaml")) {
		return FixtureModelName(filepath.ToSlash(rel), p), nil
	}
	if rel, ok := trimInputPrefix(root, input, cfg.SpecFixturesPath); ok && (strings.HasSuffix(rel, ".yml") || strings.HasSuffix(rel, ".yaml")) {
		return FixtureModelName(filepath.ToSlash(rel), p), nil
	}

	cleaned := NormalizeInput(input)
	cleanedSlash := filepath.ToSlash(cleaned)
	if filepath.IsAbs(cleaned) {
		if rel, err := filepath.Rel(root, cleaned); err == nil && !strings.HasPrefix(rel, "..") {
			cleanedSlash = filepath.ToSlash(rel)
		}
	}

	if (strings.HasSuffix(cleanedSlash, ".yml") || strings.HasSuffix(cleanedSlash, ".yaml")) && !strings.Contains(cleanedSlash, "/") {
		return FixtureModelName(cleanedSlash, p), nil
	}

	type pathRule struct {
		prefix  string
		resolve func(rel string) []string
	}

	rules := []pathRule{
		{prefix: cfg.ControllersPath, resolve: func(rel string) []string {
			return ResourceCandidates(rel, "_controller.rb", p)
		}},
		{prefix: cfg.TestControllersPath, resolve: func(rel string) []string {
			return ResourceCandidates(rel, "_controller_test.rb", p)
		}},
		{prefix: cfg.SpecControllersPath, resolve: func(rel string) []string {
			return ResourceCandidates(rel, "_controller_spec.rb", p)
		}},
		{prefix: cfg.DecoratorsPath, resolve: func(rel string) []string {
			return ResourceCandidates(rel, "_decorator.rb", p)
		}},
		{prefix: cfg.JobsPath, resolve: func(rel string) []string {
			return ResourceCandidates(rel, "_job.rb", p)
		}},
		{prefix: cfg.MailersPath, resolve: func(rel string) []string {
			return ResourceCandidates(rel, "_mailer.rb", p)
		}},
		{prefix: cfg.FormersPath, resolve: func(rel string) []string {
			return StemCandidates(rel, "_former.rb", p)
		}},
		{prefix: cfg.ServicesPath, resolve: func(rel string) []string {
			return StemCandidates(rel, "_service.rb", p)
		}},
		{prefix: cfg.DatagridsPath, resolve: func(rel string) []string {
			return ResourceCandidates(rel, "_datagrid.rb", p)
		}},
		{prefix: cfg.TestModelsPath, resolve: func(rel string) []string {
			return ResourceCandidates(rel, "_test.rb", p)
		}},
		{prefix: cfg.SpecModelsPath, resolve: func(rel string) []string {
			return ResourceCandidates(rel, "_spec.rb", p)
		}},
		{prefix: cfg.FixturesPath, resolve: func(rel string) []string {
			return []string{FixtureModelName(rel, p)}
		}},
		{prefix: cfg.SpecFixturesPath, resolve: func(rel string) []string {
			return []string{FixtureModelName(rel, p)}
		}},
		{prefix: cfg.ViewsPath, resolve: func(rel string) []string {
			return ViewCandidates(rel, p)
		}},
	}

	for _, rule := range rules {
		if rel, ok := trimStaticPrefix(cleanedSlash, rule.prefix); ok {
			return resolveExistingCandidate(root, cfg.ModelsPath, input, rule.resolve(rel))
		}
	}

	return "", fmt.Errorf("unsupported related path: %s", input)
}

func trimInputPrefix(root, input, configuredPath string) (string, bool) {
	cleaned := NormalizeInput(input)
	cleanedSlash := filepath.ToSlash(cleaned)
	if filepath.IsAbs(cleaned) {
		if rel, err := filepath.Rel(root, cleaned); err == nil && !strings.HasPrefix(rel, "..") {
			cleanedSlash = filepath.ToSlash(rel)
		}
	}

	if rel, ok := trimStaticPrefix(cleanedSlash, filepath.ToSlash(filepath.Clean(configuredPath))); ok {
		return rel, true
	}
	if rel, ok := trimStaticPrefix(cleanedSlash, filepath.ToSlash(config.ResolvePath(root, configuredPath))); ok {
		return rel, true
	}
	return "", false
}

func trimStaticPrefix(inputSlash, prefix string) (string, bool) {
	prefix = strings.TrimSuffix(filepath.ToSlash(prefix), "/")
	if inputSlash == prefix {
		return "", true
	}
	if strings.HasPrefix(inputSlash, prefix+"/") {
		return strings.TrimPrefix(inputSlash, prefix+"/"), true
	}
	return "", false
}

func resolveExistingCandidate(root, modelsPath, input string, candidates []string) (string, error) {
	var tried []string
	for _, candidate := range candidates {
		if candidate == "" || slices.Contains(tried, candidate) {
			continue
		}
		tried = append(tried, candidate)
		if _, err := model.Resolve(root, modelsPath, candidate); err == nil {
			return candidate, nil
		}
	}
	if len(tried) == 0 {
		return "", fmt.Errorf("unsupported related path: %s", input)
	}
	return "", fmt.Errorf("model file not found for '%s' (tried: %s)", input, strings.Join(tried, ", "))
}

// ResourceCandidates returns model name candidates from a controller/decorator/test path.
func ResourceCandidates(rel, suffix string, p *pluralize.Pluralizer) []string {
	if !strings.HasSuffix(rel, suffix) {
		return nil
	}
	trimmed := strings.TrimSuffix(filepath.ToSlash(rel), suffix)
	dir := filepath.Dir(trimmed)
	base := p.Singularize(filepath.Base(trimmed))
	if dir == "." {
		return []string{base}
	}
	return []string{filepath.ToSlash(filepath.Join(dir, base))}
}

// StemCandidates returns model name candidates from a service/former path by
// progressively trimming underscore-delimited suffixes.
func StemCandidates(rel, suffix string, p *pluralize.Pluralizer) []string {
	if !strings.HasSuffix(rel, suffix) {
		return nil
	}
	trimmed := strings.TrimSuffix(filepath.ToSlash(rel), suffix)
	dir := filepath.Dir(trimmed)
	stem := filepath.Base(trimmed)
	parts := strings.Split(stem, "_")
	var candidates []string
	for i := len(parts); i >= 1; i-- {
		candidateBase := p.Singularize(strings.Join(parts[:i], "_"))
		candidate := candidateBase
		if dir != "." {
			candidate = filepath.ToSlash(filepath.Join(dir, candidateBase))
		}
		if !slices.Contains(candidates, candidate) {
			candidates = append(candidates, candidate)
		}
	}
	return candidates
}

// ViewCandidates returns model name candidates from a view path by
// singularizing each directory segment.
func ViewCandidates(rel string, p *pluralize.Pluralizer) []string {
	parts := strings.Split(filepath.ToSlash(filepath.Dir(rel)), "/")
	if len(parts) == 1 && parts[0] == "." {
		return nil
	}
	var candidates []string
	for i := len(parts) - 1; i >= 0; i-- {
		resource := p.Singularize(parts[i])
		candidate := resource
		if i > 0 {
			candidate = filepath.ToSlash(filepath.Join(filepath.Join(parts[:i]...), resource))
		}
		if !slices.Contains(candidates, candidate) {
			candidates = append(candidates, candidate)
		}
	}
	return candidates
}

// FixtureModelName maps a fixture file path (relative) to its model name.
func FixtureModelName(rel string, p *pluralize.Pluralizer) string {
	slashed := filepath.ToSlash(rel)
	slashed = strings.TrimSuffix(slashed, ".yml")
	trimmed := strings.TrimSuffix(slashed, ".yaml")
	dir := filepath.Dir(trimmed)
	base := p.Singularize(filepath.Base(trimmed))
	if dir == "." {
		return base
	}
	return filepath.ToSlash(filepath.Join(dir, base))
}

// NormalizeInput cleans a user-provided path string.
func NormalizeInput(input string) string {
	input = strings.ReplaceAll(input, "\\", string(filepath.Separator))
	input = strings.ReplaceAll(input, "/", string(filepath.Separator))
	return filepath.Clean(input)
}

// PathPrefixes returns normalized directory prefixes from config for path detection.
func PathPrefixes(cfg config.Config) []string {
	paths := []string{
		cfg.ModelsPath,
		cfg.ControllersPath,
		cfg.ViewsPath,
		cfg.DecoratorsPath,
		cfg.JobsPath,
		cfg.MailersPath,
		cfg.FormersPath,
		cfg.ServicesPath,
		cfg.DatagridsPath,
		cfg.TestModelsPath,
		cfg.TestControllersPath,
		cfg.FixturesPath,
		cfg.SpecModelsPath,
		cfg.SpecControllersPath,
		cfg.SpecFixturesPath,
	}
	var prefixes []string
	for _, path := range paths {
		normalized := strings.TrimSuffix(filepath.ToSlash(filepath.Clean(path)), "/")
		if normalized == "." || normalized == "" {
			continue
		}
		prefixes = append(prefixes, normalized+"/")
	}
	return prefixes
}

// resolveModel maps a .rb file path back to the canonical model name.
func resolveModel(root, modelsPath, input string, p *pluralize.Pluralizer) (string, string, error) {
	resolved, err := model.Resolve(root, modelsPath, input)
	if err != nil {
		return "", "", err
	}
	rel, err := filepath.Rel(config.ResolvePath(root, modelsPath), resolved)
	if err != nil {
		return "", "", fmt.Errorf("model not under models path: %s", input)
	}
	name := strings.TrimSuffix(rel, ".rb")
	return name, p.Pluralize(filepath.Base(name)), nil
}
