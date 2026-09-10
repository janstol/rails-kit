package config

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds optional project-specific settings from .rails-kit.yml.
type Config struct {
	SchemaPath             string            `yaml:"schema_path"`
	FixturesPath           string            `yaml:"fixtures_path"`
	LocalesPath            string            `yaml:"locales_path"`
	ModelsPath             string            `yaml:"models_path"`
	ControllersPath        string            `yaml:"controllers_path"`
	ViewsPath              string            `yaml:"views_path"`
	DecoratorsPath         string            `yaml:"decorators_path"`
	FormersPath            string            `yaml:"formers_path"`
	PresentersPath         string            `yaml:"presenters_path"`
	ServicesPath           string            `yaml:"services_path"`
	HelpersPath            string            `yaml:"helpers_path"`
	DatagridsPath          string            `yaml:"datagrids_path"`
	JobsPath               string            `yaml:"jobs_path"`
	MailersPath            string            `yaml:"mailers_path"`
	TestModelsPath         string            `yaml:"test_models_path"`
	TestControllersPath    string            `yaml:"test_controllers_path"`
	TestSystemPath         string            `yaml:"test_system_path"`
	TestHelpersPath        string            `yaml:"test_helpers_path"`
	TestJobsPath           string            `yaml:"test_jobs_path"`
	TestMailersPath        string            `yaml:"test_mailers_path"`
	TestServicesPath       string            `yaml:"test_services_path"`
	SpecModelsPath         string            `yaml:"spec_models_path"`
	SpecControllersPath    string            `yaml:"spec_controllers_path"`
	SpecFixturesPath       string            `yaml:"spec_fixtures_path"`
	SpecRequestsPath       string            `yaml:"spec_requests_path"`
	SpecSystemPath         string            `yaml:"spec_system_path"`
	SpecHelpersPath        string            `yaml:"spec_helpers_path"`
	SpecJobsPath           string            `yaml:"spec_jobs_path"`
	SpecMailersPath        string            `yaml:"spec_mailers_path"`
	SpecServicesPath       string            `yaml:"spec_services_path"`
	GemfileLockPath        string            `yaml:"gemfile_lock_path"`
	ModelConcernsPath      string            `yaml:"model_concerns_path"`
	ControllerConcernsPath string            `yaml:"controller_concerns_path"`
	Plurals                map[string]string `yaml:"plurals"`
}

// Defaults returns a Config populated with conventional defaults.
func Defaults() Config {
	return Config{
		SchemaPath:             "db/schema.rb",
		FixturesPath:           "test/fixtures",
		LocalesPath:            "config/locales",
		ModelsPath:             "app/models",
		ControllersPath:        "app/controllers",
		ViewsPath:              "app/views",
		DecoratorsPath:         "app/decorators",
		FormersPath:            "app/formers",
		PresentersPath:         "app/presenters",
		ServicesPath:           "app/services",
		HelpersPath:            "app/helpers",
		DatagridsPath:          "app/datagrids",
		JobsPath:               "app/jobs",
		MailersPath:            "app/mailers",
		TestModelsPath:         "test/models",
		TestControllersPath:    "test/controllers",
		TestSystemPath:         "test/system",
		TestHelpersPath:        "test/helpers",
		TestJobsPath:           "test/jobs",
		TestMailersPath:        "test/mailers",
		TestServicesPath:       "test/services",
		SpecModelsPath:         "spec/models",
		SpecControllersPath:    "spec/controllers",
		SpecFixturesPath:       "spec/fixtures",
		SpecRequestsPath:       "spec/requests",
		SpecSystemPath:         "spec/system",
		SpecHelpersPath:        "spec/helpers",
		SpecJobsPath:           "spec/jobs",
		SpecMailersPath:        "spec/mailers",
		SpecServicesPath:       "spec/services",
		GemfileLockPath:        "Gemfile.lock",
		ModelConcernsPath:      "app/models/concerns",
		ControllerConcernsPath: "app/controllers/concerns",
	}
}

// Load reads .rails-kit.yml from railsRoot and merges it over defaults.
// If the file does not exist, defaults are returned without error.
func Load(railsRoot string) (Config, error) {
	cfg := Defaults()
	path := filepath.Join(railsRoot, ".rails-kit.yml")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
		return cfg, err
	}
	// Fill defaults for any field the file set to an explicit empty string.
	// yaml.v3 leaves absent keys untouched, so this only matters for `key: ""`.
	defaults := reflect.ValueOf(Defaults())
	v := reflect.ValueOf(&cfg).Elem()
	for i := range v.NumField() {
		f := v.Field(i)
		if f.Kind() == reflect.String && f.String() == "" {
			f.SetString(defaults.Field(i).String())
		}
	}
	return cfg, nil
}

// ResolvePath returns path as-is when absolute, otherwise relative to railsRoot.
func ResolvePath(railsRoot, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(railsRoot, path)
}

// ResolveSchemaPath returns the resolved schema file path.
// If the configured path does not exist on disk, it tries the alternate format
// (schema.rb ↔ structure.sql). If neither exists, the primary path is returned
// so that Parse produces the expected "cannot open schema" error.
func ResolveSchemaPath(railsRoot string, cfg Config) string {
	primary := ResolvePath(railsRoot, cfg.SchemaPath)
	if fileExists(primary) {
		return primary
	}
	alt := ResolvePath(railsRoot, alternateSchemaPath(cfg.SchemaPath))
	if fileExists(alt) {
		return alt
	}
	return primary
}

func alternateSchemaPath(path string) string {
	if strings.HasSuffix(path, "schema.rb") {
		return strings.TrimSuffix(path, "schema.rb") + "structure.sql"
	}
	if strings.HasSuffix(path, "structure.sql") {
		return strings.TrimSuffix(path, "structure.sql") + "schema.rb"
	}
	return path
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
