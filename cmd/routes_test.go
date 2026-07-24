package cmd

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRoutesCommandIgnoresInvalidRailsKitConfig(t *testing.T) {
	root := t.TempDir()
	mustWriteRoutesFile(t, filepath.Join(root, "config", "application.rb"))
	mustWriteRoutesFile(t, filepath.Join(root, "config", "routes.rb"))
	mustWriteRoutesFile(t, filepath.Join(root, ".rails-kit.yml"), ":\n")

	binDir := t.TempDir()
	bundlePath := filepath.Join(binDir, "bundle")
	script := "#!/bin/sh\nprintf 'Prefix Verb URI Pattern Controller#Action\\nusers GET /users users#index\\n'\n"
	if err := os.WriteFile(bundlePath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	prevPath := os.Getenv("PATH")
	prevWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	prevRootFlag := rootFlag

	if err := os.Setenv("PATH", binDir+string(os.PathListSeparator)+prevPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	rootFlag = ""
	routesCmd.SetContext(context.Background())
	t.Cleanup(func() {
		_ = os.Setenv("PATH", prevPath)
		_ = os.Chdir(prevWD)
		rootFlag = prevRootFlag
	})

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = oldStdout
	})

	err = routesCmd.RunE(routesCmd, nil)

	_ = w.Close()
	out, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatal(readErr)
	}

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(out), "users#index") {
		t.Fatalf("unexpected output: %s", out)
	}

	cacheFile := filepath.Join(root, "tmp", "routes_cache.txt")
	if _, err := os.Stat(cacheFile); err != nil {
		t.Fatalf("expected cache file to be written: %v", err)
	}
}

func TestRoutesFlagsMutuallyExclusive(t *testing.T) {
	prevRefresh := routesRefresh
	prevNoCache := routesNoCache
	t.Cleanup(func() {
		routesRefresh = prevRefresh
		routesNoCache = prevNoCache
	})

	routesRefresh = true
	routesNoCache = true

	err := routesCmd.RunE(routesCmd, nil)
	if err == nil {
		t.Fatal("expected error when both --refresh and --no-cache are set")
	}
	if err.Error() != "--refresh and --no-cache are mutually exclusive" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestRoutesCommandJSONOutputParsesBlankPrefixRows(t *testing.T) {
	root := t.TempDir()
	mustWriteRoutesFile(t, filepath.Join(root, "config", "application.rb"))
	mustWriteRoutesFile(t, filepath.Join(root, "config", "routes.rb"))

	binDir := t.TempDir()
	bundlePath := filepath.Join(binDir, "bundle")
	script := "#!/bin/sh\nprintf 'Prefix  Verb  URI Pattern  Controller#Action\\nroot  GET  /  home#index\\nPATCH/PUT  /users/:id  users#update\\nnew_user  GET  /users/new  users#new\\nGET  /users/:id  users#show\\n'\n"
	if err := os.WriteFile(bundlePath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	prevPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", binDir+string(os.PathListSeparator)+prevPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Setenv("PATH", prevPath)
	})

	out, errOut, err := runCmdForTestJSON(t, routesCmd, root, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, errOut)
	}

	var got []map[string]string
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal routes json: %v\noutput: %s", err, out)
	}
	if len(got) != 4 {
		t.Fatalf("expected 4 routes, got %d: %#v", len(got), got)
	}

	if got[1]["prefix"] != "" || got[1]["verb"] != "PATCH/PUT" || got[1]["uri_pattern"] != "/users/:id" || got[1]["controller_action"] != "users#update" {
		t.Fatalf("unexpected blank-prefix update route: %#v", got[1])
	}
	if got[3]["prefix"] != "" || got[3]["verb"] != "GET" || got[3]["controller_action"] != "users#show" {
		t.Fatalf("unexpected blank-prefix show route: %#v", got[3])
	}
}

func TestRoutesCommandJSONOutputFailsForNonTabularOutput(t *testing.T) {
	root := t.TempDir()
	mustWriteRoutesFile(t, filepath.Join(root, "config", "application.rb"))
	mustWriteRoutesFile(t, filepath.Join(root, "config", "routes.rb"))

	binDir := t.TempDir()
	bundlePath := filepath.Join(binDir, "bundle")
	script := "#!/bin/sh\nprintf 'Booting app...\\nroutes unavailable\\n'\n"
	if err := os.WriteFile(bundlePath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	prevPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", binDir+string(os.PathListSeparator)+prevPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Setenv("PATH", prevPath)
	})

	_, _, err := runCmdForTestJSON(t, routesCmd, root, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "standard tabular format") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoutesStaticCommand(t *testing.T) {
	root := t.TempDir()
	mustWriteRoutesFile(t, filepath.Join(root, "config", "application.rb"))
	mustWriteRoutesFile(t, filepath.Join(root, "config", "routes.rb"), `
Rails.application.routes.draw do
  root to: "posts#index"
  resources :posts, only: [:index, :show]
end
`)

	prevStatic := routesStatic
	t.Cleanup(func() { routesStatic = prevStatic })
	routesStatic = true

	out, errOut, err := runCmdForTestJSON(t, routesCmd, root, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, errOut)
	}

	var got []map[string]string
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal routes json: %v\noutput: %s", err, out)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 routes, got %d: %#v", len(got), got)
	}
	if got[0]["controller_action"] != "posts#index" || got[0]["prefix"] != "root" {
		t.Fatalf("unexpected root route: %#v", got[0])
	}

	// No cache file should be written for --static.
	if _, statErr := os.Stat(filepath.Join(root, "tmp", "routes_cache.txt")); statErr == nil {
		t.Fatal("expected no cache file to be written for --static")
	}
}

