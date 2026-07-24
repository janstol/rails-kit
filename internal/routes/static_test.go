package routes_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/janstol/rails-kit/internal/pluralize"
	"github.com/janstol/rails-kit/internal/routes"
)

func writeRoutesFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "routes.rb")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func findRoute(entries []routes.RouteEntry, verb, action string) *routes.RouteEntry {
	for i := range entries {
		parts := splitCA(entries[i].ControllerAction)
		if entries[i].Verb == verb && parts[1] == action {
			return &entries[i]
		}
	}
	return nil
}

func splitCA(ca string) [2]string {
	for i, c := range ca {
		if c == '#' {
			return [2]string{ca[:i], ca[i+1:]}
		}
	}
	return [2]string{ca, ""}
}

func TestParseStatic_Resources(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  resources :users
end
`)
	entries, err := routes.ParseStatic(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}

	wantActions := []struct {
		verb   string
		action string
		path   string
	}{
		{"GET", "index", "/users"},
		{"POST", "create", "/users"},
		{"GET", "new", "/users/new"},
		{"GET", "edit", "/users/:id/edit"},
		{"GET", "show", "/users/:id"},
		{"PATCH", "update", "/users/:id"},
		{"PUT", "update", "/users/:id"},
		{"DELETE", "destroy", "/users/:id"},
	}

	if len(entries) != len(wantActions) {
		t.Errorf("got %d entries, want %d; entries: %v", len(entries), len(wantActions), entries)
	}

	for _, w := range wantActions {
		e := findRoute(entries, w.verb, w.action)
		if e == nil {
			t.Errorf("missing route %s %s", w.verb, w.action)
			continue
		}
		if e.URIPattern != w.path {
			t.Errorf("%s %s: path = %q, want %q", w.verb, w.action, e.URIPattern, w.path)
		}
		if splitCA(e.ControllerAction)[0] != "users" {
			t.Errorf("%s %s: controller = %q, want %q", w.verb, w.action, splitCA(e.ControllerAction)[0], "users")
		}
	}
}

func TestParseStatic_Only(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  resources :users, only: [:index, :show, :create]
end
`)
	entries, err := routes.ParseStatic(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]bool{"index": true, "show": true, "create": true}
	for _, e := range entries {
		action := splitCA(e.ControllerAction)[1]
		if !want[action] {
			t.Errorf("unexpected action %q in entries", action)
		}
	}
	if len(entries) != 3 {
		t.Errorf("got %d entries, want 3", len(entries))
	}
}

func TestParseStatic_Except(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  resources :posts, except: [:destroy]
end
`)
	entries, err := routes.ParseStatic(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}

	for _, e := range entries {
		if splitCA(e.ControllerAction)[1] == "destroy" {
			t.Error("destroy should be excluded")
		}
	}
}

func TestParseStatic_Namespace(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  namespace :admin do
    resources :dashboards, only: [:index, :show]
  end
end
`)
	entries, err := routes.ParseStatic(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) == 0 {
		t.Fatal("no entries parsed")
	}
	for _, e := range entries {
		if splitCA(e.ControllerAction)[0] != "admin/dashboards" {
			t.Errorf("controller = %q, want %q", splitCA(e.ControllerAction)[0], "admin/dashboards")
		}
		if !hasPrefix(e.URIPattern, "/admin/dashboards") {
			t.Errorf("path = %q, want prefix /admin/dashboards", e.URIPattern)
		}
	}
}

func TestParseStatic_NestedResources(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  resources :posts do
    resources :comments, only: [:index, :create]
  end
end
`)
	entries, err := routes.ParseStatic(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}

	// Should have post routes + comment routes
	var commentEntries []routes.RouteEntry
	for _, e := range entries {
		if splitCA(e.ControllerAction)[0] == "comments" {
			commentEntries = append(commentEntries, e)
		}
	}

	if len(commentEntries) == 0 {
		t.Fatal("no comment routes found")
	}
	for _, e := range commentEntries {
		if !hasPrefix(e.URIPattern, "/posts/") {
			t.Errorf("nested comment path = %q, want prefix /posts/", e.URIPattern)
		}
	}
}

func TestParseStatic_VerbRoutes(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  get '/about', to: 'pages#about'
  post '/login', to: 'sessions#create'
end
`)
	entries, err := routes.ParseStatic(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}

	got := map[string]string{}
	for _, e := range entries {
		got[e.Verb] = e.ControllerAction
	}
	if got["GET"] != "pages#about" {
		t.Errorf("GET route: got %q, want %q", got["GET"], "pages#about")
	}
	if got["POST"] != "sessions#create" {
		t.Errorf("POST route: got %q, want %q", got["POST"], "sessions#create")
	}
}

