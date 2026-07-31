package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/janstol/rails-kit/internal/config"
	"github.com/janstol/rails-kit/internal/railsroot"
	"github.com/janstol/rails-kit/internal/term"
	"github.com/janstol/rails-kit/internal/version"
)

var rootFlag string
var jsonFlag bool
var colorFlag string

var rootCmd = &cobra.Command{
	Use:   "rails-kit",
	Short: "CLI toolkit for Rails projects",
	Long:  "rails-kit provides fast, compiled tools for reading Rails project structure without loading Rails.",
	Version: func() string {
		v, commit, buildDate := version.Info()
		return fmt.Sprintf("%s (commit: %s, built: %s)", v, commit, buildDate)
	}(),
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&rootFlag, "root", "r", "", "Rails root directory (default: auto-detect from CWD)")
	rootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "Output as JSON")
	rootCmd.PersistentFlags().StringVar(&colorFlag, "color", "auto",
		"Color output: auto|always|never. Disabled when NO_COLOR is set, even with always.")
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if _, err := term.ParseMode(colorFlag); err != nil {
			return coded(codeInvalidArgument, err)
		}
		return nil
	}
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
}

// stdoutStyler builds the Styler for the current invocation's stdout. It
// must be called at RunE time, not cached, since --color is only known
// after flag parsing and tests swap the os.Stdout package variable for a
// pipe.
func stdoutStyler() term.Styler {
	if jsonFlag {
		return term.Styler{}
	}
	mode, _ := term.ParseMode(colorFlag) // validated in PersistentPreRunE
	return term.NewStyler(mode, os.Stdout)
}

func printJSON(cmd *cobra.Command, v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(jsonEnvelope{
		SchemaVersion: jsonSchemaVersion,
		Command:       cmd.Name(),
		Data:          v,
	})
}

func Execute() {
	os.Exit(run(os.Args[1:]))
}

// run executes rootCmd against args and returns the process exit code. It is
// the testable core of Execute(): unlike Execute, it never calls os.Exit, so
// tests can invoke it directly and inspect the redirected stdout/stderr.
//
// Known limitation: if cobra's own flag parsing fails (e.g. an unknown flag),
// jsonFlag was never set from args, so the error prints as plain text even
// under an intended --json invocation.
func run(args []string) int {
	rootCmd.SetArgs(args)
	cmd, err := rootCmd.ExecuteC()
	if err != nil {
		name := ""
		if cmd != nil {
			name = cmd.Name()
		}
		fmt.Fprint(os.Stderr, formatExecuteError(name, jsonFlag, err))
		return 1
	}
	return 0
}

// formatExecuteError renders a top-level command error for stderr, as plain
// text or as a jsonErrorEnvelope depending on jsonMode.
func formatExecuteError(cmdName string, jsonMode bool, err error) string {
	if !jsonMode {
		return fmt.Sprintf("Error: %v\n", err)
	}
	code := codeInternal
	var ce *codedError
	if errors.As(err, &ce) {
		code = ce.code
	}
	envelope := jsonErrorEnvelope{
		SchemaVersion: jsonSchemaVersion,
		Command:       cmdName,
		Error:         jsonError{Code: code, Message: err.Error()},
	}
	b, marshalErr := json.MarshalIndent(envelope, "", "  ")
	if marshalErr != nil {
		return fmt.Sprintf("Error: %v\n", err)
	}
	return string(b) + "\n"
}

// resolveRailsRoot returns the Rails root either from --root flag or auto-detection.
func resolveRailsRoot() (string, error) {
	if rootFlag != "" {
		if _, err := os.Stat(filepath.Join(rootFlag, "config", "application.rb")); err != nil {
			return "", coded(codeNotARailsRoot, fmt.Errorf("--root %q is not a Rails root (config/application.rb not found)", rootFlag))
		}
		return rootFlag, nil
	}
	root, err := railsroot.Find()
	if err != nil {
		return "", coded(codeNotARailsRoot, err)
	}
	return root, nil
}

// loadConfig finds the Rails root and loads its config.
func loadConfig() (string, config.Config, error) {
	root, err := resolveRailsRoot()
	if err != nil {
		return "", config.Config{}, err
	}
	cfg, err := config.Load(root)
	if err != nil {
		return "", config.Config{}, fmt.Errorf("loading config: %w", err)
	}
	return root, cfg, nil
}
