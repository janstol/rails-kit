package cmd

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/janstol/rails-kit/internal/testutil"
)

func TestRoutesCommandIgnoresInvalidRailsKitConfig(t *testing.T) {
	root := t.TempDir()
	mustWriteRoutesFile(t, filepath.Join(root, "config", "application.rb"))
	mustWriteRoutesFile(t, filepath.Join(root, "config", "routes.rb"))
	mustWriteRoutesFile(t, filepath.Join(root, ".rails-kit.yml"), ":\n")

	binDir := t.TempDir()
	testutil.WriteFakeBundle(t, binDir, "Prefix Verb URI Pattern Controller#Action\nusers GET /users users#index\n")

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
	testutil.WriteFakeBundle(t, binDir, "Prefix  Verb  URI Pattern  Controller#Action\nroot  GET  /  home#index\nPATCH/PUT  /users/:id  users#update\nnew_user  GET  /users/new  users#new\nGET  /users/:id  users#show\n")
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
	testutil.WriteFakeBundle(t, binDir, "Booting app...\nroutes unavailable\n")
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

func TestRoutesStaticLiteralRedirectJSON(t *testing.T) {
	root := t.TempDir()
	mustWriteRoutesFile(t, filepath.Join(root, "config", "application.rb"))
	mustWriteRoutesFile(t, filepath.Join(root, "config", "routes.rb"), `
Rails.application.routes.draw do
  get "old", to: redirect("/new", status: 307), as: :legacy
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
	if len(got) != 1 ||
		got[0]["prefix"] != "legacy" ||
		got[0]["uri_pattern"] != "/old" ||
		got[0]["controller_action"] != "redirect(307, /new)" {
		t.Fatalf("unexpected redirect route: %#v", got)
	}
	if errOut != "" {
		t.Fatalf("unexpected redirect warning: %q", errOut)
	}
}

func TestRoutesStaticMatchJSON(t *testing.T) {
	root := t.TempDir()
	mustWriteRoutesFile(t, filepath.Join(root, "config", "application.rb"))
	mustWriteRoutesFile(t, filepath.Join(root, "config", "routes.rb"), `
Rails.application.routes.draw do
  match "events", to: "events#create", via: [:post, :options], as: :events
  match "fallback", to: "fallback#show", via: :all
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
	if len(got) != 2 ||
		got[0]["prefix"] != "events" ||
		got[0]["verb"] != "POST|OPTIONS" ||
		got[0]["controller_action"] != "events#create" ||
		got[1]["verb"] != "" ||
		got[1]["controller_action"] != "fallback#show" {
		t.Fatalf("unexpected match routes: %#v", got)
	}
	if errOut != "" {
		t.Fatalf("unexpected match warning: %q", errOut)
	}
}

func TestRoutesStaticPathConstraintJSON(t *testing.T) {
	root := t.TempDir()
	mustWriteRoutesFile(t, filepath.Join(root, "config", "application.rb"))
	mustWriteRoutesFile(t, filepath.Join(root, "config", "routes.rb"), `
Rails.application.routes.draw do
  get "items/:id", to: "items#show", constraints: { id: /[0-9]+/ }
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
	if len(got) != 1 ||
		got[0]["uri_pattern"] != "/items/:id" ||
		got[0]["controller_action"] != `items#show {id: /[0-9]+/}` {
		t.Fatalf("unexpected constrained route: %#v", got)
	}
	if errOut != "" {
		t.Fatalf("unexpected constraint warning: %q", errOut)
	}
}

func TestRoutesStaticMountJSON(t *testing.T) {
	root := t.TempDir()
	mustWriteRoutesFile(t, filepath.Join(root, "config", "application.rb"))
	mustWriteRoutesFile(t, filepath.Join(root, "config", "routes.rb"), `
Rails.application.routes.draw do
  mount Generic::Engine, at: "/engine"
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
	if len(got) != 1 ||
		got[0]["prefix"] != "generic_engine" ||
		got[0]["verb"] != "" ||
		got[0]["uri_pattern"] != "/engine" ||
		got[0]["controller_action"] != "Generic::Engine" {
		t.Fatalf("unexpected mount route: %#v", got)
	}
	if errOut != "" {
		t.Fatalf("unexpected mount warning: %q", errOut)
	}
}

func TestRoutesStaticConditionalMountReceiverJSON(t *testing.T) {
	root := t.TempDir()
	mustWriteRoutesFile(t, filepath.Join(root, "config", "application.rb"))
	mustWriteRoutesFile(t, filepath.Join(root, "config", "routes.rb"), `
Rails.application.routes.draw do
  mount GenericServer.server, at: "/socket" if socket_enabled?
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
	if len(got) != 1 ||
		got[0]["prefix"] != "" ||
		got[0]["uri_pattern"] != "/socket" ||
		got[0]["controller_action"] != "GenericServer.server" {
		t.Fatalf("unexpected conditional receiver mount: %#v", got)
	}
	if !strings.Contains(errOut, "conditional mount is included without evaluating its condition") {
		t.Fatalf("expected conditional mount warning, got %q", errOut)
	}
}

func TestRoutesStaticScopeJSON(t *testing.T) {
	root := t.TempDir()
	mustWriteRoutesFile(t, filepath.Join(root, "config", "application.rb"))
	mustWriteRoutesFile(t, filepath.Join(root, "config", "routes.rb"), `
Rails.application.routes.draw do
  scope path: :api, module: :v1, as: :api do
    get "items", to: "items#index"
  end
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
	if len(got) != 1 ||
		got[0]["prefix"] != "api_index" ||
		got[0]["uri_pattern"] != "/api/items" ||
		got[0]["controller_action"] != "v1/items#index" {
		t.Fatalf("unexpected scoped route: %#v", got)
	}
	if errOut != "" {
		t.Fatalf("unexpected scope warning: %q", errOut)
	}
}

func TestRoutesStaticControllerBlockJSON(t *testing.T) {
	root := t.TempDir()
	mustWriteRoutesFile(t, filepath.Join(root, "config", "application.rb"))
	mustWriteRoutesFile(t, filepath.Join(root, "config", "routes.rb"), `
Rails.application.routes.draw do
  controller :items do
    get "show"
  end
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
	if len(got) != 1 ||
		got[0]["prefix"] != "show" ||
		got[0]["uri_pattern"] != "/show" ||
		got[0]["controller_action"] != "items#show" {
		t.Fatalf("unexpected controller-block route: %#v", got)
	}
	if errOut != "" {
		t.Fatalf("unexpected controller-block warning: %q", errOut)
	}
}

func TestRoutesStaticMultilineVerbJSON(t *testing.T) {
	root := t.TempDir()
	mustWriteRoutesFile(t, filepath.Join(root, "config", "application.rb"))
	mustWriteRoutesFile(t, filepath.Join(root, "config", "routes.rb"), `
Rails.application.routes.draw do
  get "/items/:id",
      to: "items#show",
      as: :item_preview
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
	if len(got) != 1 ||
		got[0]["prefix"] != "item_preview" ||
		got[0]["uri_pattern"] != "/items/:id" ||
		got[0]["controller_action"] != "items#show" {
		t.Fatalf("unexpected multiline route: %#v", got)
	}
	if errOut != "" {
		t.Fatalf("unexpected multiline route warning: %q", errOut)
	}
}

func TestRoutesStaticInlineNamespaceJSON(t *testing.T) {
	root := t.TempDir()
	mustWriteRoutesFile(t, filepath.Join(root, "config", "application.rb"))
	mustWriteRoutesFile(t, filepath.Join(root, "config", "routes.rb"), `
Rails.application.routes.draw do
  namespace(:admin) { get "status", to: "status#show" }
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
	if len(got) != 1 ||
		got[0]["prefix"] != "admin_show" ||
		got[0]["uri_pattern"] != "/admin/status" ||
		got[0]["controller_action"] != "admin/status#show" {
		t.Fatalf("unexpected inline namespace route: %#v", got)
	}
	if errOut != "" {
		t.Fatalf("unexpected inline namespace warning: %q", errOut)
	}
}

func TestRoutesStaticResourcePrefixesMatchRailsJSON(t *testing.T) {
	root := t.TempDir()
	mustWriteRoutesFile(t, filepath.Join(root, "config", "application.rb"))
	mustWriteRoutesFile(t, filepath.Join(root, "config", "routes.rb"), `
Rails.application.routes.draw do
  resources :users
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
	prefixes := make(map[string]string)
	for _, entry := range got {
		key := entry["verb"] + " " + entry["controller_action"]
		prefixes[key] = entry["prefix"]
	}
	want := map[string]string{
		"GET users#index":      "users",
		"POST users#create":    "",
		"GET users#show":       "user",
		"PATCH users#update":   "",
		"PUT users#update":     "",
		"DELETE users#destroy": "",
	}
	for key, prefix := range want {
		if prefixes[key] != prefix {
			t.Errorf("%s prefix = %q, want %q", key, prefixes[key], prefix)
		}
	}
	if errOut != "" {
		t.Fatalf("unexpected resource prefix warning: %q", errOut)
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
  mount app_for(:generic), at: "/engine"
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
	if !strings.Contains(errOut, "Warning:") || !strings.Contains(errOut, ":4: dynamic mount target") {
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
mount app_for(:generic), at: "/engine"
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
	sourceSuffix := filepath.Join("config", "routes", "extra.rb") + ":2: dynamic mount target"
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
    mount app_for(:generic), at: "/engine"
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
	sourceSuffix := filepath.Join("config", "routes.rb") + ":4: dynamic mount target"
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

func TestRoutesWatchIntervalFloor(t *testing.T) {
	prevWatch := routesWatch
	prevInterval := routesWatchInterval
	t.Cleanup(func() {
		routesWatch = prevWatch
		routesWatchInterval = prevInterval
	})
	routesWatch = true
	routesWatchInterval = 50 * time.Millisecond

	err := routesCmd.RunE(routesCmd, nil)
	if err == nil {
		t.Fatal("expected error for --watch-interval below 100ms")
	}
	if !strings.Contains(err.Error(), "--watch-interval") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoutesWatchNonTTYRendersOnceThenExits(t *testing.T) {
	root := t.TempDir()
	mustWriteRoutesFile(t, filepath.Join(root, "config", "application.rb"))
	mustWriteRoutesFile(t, filepath.Join(root, "config", "routes.rb"), `
Rails.application.routes.draw do
  resources :posts, only: [:index]
end
`)

	prevStatic := routesStatic
	prevWatch := routesWatch
	prevInterval := routesWatchInterval
	t.Cleanup(func() {
		routesStatic = prevStatic
		routesWatch = prevWatch
		routesWatchInterval = prevInterval
	})
	routesStatic = true
	routesWatch = true
	routesWatchInterval = 100 * time.Millisecond

	prevCtx := routesCmd.Context()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	routesCmd.SetContext(ctx)
	t.Cleanup(func() { routesCmd.SetContext(prevCtx) })

	out, errOut, err := runCmdForTest(t, routesCmd, root, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v\nstderr: %s", err, errOut)
	}
	if !strings.Contains(out, "posts#index") {
		t.Fatalf("expected routes output, got: %q", out)
	}
	if strings.Contains(out, "\x1b[") {
		t.Fatalf("expected no ANSI escapes in non-TTY stdout, got: %q", out)
	}
	if strings.Contains(errOut, "\x1b[") {
		t.Fatalf("expected no ANSI escapes in non-TTY stderr, got: %q", errOut)
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
