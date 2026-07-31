package cmd

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// runCompletionForTest invokes c.ValidArgsFunction directly, chdir'd into
// root, with stdout/stderr captured so tests can assert a completion request
// never writes noise to the user's terminal.
func runCompletionForTest(t *testing.T, c *cobra.Command, root string, args []string, toComplete string) ([]string, cobra.ShellCompDirective, string, string) {
	t.Helper()

	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	prevRootFlag := rootFlag
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	rootFlag = ""
	t.Cleanup(func() {
		_ = os.Chdir(prevWD)
		rootFlag = prevRootFlag
	})

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = stdoutW
	os.Stderr = stderrW
	t.Cleanup(func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	})

	var stdoutBytes, stderrBytes []byte
	var stdoutErr, stderrErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		stdoutBytes, stdoutErr = io.ReadAll(stdoutR)
	}()
	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		stderrBytes, stderrErr = io.ReadAll(stderrR)
	}()

	if c.ValidArgsFunction == nil {
		t.Fatalf("%s has no ValidArgsFunction", c.Name())
	}
	candidates, directive := c.ValidArgsFunction(c, args, toComplete)

	_ = stdoutW.Close()
	_ = stderrW.Close()
	<-done
	<-stderrDone
	if stdoutErr != nil {
		t.Fatal(stdoutErr)
	}
	if stderrErr != nil {
		t.Fatal(stderrErr)
	}
	return candidates, directive, string(stdoutBytes), string(stderrBytes)
}

func mustEqualStrings(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func writeCompletionFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	mustWriteCmdFile(t, filepath.Join(root, "config", "application.rb"), "")

	mustWriteCmdFile(t, filepath.Join(root, "app", "models", "user.rb"), "class User\nend\n")
	mustWriteCmdFile(t, filepath.Join(root, "app", "models", "post.rb"), "class Post\nend\n")
	mustWriteCmdFile(t, filepath.Join(root, "app", "models", "admin", "dashboard.rb"), "class Admin::Dashboard\nend\n")

	mustWriteCmdFile(t, filepath.Join(root, "db", "schema.rb"), strings.Join([]string{
		`ActiveRecord::Schema[7.2].define(version: 2024_01_01_000001) do`,
		`  create_table "users", force: :cascade do |t|`,
		`  end`,
		`  create_table "posts", force: :cascade do |t|`,
		`  end`,
		`end`,
		"",
	}, "\n"))

	mustWriteCmdFile(t, filepath.Join(root, "config", "locales", "en.yml"), strings.Join([]string{
		`en:`,
		`  views:`,
		`    users: "Users"`,
		`    posts:`,
		`      title: "Posts"`,
		`cs:`,
		`  views:`,
		`    posts:`,
		`      title: "Prispevky"`,
		"",
	}, "\n"))

	mustWriteCmdFile(t, filepath.Join(root, "app", "models", "concerns", "searchable.rb"), "module Searchable\nend\n")
	mustWriteCmdFile(t, filepath.Join(root, "app", "controllers", "concerns", "authenticatable.rb"), "module Authenticatable\nend\n")

	mustWriteCmdFile(t, filepath.Join(root, "test", "fixtures", "users.yml"), "alice:\n  name: Alice\n")
	mustWriteCmdFile(t, filepath.Join(root, "test", "fixtures", "posts.yml"), "one:\n  title: Hello\n")

	mustWriteCmdFile(t, filepath.Join(root, "Gemfile.lock"), strings.Join([]string{
		`GEM`,
		`  remote: https://rubygems.org/`,
		`  specs:`,
		`    rails (7.1.3)`,
		`    rake (13.0.6)`,
		``,
		`PLATFORMS`,
		`  ruby`,
		``,
		`DEPENDENCIES`,
		`  rails`,
		`  rake`,
		"",
	}, "\n"))

	return root
}

func TestCompleteModelNames(t *testing.T) {
	root := writeCompletionFixture(t)

	candidates, directive, out, errOut := runCompletionForTest(t, modelCmd, root, nil, "")
	if out != "" || errOut != "" {
		t.Fatalf("expected no stdout/stderr, got stdout=%q stderr=%q", out, errOut)
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("directive = %v, want NoFileComp", directive)
	}
	mustEqualStrings(t, candidates, []string{"admin/dashboard", "concerns/searchable", "post", "user"})
}

func TestCompleteRelatedNames(t *testing.T) {
	root := writeCompletionFixture(t)

	candidates, directive, _, _ := runCompletionForTest(t, relatedCmd, root, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("directive = %v, want NoFileComp", directive)
	}
	mustEqualStrings(t, candidates, []string{"admin/dashboard", "concerns/searchable", "post", "user"})
}

