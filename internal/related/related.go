package related

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/janstol/rails-kit/internal/config"
)

// Category represents a group of related files.
type Category struct {
	Label string
	Files []string
}

// Find returns file categories for the given model name, relative to railsRoot.
func Find(railsRoot string, cfg config.Config, name, plural string) ([]Category, error) {
	r := railsRoot
	modelsDir := config.ResolvePath(railsRoot, cfg.ModelsPath)
	fixturesDir := config.ResolvePath(railsRoot, cfg.FixturesPath)
	specFixturesDir := config.ResolvePath(railsRoot, cfg.SpecFixturesPath)

	namespace := ""
	baseName := name
	if i := strings.LastIndex(name, "/"); i >= 0 {
		namespace = name[:i]
		baseName = name[i+1:]
	}

	defs := []struct {
		label string
		find  func() ([]string, error)
	}{
		{"Model", func() ([]string, error) {
			return exactGlob(filepath.Join(modelsDir, name+".rb"))
		}},
		{"Controller", func() ([]string, error) {
			return walkMatchNS(config.ResolvePath(r, cfg.ControllersPath), namespace, plural+"_controller.rb")
		}},
		{"Views", func() ([]string, error) {
			return walkMatchDirNS(config.ResolvePath(r, cfg.ViewsPath), namespace, plural)
		}},
		{"Decorator", func() ([]string, error) {
			return exactGlob(filepath.Join(config.ResolvePath(r, cfg.DecoratorsPath), name+"_decorator.rb"))
		}},
		{"Job", func() ([]string, error) {
			return exactGlob(filepath.Join(config.ResolvePath(r, cfg.JobsPath), name+"_job.rb"))
		}},
		{"Mailer", func() ([]string, error) {
			return exactGlob(filepath.Join(config.ResolvePath(r, cfg.MailersPath), name+"_mailer.rb"))
		}},
		{"Former", func() ([]string, error) {
			root := config.ResolvePath(r, cfg.FormersPath)
			results, err := WalkMatchSegment(root, baseName)
			if err != nil {
				return results, err
			}
			return filterByNamespaceOrModelDir(root, namespace, baseName, results), nil
		}},
		{"Service", func() ([]string, error) {
			root := config.ResolvePath(r, cfg.ServicesPath)
			results, err := WalkMatchSegment(root, baseName)
			if err != nil {
				return results, err
			}
			return filterByNamespaceOrModelDir(root, namespace, baseName, results), nil
		}},
		{"Datagrid", func() ([]string, error) {
			return exactGlob(filepath.Join(config.ResolvePath(r, cfg.DatagridsPath), namespace, plural+"_datagrid.rb"))
		}},
		{"Model test", func() ([]string, error) {
			return exactGlob(filepath.Join(config.ResolvePath(r, cfg.TestModelsPath), name+"_test.rb"))
		}},
		{"Controller test", func() ([]string, error) {
			return walkMatchNS(config.ResolvePath(r, cfg.TestControllersPath), namespace, plural+"_controller_test.rb")
		}},
		{"Model spec", func() ([]string, error) {
			return exactGlob(filepath.Join(config.ResolvePath(r, cfg.SpecModelsPath), name+"_spec.rb"))
		}},
		{"Controller spec", func() ([]string, error) {
			return walkMatchNS(config.ResolvePath(r, cfg.SpecControllersPath), namespace, plural+"_controller_spec.rb")
		}},
		{"Request spec", func() ([]string, error) {
			return walkMatchNS(config.ResolvePath(r, cfg.SpecRequestsPath), namespace, plural+"_spec.rb")
		}},
		{"System spec", func() ([]string, error) {
			return walkMatchNS(config.ResolvePath(r, cfg.SpecSystemPath), namespace, plural+"_spec.rb")
		}},
		{"Helper spec", func() ([]string, error) {
			return walkMatchNS(config.ResolvePath(r, cfg.SpecHelpersPath), namespace, plural+"_helper_spec.rb")
		}},
		{"Job spec", func() ([]string, error) {
			return exactGlob(filepath.Join(config.ResolvePath(r, cfg.SpecJobsPath), name+"_job_spec.rb"))
		}},
		{"Mailer spec", func() ([]string, error) {
			return exactGlob(filepath.Join(config.ResolvePath(r, cfg.SpecMailersPath), name+"_mailer_spec.rb"))
		}},
		{"Service spec", func() ([]string, error) {
			root := config.ResolvePath(r, cfg.SpecServicesPath)
			results, err := WalkMatchSegment(root, baseName)
			if err != nil {
				return results, err
			}
			return filterByNamespaceOrModelDir(root, namespace, baseName, results), nil
		}},
		{"Fixtures", func() ([]string, error) {
			if f, err := exactGlob(filepath.Join(fixturesDir, filepath.Dir(name), plural+".yml")); err != nil || len(f) > 0 {
				return f, err
			}
			if f, err := exactGlob(filepath.Join(fixturesDir, filepath.Dir(name), plural+".yaml")); err != nil || len(f) > 0 {
				return f, err
			}
			if f, err := exactGlob(filepath.Join(specFixturesDir, filepath.Dir(name), plural+".yml")); err != nil || len(f) > 0 {
				return f, err
			}
			return exactGlob(filepath.Join(specFixturesDir, filepath.Dir(name), plural+".yaml"))
		}},
	}

	var categories []Category
	for _, d := range defs {
		files, err := d.find()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", d.label, err)
		}
		if len(files) == 0 {
			continue
		}
		sort.Strings(files)
		var rel []string
		for _, f := range files {
			r2, err := filepath.Rel(railsRoot, f)
			if err != nil || r2 == ".." || strings.HasPrefix(r2, ".."+string(filepath.Separator)) {
				r2 = f
			}
			rel = append(rel, filepath.ToSlash(r2))
		}
		categories = append(categories, Category{Label: d.label, Files: rel})
	}
	return categories, nil
}

