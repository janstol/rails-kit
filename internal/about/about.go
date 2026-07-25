package about

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/gem"
)

type Versions struct {
	Rails    string `json:"rails,omitempty"`
	Ruby     string `json:"ruby,omitempty"`
	RubyGems string `json:"rubygems,omitempty"`
	Rack     string `json:"rack,omitempty"`
	Bundler  string `json:"bundler,omitempty"`
}

type Database struct {
	Adapters     []string `json:"adapters,omitempty"`
	SchemaFormat string   `json:"schema_format,omitempty"`
}

type Report struct {
	Application string   `json:"application,omitempty"`
	Root        string   `json:"root"`
	Environment string   `json:"environment"`
	Source      string   `json:"source"`
	Versions    Versions `json:"versions"`
	Database    Database `json:"database"`
	Warnings    []string `json:"warnings,omitempty"`
}

var (
	classApplication = regexp.MustCompile(`(?m)^\s*class\s+([A-Z]\w*(?:::[A-Z]\w*)*)::Application\s*<\s*Rails::Application`)
	moduleName       = regexp.MustCompile(`(?m)^\s*module\s+([A-Z]\w*(?:::[A-Z]\w*)*)\s*$`)
	appClass         = regexp.MustCompile(`(?m)^\s*class\s+Application\s*<\s*Rails::Application`)
	adapterLine      = regexp.MustCompile(`^\s*adapter:\s*([A-Za-z0-9_]+)\s*(?:#.*)?$`)
	miseRuby         = regexp.MustCompile(`(?m)^\s*ruby\s*=\s*["']([^"']+)["']`)
)

// Inspect reads project metadata without loading Rails or evaluating project code.
func Inspect(root string, cfg config.Config) Report {
	report := Report{Root: root, Environment: environment(), Source: "static"}

	appData, err := os.ReadFile(filepath.Join(root, "config", "application.rb"))
	if err != nil {
		report.Warnings = append(report.Warnings, "application name unavailable: "+err.Error())
	} else if report.Application = applicationName(string(appData)); report.Application == "" {
		report.Warnings = append(report.Warnings, "application name unavailable: Rails application class not found")
	}

	lock, err := gem.Parse(config.ResolvePath(root, cfg.GemfileLockPath))
	if err != nil {
		report.Warnings = append(report.Warnings, "dependency versions unavailable: "+err.Error())
	} else {
		report.Versions.Rails = gemVersion(lock, "rails")
		report.Versions.Rack = gemVersion(lock, "rack")
		report.Versions.Ruby = lock.RubyVersion()
		report.Versions.Bundler = lock.BundlerVersion()
	}
	if report.Versions.Ruby == "" {
		report.Versions.Ruby = rubyVersionFallback(root)
		if report.Versions.Ruby == "" {
			report.Warnings = append(report.Warnings, "Ruby version unavailable")
		}
	}

	report.Database.Adapters, err = databaseAdapters(filepath.Join(root, "config", "database.yml"))
	if err != nil {
		report.Warnings = append(report.Warnings, "database adapters unavailable: "+err.Error())
	} else if len(report.Database.Adapters) == 0 {
		report.Warnings = append(report.Warnings, "database adapters unavailable: no literal adapter declarations found")
	}

	schemaPath := config.ResolveSchemaPath(root, cfg)
	if _, err := os.Stat(schemaPath); err != nil {
		report.Warnings = append(report.Warnings, "schema format unavailable: "+err.Error())
	} else {
		switch filepath.Ext(schemaPath) {
		case ".rb":
			report.Database.SchemaFormat = "ruby"
		case ".sql":
			report.Database.SchemaFormat = "sql"
		default:
			report.Warnings = append(report.Warnings, "schema format unavailable: unrecognized schema extension")
		}
	}
	return report
}

func applicationName(source string) string {
	if match := classApplication.FindStringSubmatch(source); match != nil {
		return match[1]
	}
	if appClass.MatchString(source) {
		before := source[:appClass.FindStringIndex(source)[0]]
		matches := moduleName.FindAllStringSubmatch(before, -1)
		if len(matches) > 0 {
			return matches[len(matches)-1][1]
		}
	}
	return ""
}

func environment() string {
	if value := os.Getenv("RAILS_ENV"); value != "" {
		return value
	}
	if value := os.Getenv("RACK_ENV"); value != "" {
		return value
	}
	return "development"
}

func gemVersion(lock *gem.Lockfile, name string) string {
	if found := lock.Find(name); found != nil {
		return found.Version
	}
	return ""
}

func rubyVersionFallback(root string) string {
	if data, err := os.ReadFile(filepath.Join(root, ".ruby-version")); err == nil {
		return strings.TrimPrefix(strings.TrimSpace(string(data)), "ruby-")
	}
	if data, err := os.ReadFile(filepath.Join(root, ".tool-versions")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[0] == "ruby" {
				return fields[1]
			}
		}
	}
	if data, err := os.ReadFile(filepath.Join(root, "mise.toml")); err == nil {
		if match := miseRuby.FindStringSubmatch(string(data)); match != nil {
			return match[1]
		}
	}
	return ""
}

func databaseAdapters(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()
	seen := make(map[string]bool)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if match := adapterLine.FindStringSubmatch(line); match != nil {
			seen[match[1]] = true
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	adapters := make([]string, 0, len(seen))
	for adapter := range seen {
		adapters = append(adapters, adapter)
	}
	sort.Strings(adapters)
	return adapters, nil
}

// Format renders a human-readable report.
func Format(report Report) string {
	var builder strings.Builder
	writer := tabwriter.NewWriter(&builder, 0, 4, 2, ' ', 0)
	rows := [][2]string{
		{"Application", report.Application}, {"Application root", report.Root},
		{"Environment", report.Environment}, {"Source", report.Source},
		{"Rails version", report.Versions.Rails}, {"Ruby version", report.Versions.Ruby},
		{"RubyGems version", report.Versions.RubyGems}, {"Rack version", report.Versions.Rack},
		{"Bundler version", report.Versions.Bundler},
		{"Database adapters", strings.Join(report.Database.Adapters, ", ")},
		{"Schema format", report.Database.SchemaFormat},
	}
	for _, row := range rows {
		if row[1] != "" {
			_, _ = fmt.Fprintf(writer, "%s:\t%s\n", row[0], row[1])
		}
	}
	_ = writer.Flush()
	if len(report.Warnings) > 0 {
		builder.WriteString("\nWarnings:\n")
		for _, warning := range report.Warnings {
			builder.WriteString("- " + warning + "\n")
		}
	}
	return builder.String()
}