func TestCompleteSkeletonArgs(t *testing.T) {
	root := writeCompletionFixture(t)

	candidates, directive, _, _ := runCompletionForTest(t, skeletonCmd, root, nil, "")
	if directive != cobra.ShellCompDirectiveDefault {
		t.Fatalf("directive = %v, want Default", directive)
	}
	mustEqualStrings(t, candidates, []string{"admin/dashboard", "concerns/searchable", "post", "user"})

	// Already-typed args are filtered out.
	candidates, _, _, _ = runCompletionForTest(t, skeletonCmd, root, []string{"post"}, "")
	mustEqualStrings(t, candidates, []string{"admin/dashboard", "concerns/searchable", "user"})
}

func TestCompleteSchemaTables(t *testing.T) {
	root := writeCompletionFixture(t)

	candidates, directive, _, _ := runCompletionForTest(t, schemaCmd, root, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("directive = %v, want NoFileComp", directive)
	}
	mustEqualStrings(t, candidates, []string{"posts", "users"})

	candidates, _, _, _ = runCompletionForTest(t, schemaCmd, root, []string{"posts"}, "")
	mustEqualStrings(t, candidates, []string{"users"})
}

func TestCompleteConcernNames(t *testing.T) {
	root := writeCompletionFixture(t)

	candidates, directive, _, _ := runCompletionForTest(t, concernsCmd, root, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("directive = %v, want NoFileComp", directive)
	}
	mustEqualStrings(t, candidates, []string{"authenticatable", "searchable"})
}

func TestCompleteFixtureNames(t *testing.T) {
	root := writeCompletionFixture(t)

	candidates, directive, _, _ := runCompletionForTest(t, fixturesCmd, root, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("directive = %v, want NoFileComp", directive)
	}
	mustEqualStrings(t, candidates, []string{"posts", "users"})
}

func TestCompleteGemNames(t *testing.T) {
	root := writeCompletionFixture(t)

	candidates, directive, _, _ := runCompletionForTest(t, gemCmd, root, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("directive = %v, want NoFileComp", directive)
	}
	mustEqualStrings(t, candidates, []string{"rails", "rake"})
}

func TestCompleteLocalesScope(t *testing.T) {
	root := writeCompletionFixture(t)

	// Empty toComplete lists top-level locale codes.
	candidates, directive, _, _ := runCompletionForTest(t, localesCmd, root, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("directive = %v, want NoFileComp", directive)
	}
	mustEqualStrings(t, candidates, []string{"cs", "en"})

	// A settled prefix ending in '.' lists only its immediate children.
	candidates, _, _, _ = runCompletionForTest(t, localesCmd, root, nil, "en.")
	mustEqualStrings(t, candidates, []string{"en.views"})

	candidates, _, _, _ = runCompletionForTest(t, localesCmd, root, nil, "en.views.")
	mustEqualStrings(t, candidates, []string{"en.views.posts", "en.views.users"})

	// A leaf scope (en.views.users is a plain string) has nothing further
	// to drill into.
	candidates, _, _, _ = runCompletionForTest(t, localesCmd, root, nil, "en.views.users.title")
	if len(candidates) != 0 {
		t.Fatalf("expected no candidates for a leaf scope, got %v", candidates)
	}
}

func TestCompletionOutsideRailsRootYieldsNoCandidatesAndNoOutput(t *testing.T) {
	root := t.TempDir() // no config/application.rb

	cmds := []*cobra.Command{modelCmd, relatedCmd, schemaCmd, localesCmd, concernsCmd, fixturesCmd, gemCmd}
	for _, c := range cmds {
		candidates, directive, out, errOut := runCompletionForTest(t, c, root, nil, "")
		if len(candidates) != 0 {
			t.Errorf("%s: expected no candidates outside a Rails root, got %v", c.Name(), candidates)
		}
		if directive != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("%s: directive = %v, want NoFileComp", c.Name(), directive)
		}
		if out != "" || errOut != "" {
			t.Errorf("%s: expected no stdout/stderr outside a Rails root, got stdout=%q stderr=%q", c.Name(), out, errOut)
		}
	}

	candidates, directive, out, errOut := runCompletionForTest(t, skeletonCmd, root, nil, "")
	if len(candidates) != 0 {
		t.Errorf("skeleton: expected no candidates outside a Rails root, got %v", candidates)
	}
	if directive != cobra.ShellCompDirectiveDefault {
		t.Errorf("skeleton: directive = %v, want Default", directive)
	}
	if out != "" || errOut != "" {
		t.Errorf("skeleton: expected no stdout/stderr outside a Rails root, got stdout=%q stderr=%q", out, errOut)
	}
}
