package config

import (
	"bytes"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds optional project-specific settings from .rails-kit.yml.
type Config struct {
	SchemaPath          string            `yaml:"schema_path"`
	FixturesPath        string            `yaml:"fixtures_path"`
	LocalesPath         string            `yaml:"locales_path"`
	ModelsPath          string            `yaml:"models_path"`
	ControllersPath     string            `yaml:"controllers_path"`
	ViewsPath           string            `yaml:"views_path"`
	DecoratorsPath      string            `yaml:"decorators_path"`
	FormersPath         string            `yaml:"formers_path"`
	ServicesPath        string            `yaml:"services_path"`
	DatagridsPath       string            `yaml:"datagrids_path"`
	JobsPath            string            `yaml:"jobs_path"`
	MailersPath         string            `yaml:"mailers_path"`
	TestModelsPath      string            `yaml:"test_models_path"`
	TestControllersPath string            `yaml:"test_controllers_path"`
	SpecModelsPath      string            `yaml:"spec_models_path"`
	SpecControllersPath string            `yaml:"spec_controllers_path"`
	SpecFixturesPath    string            `yaml:"spec_fixtures_path"`
	SpecRequestsPath    string            `yaml:"spec_requests_path"`
	SpecSystemPath      string            `yaml:"spec_system_path"`
	SpecHelpersPath     string            `yaml:"spec_helpers_path"`
	SpecJobsPath        string            `yaml:"spec_jobs_path"`
	SpecMailersPath     string            `yaml:"spec_mailers_path"`
	SpecServicesPath    string            `yaml:"spec_services_path"`
	GemfileLockPath        string            `yaml:"gemfile_lock_path"`
	ModelConcernsPath      string            `yaml:"model_concerns_path"`
	ControllerConcernsPath string            `yaml:"controller_concerns_path"`
	Plurals                map[string]string `yaml:"plurals"`
}

// Defaults returns a Config populated with conventional defaults.
func Defaults() Config {
	return Config{
		SchemaPath:          "db/schema.rb",
		FixturesPath:        "test/fixtures",
		LocalesPath:         "config/locales",
		ModelsPath:          "app/models",
		ControllersPath:     "app/controllers",
		ViewsPath:           "app/views",
		DecoratorsPath:      "app/decorators",
		FormersPath:         "app/formers",
		ServicesPath:        "app/services",
		DatagridsPath:       "app/datagrids",
		JobsPath:            "app/jobs",
		MailersPath:         "app/mailers",
		TestModelsPath:      "test/models",
		TestControllersPath: "test/controllers",
		SpecModelsPath:      "spec/models",
		SpecControllersPath: "spec/controllers",
		SpecFixturesPath:    "spec/fixtures",
		SpecRequestsPath:    "spec/requests",
		SpecSystemPath:      "spec/system",
		SpecHelpersPath:     "spec/helpers",
		SpecJobsPath:        "spec/jobs",
		SpecMailersPath:     "spec/mailers",
		SpecServicesPath:    "spec/services",
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
	if err := dec.Decode(&cfg); err != nil && err != io.EOF {
		return cfg, err
	}
	// Fill defaults for any empty fields
	if cfg.SchemaPath == "" {
		cfg.SchemaPath = "db/schema.rb"
	}
	if cfg.FixturesPath == "" {
		cfg.FixturesPath = "test/fixtures"
	}
	if cfg.LocalesPath == "" {
		cfg.LocalesPath = "config/locales"
	}
	if cfg.ModelsPath == "" {
		cfg.ModelsPath = "app/models"
	}
	if cfg.ControllersPath == "" {
		cfg.ControllersPath = "app/controllers"
	}
	if cfg.ViewsPath == "" {
		cfg.ViewsPath = "app/views"
	}
	if cfg.DecoratorsPath == "" {
		cfg.DecoratorsPath = "app/decorators"
	}
	if cfg.FormersPath == "" {
		cfg.FormersPath = "app/formers"
	}
	if cfg.ServicesPath == "" {
		cfg.ServicesPath = "app/services"
	}
	if cfg.DatagridsPath == "" {
		cfg.DatagridsPath = "app/datagrids"
	}
	if cfg.JobsPath == "" {
		cfg.JobsPath = "app/jobs"
	}
	if cfg.MailersPath == "" {
		cfg.MailersPath = "app/mailers"
	}
	if cfg.TestModelsPath == "" {
		cfg.TestModelsPath = "test/models"
	}
	if cfg.TestControllersPath == "" {
		cfg.TestControllersPath = "test/controllers"
	}
	if cfg.SpecModelsPath == "" {
		cfg.SpecModelsPath = "spec/models"
	}
	if cfg.SpecControllersPath == "" {
		cfg.SpecControllersPath = "spec/controllers"
	}
	if cfg.SpecFixturesPath == "" {
		cfg.SpecFixturesPath = "spec/fixtures"
	}
	if cfg.SpecRequestsPath == "" {
		cfg.SpecRequestsPath = "spec/requests"
	}
	if cfg.SpecSystemPath == "" {
		cfg.SpecSystemPath = "spec/system"
	}
	if cfg.SpecHelpersPath == "" {
		cfg.SpecHelpersPath = "spec/helpers"
	}
	if cfg.SpecJobsPath == "" {
		cfg.SpecJobsPath = "spec/jobs"
	}
	if cfg.SpecMailersPath == "" {
		cfg.SpecMailersPath = "spec/mailers"
	}
	if cfg.SpecServicesPath == "" {
		cfg.SpecServicesPath = "spec/services"
	}
	if cfg.GemfileLockPath == "" {
		cfg.GemfileLockPath = "Gemfile.lock"
	}
	if cfg.ModelConcernsPath == "" {
		cfg.ModelConcernsPath = "app/models/concerns"
	}
	if cfg.ControllerConcernsPath == "" {
		cfg.ControllerConcernsPath = "app/controllers/concerns"
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