func TestParseStatic_Root(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  root 'home#index'
end
`)
	entries, err := routes.ParseStatic(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	e := entries[0]
	if e.Verb != "GET" || e.ControllerAction != "home#index" || e.Prefix != "root" {
		t.Errorf("root route = %+v", e)
	}
}

func TestParseStatic_Testdata(t *testing.T) {
	// Test against the actual testdata/config/routes.rb fixture.
	// Derive path relative to this test file.
	path := filepath.Join("..", "..", "testdata", "config", "routes.rb")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("testdata/config/routes.rb not found")
	}

	entries, err := routes.ParseStatic(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) == 0 {
		t.Error("no routes parsed from testdata")
	}

	controllers := map[string]bool{}
	for _, e := range entries {
		controllers[splitCA(e.ControllerAction)[0]] = true
	}

	wantControllers := []string{"home", "users", "posts", "comments", "admin/dashboards", "pages", "sessions"}
	for _, c := range wantControllers {
		if !controllers[c] {
			t.Errorf("expected controller %q in routes", c)
		}
	}
}

func TestParseStatic_MemberAndCollection(t *testing.T) {
	// Verifies that member/collection blocks don't corrupt the depth counter,
	// so standard resource routes still parse correctly after these blocks.
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  resources :posts do
    member do
      get '/publish', to: 'posts#publish'
    end
    collection do
      get '/drafts', to: 'posts#drafts'
    end
  end
end
`)
	entries, err := routes.ParseStatic(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}

	// All 8 standard post route entries (PATCH and PUT both map to update) must be present.
	for _, w := range []struct{ verb, action string }{
		{"GET", "index"}, {"POST", "create"}, {"GET", "new"},
		{"GET", "show"}, {"GET", "edit"}, {"PATCH", "update"},
		{"PUT", "update"}, {"DELETE", "destroy"},
	} {
		if findRoute(entries, w.verb, w.action) == nil {
			t.Errorf("missing standard route %s %s", w.verb, w.action)
		}
	}

	// Verb routes inside member/collection blocks should also be captured.
	if findRoute(entries, "GET", "publish") == nil {
		t.Error("missing verb route GET publish from member block")
	} else if got := findRoute(entries, "GET", "publish").URIPattern; got != "/posts/:id/publish" {
		t.Errorf("member route path = %q, want /posts/:id/publish", got)
	}
	if findRoute(entries, "GET", "drafts") == nil {
		t.Error("missing verb route GET drafts from collection block")
	} else if got := findRoute(entries, "GET", "drafts").URIPattern; got != "/posts/drafts" {
		t.Errorf("collection route path = %q, want /posts/drafts", got)
	}
}

func TestParseStatic_NestedMemberAndCollectionPaths(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  namespace :admin do
    resources :accounts do
      resources :posts do
        member do
          get 'publish', to: 'posts#publish'
        end
        collection do
          get 'drafts', to: 'posts#drafts'
        end
      end
    end
  end
end
`)
	entries, err := routes.ParseStatic(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}

	member := findRoute(entries, "GET", "publish")
	if member == nil || member.URIPattern != "/admin/accounts/:account_id/posts/:id/publish" {
		t.Fatalf("unexpected nested member route: %#v", member)
	}
	collection := findRoute(entries, "GET", "drafts")
	if collection == nil || collection.URIPattern != "/admin/accounts/:account_id/posts/drafts" {
		t.Fatalf("unexpected nested collection route: %#v", collection)
	}
	if splitCA(member.ControllerAction)[0] != "admin/posts" {
		t.Errorf("member controller = %q, want admin/posts", member.ControllerAction)
	}
}

func TestParseStatic_ScalarOnlyAndExcept(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  resources :users, only: :index
  resource :profile, except: 'destroy'
end
`)
	entries, err := routes.ParseStatic(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}

	var userEntries, profileEntries []routes.RouteEntry
	for _, entry := range entries {
		switch splitCA(entry.ControllerAction)[0] {
		case "users":
			userEntries = append(userEntries, entry)
		case "profile":
			profileEntries = append(profileEntries, entry)
		}
	}
	if len(userEntries) != 1 || splitCA(userEntries[0].ControllerAction)[1] != "index" {
		t.Fatalf("scalar only produced unexpected routes: %#v", userEntries)
	}
	if len(profileEntries) != 6 {
		t.Fatalf("scalar except produced %d routes, want 6: %#v", len(profileEntries), profileEntries)
	}
	for _, entry := range profileEntries {
		if splitCA(entry.ControllerAction)[1] == "destroy" {
			t.Fatalf("scalar except retained destroy route: %#v", entry)
		}
	}
}