// exactGlob returns a slice with the path if it exists, or empty slice.
func exactGlob(path string) ([]string, error) {
	if _, err := os.Stat(path); err == nil {
		return []string{path}, nil
	}
	return nil, nil
}

// walkMatchNS finds files with the given filename under root.
// Matches are limited to the exact requested namespace, not nested namespaces.
func walkMatchNS(root, namespace, filename string) ([]string, error) {
	if _, err := os.Stat(root); err != nil {
		return nil, nil
	}
	var matches []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Base(path) != filename {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		if !matchesExactNamespace(rel, namespace) {
			return nil
		}
		matches = append(matches, path)
		return nil
	})
	return matches, err
}

// walkMatchDirNS finds all regular files within the matching view directory.
// Matches are limited to the exact requested namespace, not nested namespaces.
func walkMatchDirNS(root, namespace, dirName string) ([]string, error) {
	if _, err := os.Stat(root); err != nil {
		return nil, nil
	}
	var matches []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == dirName {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return nil
			}
			if !matchesExactNamespace(rel, namespace) {
				return nil
			}
			if walkErr := filepath.WalkDir(path, func(subpath string, subd fs.DirEntry, suberr error) error {
				if suberr != nil {
					return suberr
				}
				if !subd.IsDir() {
					matches = append(matches, subpath)
				}
				return nil
			}); walkErr != nil {
				return walkErr
			}
		}
		return nil
	})
	return matches, err
}

func matchesExactNamespace(rel, namespace string) bool {
	dir := filepath.ToSlash(filepath.Dir(rel))
	if namespace == "" {
		return dir == "."
	}
	return dir == namespace
}

