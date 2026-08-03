package cmd

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

var updateGolden = flag.Bool("update", false, "write golden files instead of comparing against them")

// goldenCase describes one command invocation whose whole stdout (and, if
// non-empty, stderr) is pinned against a file under cmd/testdata/golden.
type goldenCase struct {
	name  string // golden basename, without extension
	cmd   *cobra.Command
	args  []string
	flags map[string]string // cobra flag name -> value; set before the run, restored after
	json  bool
}

func TestGolden(t *testing.T) {
	goldenDir, err := filepath.Abs("testdata/golden")
	if err != nil {
		t.Fatal(err)
	}
	fixtureRoot, err := filepath.Abs("../testdata")
	if err != nil {
		t.Fatal(err)
	}

	cases := []goldenCase{
		{name: "schema_list", cmd: schemaCmd},
		{name: "schema_list_json", cmd: schemaCmd, json: true},
		{name: "schema_extract", cmd: schemaCmd, args: []string{"users", "posts"}},
		{name: "schema_extract_json", cmd: schemaCmd, args: []string{"users", "posts"}, json: true},

		{name: "model_user", cmd: modelCmd, args: []string{"user"}},
		{name: "model_user_json", cmd: modelCmd, args: []string{"user"}, json: true},

		{name: "routes_static", cmd: routesCmd, flags: map[string]string{"static": "true"}},
		{name: "routes_static_json", cmd: routesCmd, flags: map[string]string{"static": "true"}, json: true},

		{name: "concerns_list", cmd: concernsCmd},
		{name: "concerns_list_json", cmd: concernsCmd, json: true},
		{name: "concerns_searchable", cmd: concernsCmd, args: []string{"model/searchable"}},
		{name: "concerns_searchable_json", cmd: concernsCmd, args: []string{"model/searchable"}, json: true},

		{name: "locales_list", cmd: localesCmd},
		{name: "locales_list_json", cmd: localesCmd, json: true},
		{name: "locales_scope", cmd: localesCmd, args: []string{"en.views.users"}},
		{name: "locales_scope_json", cmd: localesCmd, args: []string{"en.views.users"}, json: true},

		{name: "skeleton_user", cmd: skeletonCmd, args: []string{"app/models/user.rb"}},
		{name: "skeleton_user_json", cmd: skeletonCmd, args: []string{"app/models/user.rb"}, json: true},

		{name: "fixtures_list", cmd: fixturesCmd},
		{name: "fixtures_list_json", cmd: fixturesCmd, json: true},
		{name: "fixtures_users", cmd: fixturesCmd, args: []string{"users"}},
		{name: "fixtures_users_json", cmd: fixturesCmd, args: []string{"users"}, json: true},

		{name: "gem_list", cmd: gemCmd},
		{name: "gem_list_json", cmd: gemCmd, json: true},
		{name: "gem_rails", cmd: gemCmd, args: []string{"rails"}},
		{name: "gem_rails_json", cmd: gemCmd, args: []string{"rails"}, json: true},

		{name: "related_user", cmd: relatedCmd, args: []string{"user"}},
		{name: "related_user_json", cmd: relatedCmd, args: []string{"user"}, json: true},

		{name: "controllers_list", cmd: controllersCmd},
		{name: "controllers_list_json", cmd: controllersCmd, json: true},
		{name: "controllers_users", cmd: controllersCmd, args: []string{"users"}},
		{name: "controllers_users_json", cmd: controllersCmd, args: []string{"users"}, json: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			applyGoldenFlags(t, tc.cmd, tc.flags)

			var out, errOut string
			var runErr error
			if tc.json {
				out, errOut, runErr = runCmdForTestJSON(t, tc.cmd, fixtureRoot, tc.args)
			} else {
				out, errOut, runErr = runCmdForTest(t, tc.cmd, fixtureRoot, tc.args)
			}
			if runErr != nil {
				t.Fatalf("unexpected error: %v\nstderr:%s", runErr, errOut)
			}

			got := out
			if errOut != "" {
				got += "--- stderr ---\n" + errOut
			}
			// skeleton --json embeds the absolute input path (prism.File.Path)
			// alongside the root-relative one; normalize it so the golden is
			// portable across machines and CI, where the fixture's absolute
			// path differs.
			got = strings.ReplaceAll(got, fixtureRoot, "<fixture-root>")
			got = strings.ReplaceAll(got, filepath.ToSlash(fixtureRoot), "<fixture-root>")

			compareGolden(t, filepath.Join(goldenDir, tc.name+".golden"), got)
		})
	}
}

// applyGoldenFlags sets cobra flags on c for the duration of the test,
// restoring their prior values on cleanup. Needed for command-specific flag
// vars (e.g. routesStatic) that runCmdForTest does not know about.
func applyGoldenFlags(t *testing.T, c *cobra.Command, flags map[string]string) {
	t.Helper()
	for name, value := range flags {
		f := c.Flags().Lookup(name)
		if f == nil {
			t.Fatalf("unknown flag %q on command %q", name, c.Name())
		}
		prev := f.Value.String()
		if err := c.Flags().Set(name, value); err != nil {
			t.Fatalf("setting flag %q=%q: %v", name, value, err)
		}
		t.Cleanup(func() {
			if err := c.Flags().Set(name, prev); err != nil {
				t.Fatalf("restoring flag %q: %v", name, err)
			}
		})
	}
}

func compareGolden(t *testing.T, path, got string) {
	t.Helper()

	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("creating golden dir: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("writing golden %s: %v", path, err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden %s: %v (run `go test ./cmd/ -run TestGolden -update` to create it)", path, err)
	}
	if got != string(want) {
		t.Fatalf("output for %s does not match golden (run `go test ./cmd/ -run TestGolden -update` to review and regenerate):\n--- got ---\n%s\n--- want ---\n%s", filepath.Base(path), got, string(want))
	}
}