func TestParseStaticDetailed_WarnsForUnsupportedSyntax(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  resources :users, only: :index, path: "people"
  scope "/admin" do
    get :health, to: "health#show"
  end
end
`)
	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("got %d supported entries, want 2: %#v", len(result.Entries), result.Entries)
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("got %d warnings, want 1: %#v", len(result.Warnings), result.Warnings)
	}
	if result.Warnings[0].Line != 4 || !strings.Contains(result.Warnings[0].Message, "unsupported route DSL") {
		t.Errorf("unexpected warning: %#v", result.Warnings[0])
	}
}

func TestParseStatic_SingularResource(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  resource :profile
end
`)
	entries, err := routes.ParseStatic(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}

	// singular resource has 6 actions (no index), PATCH and PUT both map to update
	if len(entries) != 7 {
		t.Errorf("got %d entries, want 7; entries: %v", len(entries), entries)
	}

	for _, e := range entries {
		action := splitCA(e.ControllerAction)[1]
		if action == "index" {
			t.Error("singular resource should not have index action")
		}
	}

	// Paths should use /profile (no :id param)
	for _, e := range entries {
		if e.URIPattern != "/profile" && e.URIPattern != "/profile/new" && e.URIPattern != "/profile/edit" {
			t.Errorf("unexpected path for singular resource: %q", e.URIPattern)
		}
	}
}

func TestParseStatic_IrregularPlural(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  resources :addresses do
    resources :categories, only: [:index]
  end
end
`)
	entries, err := routes.ParseStatic(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}

	// Standard member routes use Rails' default :id parameter.
	for _, e := range entries {
		if splitCA(e.ControllerAction)[0] == "addresses" && e.URIPattern != "/addresses" && e.URIPattern != "/addresses/new" {
			if !hasPrefix(e.URIPattern, "/addresses/:id") {
				t.Errorf("expected :id param, got path %q", e.URIPattern)
			}
		}
	}

	// Nested categories path should be under /addresses/:address_id/categories
	for _, e := range entries {
		if splitCA(e.ControllerAction)[0] == "categories" {
			if !hasPrefix(e.URIPattern, "/addresses/:address_id/categories") {
				t.Errorf("nested categories path = %q, want prefix /addresses/:address_id/categories", e.URIPattern)
			}
		}
	}
}

func hasPrefix(s, prefix string) bool {
	return strings.HasPrefix(s, prefix)
}

func TestParseStatic_VerbRoutePathSeparator(t *testing.T) {
	// Verb routes whose path lacks a leading slash should not be concatenated
	// directly against the namespace prefix (e.g. "/admin" + "dashboard" must
	// produce "/admin/dashboard", not "/admindashboard").
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  namespace :admin do
    get 'dashboard', to: 'dashboard#index'
  end
  get 'health', to: 'health#show'
end
`)
	entries, err := routes.ParseStatic(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}

	findByAction := func(action string) *routes.RouteEntry {
		for i := range entries {
			if splitCA(entries[i].ControllerAction)[1] == action {
				return &entries[i]
			}
		}
		return nil
	}

	if e := findByAction("index"); e != nil {
		if e.URIPattern != "/admin/dashboard" {
			t.Errorf("namespaced verb route path = %q, want /admin/dashboard", e.URIPattern)
		}
	} else {
		t.Error("missing namespaced verb route dashboard#index")
	}

	if e := findByAction("show"); e != nil {
		if e.URIPattern != "/health" {
			t.Errorf("root verb route path = %q, want /health", e.URIPattern)
		}
	} else {
		t.Error("missing root verb route health#show")
	}
}

func TestParseStatic_EndWithComment(t *testing.T) {
	// "end # comment" must still decrement depth; routes after it must not be
	// incorrectly nested inside the preceding block.
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  namespace :admin do
    resources :users
  end # close admin namespace
  resources :posts
end
`)
	entries, err := routes.ParseStatic(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}

	for _, e := range entries {
		ctrl := splitCA(e.ControllerAction)[0]
		if ctrl == "posts" && !hasPrefix(e.URIPattern, "/posts") {
			t.Errorf("posts route should not be nested under admin, got path %q", e.URIPattern)
		}
		if ctrl == "users" && !hasPrefix(e.URIPattern, "/admin/users") {
			t.Errorf("users route should be under /admin/users, got path %q", e.URIPattern)
		}
	}
}

func TestParseStatic_ResourceActionFilterForms(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  resources :users, only: %i[index show]
  resources :posts, only: []
  resources :comments, except: %w[destroy update]
  resources :tags, :only => :index
end
`)
	entries, err := routes.ParseStatic(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}

	counts := map[string]int{}
	actions := map[string]map[string]bool{}
	for _, entry := range entries {
		controller, action := splitCA(entry.ControllerAction)[0], splitCA(entry.ControllerAction)[1]
		counts[controller]++
		if actions[controller] == nil {
			actions[controller] = map[string]bool{}
		}
		actions[controller][action] = true
	}
	if counts["users"] != 2 || !actions["users"]["index"] || !actions["users"]["show"] {
		t.Errorf("unexpected user routes: %#v", actions["users"])
	}
	if counts["posts"] != 0 {
		t.Errorf("only: [] produced post routes: %#v", actions["posts"])
	}
	if counts["comments"] != 5 || actions["comments"]["destroy"] || actions["comments"]["update"] {
		t.Errorf("unexpected comment routes: %#v", actions["comments"])
	}
	if counts["tags"] != 1 || !actions["tags"]["index"] {
		t.Errorf("unexpected tag routes: %#v", actions["tags"])
	}
}