// filterByNamespaceOrModelDir returns paths in the exact namespace, or in a
// subdirectory named after the model (baseName) within that namespace.
// This allows matching files like app/services/user/export_service.rb for model "user".
func filterByNamespaceOrModelDir(root, namespace, baseName string, paths []string) []string {
	var filtered []string
	var modelDir string
	if namespace == "" {
		modelDir = baseName
	} else {
		modelDir = namespace + "/" + baseName
	}
	for _, p := range paths {
		rel, err := filepath.Rel(root, p)
		if err != nil {
			continue
		}
		dir := filepath.ToSlash(filepath.Dir(rel))
		if matchesExactNamespace(rel, namespace) || dir == modelDir {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// WalkMatchSegment finds files whose base name contains name as a complete _-delimited segment,
// or whose containing directory path has a segment that exactly equals name.
func WalkMatchSegment(root, name string) ([]string, error) {
	if _, err := os.Stat(root); err != nil {
		return nil, nil
	}
	var matches []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".rb") {
			base := filepath.Base(path)
			if strings.HasPrefix(base, name) {
				rest := base[len(name):]
				if rest == ".rb" || strings.HasPrefix(rest, "_") {
					matches = append(matches, path)
					return nil
				}
			}
			// Also match when any directory segment of the relative path exactly equals name.
			rel, relErr := filepath.Rel(root, path)
			if relErr == nil {
				for _, seg := range strings.Split(filepath.ToSlash(filepath.Dir(rel)), "/") {
					if seg == name {
						matches = append(matches, path)
						break
					}
				}
			}
		}
		return nil
	})
	return matches, err
}

// defaultRailsPrefixes lists standard Rails directory prefixes that are not part
// of the model name and should be stripped when normalizing a file path.
var defaultRailsPrefixes = []string{
	"app/models/",
	"app/controllers/",
	"app/views/",
	"app/decorators/",
	"app/jobs/",
	"app/mailers/",
	"app/formers/",
	"app/services/",
	"app/datagrids/",
	"test/models/",
	"test/controllers/",
	"test/fixtures/",
	"spec/models/",
	"spec/controllers/",
	"spec/fixtures/",
	"spec/requests/",
	"spec/system/",
	"spec/helpers/",
	"spec/jobs/",
	"spec/mailers/",
	"spec/services/",
}

// NormalizeName strips known Rails path prefixes and file suffixes to produce a
// model name that preserves full namespace depth. It uses the default prefix list.
// Pass extra prefixes via NormalizeNameWithPrefixes if config-aware stripping is needed.
// Examples:
//
//	"app/models/admin/billing/invoice.rb" → "admin/billing/invoice"
//	"admin/billing/invoice.rb"            → "admin/billing/invoice"
//	"admin/user.rb"                       → "admin/user"
//	"user.rb"                             → "user"
func NormalizeName(input string) string {
	return NormalizeNameWithPrefixes(input, nil)
}

// NormalizeNameWithPrefixes is like NormalizeName but also strips the provided
// extra prefixes (e.g. custom paths from config). The extra list is checked
// before the default Rails prefixes.
func NormalizeNameWithPrefixes(input string, extra []string) string {
	name := strings.TrimSuffix(input, ".rb")
	name = strings.TrimSuffix(name, ".yml")
	name = strings.TrimSuffix(name, ".yaml")

	// Strip known Rails prefixes before extracting namespace so that
	// multi-level namespaces like "admin/billing" are preserved.
	normalized := filepath.ToSlash(strings.ToLower(name))
	for _, prefix := range append(extra, defaultRailsPrefixes...) {
		p := strings.TrimSuffix(filepath.ToSlash(prefix), "/") + "/"
		if strings.HasPrefix(normalized, p) {
			name = name[len(p):]
			break
		}
	}

	base := filepath.Base(name)
	for _, suffix := range []string{"_controller_test", "_controller_spec", "_controller", "_test", "_helper_spec", "_job_spec", "_mailer_spec", "_service_spec", "_spec", "_decorator", "_datagrid", "_former", "_service", "_job", "_mailer"} {
		if strings.HasSuffix(base, suffix) {
			base = strings.TrimSuffix(base, suffix)
			break
		}
	}
	dir := filepath.ToSlash(filepath.Dir(name))
	if dir != "." {
		return strings.ToLower(dir) + "/" + strings.ToLower(base)
	}
	return strings.ToLower(base)
}
