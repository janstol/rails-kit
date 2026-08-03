package cmd

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/janstol/rails-kit/internal/concerns"
	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/controllers"
	"github.com/janstol/rails-kit/internal/fixtures"
	"github.com/janstol/rails-kit/internal/gem"
	"github.com/janstol/rails-kit/internal/locales"
	"github.com/janstol/rails-kit/internal/model"
	"github.com/janstol/rails-kit/internal/schema"
)

// completeWithConfig adapts a candidate-lister to cobra's completion signature.
// Any error (not a Rails root, unreadable schema) yields no candidates rather
// than noise on the user's terminal.
func completeWithConfig(
	list func(root string, cfg config.Config, toComplete string) []string,
) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		root, cfg, err := loadConfig()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return list(root, cfg, toComplete), cobra.ShellCompDirectiveNoFileComp
	}
}

func listModelNames(root string, cfg config.Config, _ string) []string {
	names, err := model.ListNames(root, cfg.ModelsPath)
	if err != nil {
		return nil
	}
	return names
}

var completeModelNames = completeWithConfig(listModelNames)

func listControllerNames(root string, cfg config.Config, _ string) []string {
	names, err := controllers.ListNames(root, cfg.ControllersPath, cfg.ControllerConcernsPath)
	if err != nil {
		return nil
	}
	return names
}

var completeControllerNames = completeWithConfig(listControllerNames)

// completeSkeletonArgs offers model names alongside the shell's own file
// completion, since skeleton also accepts paths, directories, and globs.
func completeSkeletonArgs(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	root, cfg, err := loadConfig()
	if err != nil {
		return nil, cobra.ShellCompDirectiveDefault
	}
	return excludeArgs(listModelNames(root, cfg, ""), args), cobra.ShellCompDirectiveDefault
}

func listSchemaTables(root string, cfg config.Config, _ string) []string {
	schemaPath := config.ResolveSchemaPath(root, cfg)
	tables, err := schema.ListTables(schemaPath)
	if err != nil {
		return nil
	}
	return tables
}

func completeSchemaArgs(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	root, cfg, err := loadConfig()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return excludeArgs(listSchemaTables(root, cfg, ""), args), cobra.ShellCompDirectiveNoFileComp
}

func listConcernNames(root string, cfg config.Config, _ string) []string {
	modelDir := config.ResolvePath(root, cfg.ModelConcernsPath)
	ctrlDir := config.ResolvePath(root, cfg.ControllerConcernsPath)
	modelNames, err := concerns.ListFiles(modelDir)
	if err != nil {
		modelNames = nil
	}
	ctrlNames, err := concerns.ListFiles(ctrlDir)
	if err != nil {
		ctrlNames = nil
	}
	seen := make(map[string]bool, len(modelNames)+len(ctrlNames))
	var names []string
	for _, n := range modelNames {
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	for _, n := range ctrlNames {
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return names
}

var completeConcernNames = completeWithConfig(listConcernNames)

func listFixtureNames(root string, cfg config.Config, _ string) []string {
	fixturesDir := config.ResolvePath(root, cfg.FixturesPath)
	names, err := fixtures.ListFiles(fixturesDir)
	if err != nil {
		return nil
	}
	return names
}

var completeFixtureNames = completeWithConfig(listFixtureNames)

func listGemNames(root string, cfg config.Config, _ string) []string {
	lockPath := config.ResolvePath(root, cfg.GemfileLockPath)
	lockfile, err := gem.Parse(lockPath)
	if err != nil {
		return nil
	}
	gems := lockfile.List()
	names := make([]string, len(gems))
	for i, g := range gems {
		names[i] = g.Name
	}
	return names
}

var completeGemNames = completeWithConfig(listGemNames)

// completeLocalesScope completes one dotted scope segment at a time rather
// than flattening the whole locale tree, which can run to thousands of keys
// on a real app. toComplete is split on the last '.'; everything before it
// is a settled scope that gets navigated to, and its immediate children
// become the candidates.
func completeLocalesScope(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	root, cfg, err := loadConfig()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	localesDir := config.ResolvePath(root, cfg.LocalesPath)
	merged, err := locales.Load(localesDir)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	idx := strings.LastIndex(toComplete, ".")
	if idx == -1 {
		return sortedTopLevelKeys(merged), cobra.ShellCompDirectiveNoFileComp
	}

	prefix := toComplete[:idx]
	node, err := locales.Navigate(merged, prefix)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	m, ok := node.(map[string]interface{})
	if !ok {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, prefix+"."+k)
	}
	sort.Strings(keys)
	return keys, cobra.ShellCompDirectiveNoFileComp
}

func sortedTopLevelKeys(merged map[string]interface{}) []string {
	keys := make([]string, 0, len(merged))
	for k := range merged {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// excludeArgs drops candidates already present in args, so a second Tab on a
// repeatable positional (schema's tables, skeleton's inputs) doesn't re-offer
// a value already on the command line.
func excludeArgs(candidates, args []string) []string {
	if len(args) == 0 {
		return candidates
	}
	seen := make(map[string]bool, len(args))
	for _, a := range args {
		seen[a] = true
	}
	filtered := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if !seen[c] {
			filtered = append(filtered, c)
		}
	}
	return filtered
}