func TestParseStatic_ResourceOptionsAndNestedParameters(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  namespace :admin do
    resources :users, only: [:show], path: "people", controller: "accounts", as: :members, param: :uuid do
      resources :posts, only: :index
    end
  end
end
`)
	entries, err := routes.ParseStatic(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d routes, want 2: %#v", len(entries), entries)
	}
	var parent, child *routes.RouteEntry
	for i := range entries {
		switch splitCA(entries[i].ControllerAction)[0] {
		case "admin/accounts":
			parent = &entries[i]
		case "admin/posts":
			child = &entries[i]
		}
	}
	if parent == nil || parent.URIPattern != "/admin/people/:uuid" || parent.Prefix != "admin_member" {
		t.Errorf("unexpected parent route: %#v", parent)
	}
	if child == nil || child.URIPattern != "/admin/people/:user_uuid/posts" || child.Prefix != "admin_member_posts" {
		t.Errorf("unexpected child route: %#v", child)
	}
}

func TestParseStatic_SymbolicResourceRoutes(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  resources :users, only: [] do
    get :search, on: :collection
    patch :archive, on: :member
    get :preview
    member do
      post :publish
    end
    collection do
      get :lookup, controller: "directory", action: :find, as: :find_users
    end
  end
end
`)
	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", result.Warnings)
	}
	want := map[string]struct {
		verb, path, controller, helper string
	}{
		"search":  {"GET", "/users/search", "users", "search_users"},
		"archive": {"PATCH", "/users/:id/archive", "users", "archive_user"},
		"preview": {"GET", "/users/:user_id/preview", "users", "user_preview"},
		"publish": {"POST", "/users/:id/publish", "users", "publish_user"},
		"find":    {"GET", "/users/lookup", "directory", "user_find_users"},
	}
	if len(result.Entries) != len(want) {
		t.Fatalf("got %d routes, want %d: %#v", len(result.Entries), len(want), result.Entries)
	}
	for _, entry := range result.Entries {
		action := splitCA(entry.ControllerAction)[1]
		expected, ok := want[action]
		if !ok {
			t.Errorf("unexpected route: %#v", entry)
			continue
		}
		if entry.Verb != expected.verb || entry.URIPattern != expected.path ||
			splitCA(entry.ControllerAction)[0] != expected.controller || entry.Prefix != expected.helper {
			t.Errorf("%s route = %#v, want %#v", action, entry, expected)
		}
	}
}

func TestParseStatic_HashRocketAndInferredRoutes(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  get "users/profile"
  get "health", to: "health#show", as: :health
  patch "users/:id" => "users#update", :as => "update_user", constraints: { id: /\d+/ }
  get "legacy" => redirect("/new")
end
`)
	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 3 {
		t.Fatalf("got %d routes, want 3: %#v", len(result.Entries), result.Entries)
	}
	want := map[string]routes.RouteEntry{
		"profile": {Prefix: "profile", Verb: "GET", URIPattern: "/users/profile", ControllerAction: "users#profile"},
		"show":    {Prefix: "health", Verb: "GET", URIPattern: "/health", ControllerAction: "health#show"},
		"update":  {Prefix: "update_user", Verb: "PATCH", URIPattern: "/users/:id", ControllerAction: "users#update"},
	}
	for _, entry := range result.Entries {
		action := splitCA(entry.ControllerAction)[1]
		if entry != want[action] {
			t.Errorf("%s route = %#v, want %#v", action, entry, want[action])
		}
	}
	if len(result.Warnings) != 2 ||
		!strings.Contains(result.Warnings[0].Message, "constraints") ||
		!strings.Contains(result.Warnings[1].Message, "dynamic") {
		t.Fatalf("unexpected warnings: %#v", result.Warnings)
	}
}

func TestParseStatic_ScopeModule(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  namespace :admin do
    scope module: :reports, as: "reports" do
      resources :users, only: %i[index]
    end
  end
end
`)
	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", result.Warnings)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("got %d routes, want 1: %#v", len(result.Entries), result.Entries)
	}
	entry := result.Entries[0]
	if entry.URIPattern != "/admin/users" ||
		entry.ControllerAction != "admin/reports/users#index" ||
		entry.Prefix != "admin_reports_users" {
		t.Fatalf("unexpected scoped route: %#v", entry)
	}
}