func TestRoutesStaticFiltersAndFormatsTable(t *testing.T) {
	root := t.TempDir()
	mustWriteRoutesFile(t, filepath.Join(root, "config", "application.rb"))
	mustWriteRoutesFile(t, filepath.Join(root, "config", "routes.rb"), `
Rails.application.routes.draw do
  resources :posts, only: [:index]
  resources :comments, only: [:index]
end
`)

	prevStatic := routesStatic
	t.Cleanup(func() { routesStatic = prevStatic })
	routesStatic = true

	out, errOut, err := runCmdForTestJSON(t, routesCmd, root, []string{"comments"})
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, errOut)
	}
	var got []map[string]string
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal routes json: %v\noutput: %s", err, out)
	}
	if len(got) != 1 || got[0]["controller_action"] != "comments#index" {
		t.Fatalf("unexpected filtered routes: %#v", got)
	}
}

func TestRoutesStaticWarningsStayOnStderr(t *testing.T) {
	root := t.TempDir()
	mustWriteRoutesFile(t, filepath.Join(root, "config", "application.rb"))
	mustWriteRoutesFile(t, filepath.Join(root, "config", "routes.rb"), `
Rails.application.routes.draw do
  resources :posts, only: :index
  mount Sidekiq::Web => "/sidekiq"
end
`)

	prevStatic := routesStatic
	t.Cleanup(func() { routesStatic = prevStatic })
	routesStatic = true

	out, errOut, err := runCmdForTestJSON(t, routesCmd, root, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, errOut)
	}
	var got []map[string]string
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("warning corrupted JSON stdout: %v\noutput: %s", err, out)
	}
	if len(got) != 1 || got[0]["controller_action"] != "posts#index" {
		t.Fatalf("unexpected routes: %#v", got)
	}
	if !strings.Contains(errOut, "Warning:") || !strings.Contains(errOut, ":4: unsupported route DSL") {
		t.Fatalf("expected line-specific warning on stderr, got %q", errOut)
	}
}

func TestRoutesStaticDrawWarningUsesDrawnFilePath(t *testing.T) {
	root := t.TempDir()
	mustWriteRoutesFile(t, filepath.Join(root, "config", "application.rb"))
	mustWriteRoutesFile(t, filepath.Join(root, "config", "routes.rb"), `
Rails.application.routes.draw do
  draw :extra
end
`)
	drawnPath := filepath.Join(root, "config", "routes", "extra.rb")
	mustWriteRoutesFile(t, drawnPath, `
mount Generic::Engine => "/engine"
`)

	prevStatic := routesStatic
	t.Cleanup(func() { routesStatic = prevStatic })
	routesStatic = true

	out, errOut, err := runCmdForTestJSON(t, routesCmd, root, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, errOut)
	}
	var got []map[string]string
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("warning corrupted JSON stdout: %v\noutput: %s", err, out)
	}
	if len(got) != 0 {
		t.Fatalf("unexpected routes: %#v", got)
	}
	sourceSuffix := filepath.Join("config", "routes", "extra.rb") + ":2: unsupported route DSL"
	if !strings.Contains(errOut, sourceSuffix) {
		t.Fatalf("expected drawn-file warning on stderr, got %q", errOut)
	}
}

func TestRoutesStaticConcernWarningsStayOnStderr(t *testing.T) {
	root := t.TempDir()
	mustWriteRoutesFile(t, filepath.Join(root, "config", "application.rb"))
	routesPath := filepath.Join(root, "config", "routes.rb")
	mustWriteRoutesFile(t, routesPath, `
Rails.application.routes.draw do
  concern :mountable do
    mount Generic::Engine => "/engine"
  end
  concerns :mountable
end
`)

	prevStatic := routesStatic
	t.Cleanup(func() { routesStatic = prevStatic })
	routesStatic = true

	out, errOut, err := runCmdForTestJSON(t, routesCmd, root, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, errOut)
	}
	var got []map[string]string
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("warning corrupted JSON stdout: %v\noutput: %s", err, out)
	}
	if len(got) != 0 {
		t.Fatalf("unexpected routes: %#v", got)
	}
	sourceSuffix := filepath.Join("config", "routes.rb") + ":4: unsupported route DSL"
	if !strings.Contains(errOut, sourceSuffix) {
		t.Fatalf("expected concern-body warning on stderr, got %q", errOut)
	}
}

func TestRoutesStaticCannotCombineWithCacheFlags(t *testing.T) {
	prevStatic := routesStatic
	prevRefresh := routesRefresh
	t.Cleanup(func() {
		routesStatic = prevStatic
		routesRefresh = prevRefresh
	})
	routesStatic = true
	routesRefresh = true

	err := routesCmd.RunE(routesCmd, nil)
	if err == nil {
		t.Fatal("expected error when combining --static and --refresh")
	}
	if !strings.Contains(err.Error(), "--static cannot be combined") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func mustWriteRoutesFile(t *testing.T, path string, content ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	body := ""
	if len(content) > 0 {
		body = content[0]
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
