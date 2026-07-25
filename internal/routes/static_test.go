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

func TestParseStatic_ParenthesizedNamespaces(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  resources :users, only: [] do
    namespace(:exports) do
      resources :reports, only: :create
    end
  end
end
`)
	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 1 || len(result.Warnings) != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	want := routes.RouteEntry{
		Prefix:           "user_exports_reports",
		Verb:             "POST",
		URIPattern:       "/users/:user_id/exports/reports",
		ControllerAction: "exports/reports#create",
	}
	if result.Entries[0] != want {
		t.Fatalf("route = %#v, want %#v", result.Entries[0], want)
	}
}

func TestParseStatic_StaticInlineRouteBlocks(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  namespace(:admin) { resources :reports, only: %i[new create] }
  namespace :api { get "status/{ready}", to: "status#show" }
  namespace(:v1) { get "items/:id", to: "items#show", constraints: { id: /[0-9]{2}/ } }
  resources :users, only: [] do
    member { get :preview }
    collection { post :search }
  end
end
`)
	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 6 || len(result.Warnings) != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	want := []routes.RouteEntry{
		{Prefix: "admin_reports", Verb: "POST", URIPattern: "/admin/reports", ControllerAction: "admin/reports#create"},
		{Prefix: "new_admin_report", Verb: "GET", URIPattern: "/admin/reports/new", ControllerAction: "admin/reports#new"},
		{Prefix: "api_show", Verb: "GET", URIPattern: "/api/status/{ready}", ControllerAction: "api/status#show"},
		{Prefix: "v1_show", Verb: "GET", URIPattern: "/v1/items/:id", ControllerAction: "v1/items#show {id: /[0-9]{2}/}"},
		{Prefix: "preview_user", Verb: "GET", URIPattern: "/users/:id/preview", ControllerAction: "users#preview"},
		{Prefix: "search_users", Verb: "POST", URIPattern: "/users/search", ControllerAction: "users#search"},
	}
	for index, entry := range result.Entries {
		if entry != want[index] {
			t.Errorf("route %d = %#v, want %#v", index, entry, want[index])
		}
	}
}

func TestParseStatic_UnsupportedInlineRouteBlocksWarnAndContinue(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  namespace(:dynamic) { ACTIONS.each { |action| get action } }
  namespace(:multiple) { get "one"; get "two" }
  namespace(:nested) { resources :items do }
  namespace(:broken) { resources :items
  member(extra) { get :invalid }
  member { get :outside }
  get "valid", to: "items#index"
end
`)
	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 1 || len(result.Warnings) != 6 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Entries[0].URIPattern != "/valid" {
		t.Fatalf("following valid route was lost: %#v", result.Entries)
	}
	joined := ""
	for _, warning := range result.Warnings {
		joined += warning.Message + "\n"
	}
	for _, fragment := range []string{
		"dynamic inline namespace",
		"malformed inline namespace",
		"dynamic inline member",
		"member block outside a resource",
	} {
		if !strings.Contains(joined, fragment) {
			t.Errorf("warnings missing %q: %s", fragment, joined)
		}
	}
}

func TestParseStatic_InlineAndParenthesizedNamespacesInConcernsAndDrawnFiles(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  concern :visible do
    namespace(:audit) { get "summary", to: "reports#summary" }
  end
  concerns :visible
  draw :extra
end
`)
	routesDir := filepath.Join(filepath.Dir(path), "routes")
	mustWriteStaticRouteFile(t, filepath.Join(routesDir, "extra.rb"), `
namespace(:api) do
  get "detail", to: "reports#detail"
end
`)

	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 2 || len(result.Warnings) != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	want := []routes.RouteEntry{
		{Prefix: "audit_summary", Verb: "GET", URIPattern: "/audit/summary", ControllerAction: "audit/reports#summary"},
		{Prefix: "api_detail", Verb: "GET", URIPattern: "/api/detail", ControllerAction: "api/reports#detail"},
	}
	for index, entry := range result.Entries {
		if entry != want[index] {
			t.Errorf("route %d = %#v, want %#v", index, entry, want[index])
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

func TestParseStatic_MultilineVerbAndMatchRoutes(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  get "/items,archived/:id",
      to: "items#show",
      as: :item_preview,
      constraints: {
        id: /[0-9]+/
      }
  match "/items",
        to: "items#update",
        via: [
          :put,
          :patch
        ],
        as: :items
  get "/legacy",
      to: redirect(
        "/items",
        status: 302
      ),
      as: :legacy # trailing comment
end
`)
	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 3 || len(result.Warnings) != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	want := []routes.RouteEntry{
		{Prefix: "item_preview", Verb: "GET", URIPattern: "/items,archived/:id", ControllerAction: "items#show {id: /[0-9]+/}"},
		{Prefix: "items", Verb: "PUT|PATCH", URIPattern: "/items", ControllerAction: "items#update"},
		{Prefix: "legacy", Verb: "GET", URIPattern: "/legacy", ControllerAction: "redirect(302, /items)"},
	}
	for index, entry := range result.Entries {
		if entry != want[index] {
			t.Errorf("route %d = %#v, want %#v", index, entry, want[index])
		}
	}
}

func TestParseStatic_MultilineVerbWarningsUseStartingLine(t *testing.T) {
	path := writeRoutesFile(t, `
get "/dynamic",
    to: target_for(:item)
get "/valid",
    to: "items#index"
`)
	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 1 || len(result.Warnings) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Entries[0].ControllerAction != "items#index" {
		t.Fatalf("following route was corrupted: %#v", result.Entries)
	}
	if result.Warnings[0].Line != 2 ||
		!strings.Contains(result.Warnings[0].Message, "dynamic route target") {
		t.Fatalf("unexpected warning: %#v", result.Warnings[0])
	}
}

func TestParseStatic_MultilineVerbsInConcernsAndDrawnFiles(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  concern :visible do
    get "/summary",
        to: "reports#summary",
        as: :summary
  end
  concerns :visible
  draw :extra
end
`)
	routesDir := filepath.Join(filepath.Dir(path), "routes")
	drawnPath := filepath.Join(routesDir, "extra.rb")
	mustWriteStaticRouteFile(t, drawnPath, `get "/dynamic",
  to: target_for(:item)
get "/detail",
  to: "reports#detail",
  as: :detail`)

	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 2 || len(result.Warnings) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Entries[0].Prefix != "summary" ||
		result.Entries[0].ControllerAction != "reports#summary" ||
		result.Entries[1].Prefix != "detail" ||
		result.Entries[1].ControllerAction != "reports#detail" {
		t.Fatalf("multiline expansion lost route metadata: %#v", result.Entries)
	}
	if result.Warnings[0].Path != drawnPath ||
		result.Warnings[0].Line != 1 ||
		!strings.Contains(result.Warnings[0].Message, "dynamic route target") {
		t.Fatalf("unexpected drawn-file warning: %#v", result.Warnings[0])
	}
}

func TestParseStatic_UnterminatedMultilineVerbWarns(t *testing.T) {
	path := writeRoutesFile(t, `get "/unfinished",`)
	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 0 || len(result.Warnings) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Warnings[0].Line != 1 ||
		result.Warnings[0].Message != "unterminated multiline route declaration" {
		t.Fatalf("unexpected warning: %#v", result.Warnings[0])
	}
}

func TestParseStatic_MatchRoutes(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  namespace :admin do
    match "reports", to: "reports#index", via: :get, as: :reports
    match "events", controller: "events", action: :create, via: [:post, :options]
    match "legacy" => redirect("/new"), via: %i[get head]
  end
  resources :users do
    match "preview", action: :preview, on: :member, via: %w[head options], constraints: { id: /\d+/ }
  end
  match "/fallback", to: "fallback#show", via: :all
end
`)
	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 13 {
		t.Fatalf("got %d entries, want 13: %#v", len(result.Entries), result.Entries)
	}

	want := map[string]routes.RouteEntry{
		"/admin/reports": {
			Prefix:           "admin_reports",
			Verb:             "GET",
			URIPattern:       "/admin/reports",
			ControllerAction: "admin/reports#index",
		},
		"/admin/events": {
			Prefix:           "admin_create",
			Verb:             "POST|OPTIONS",
			URIPattern:       "/admin/events",
			ControllerAction: "admin/events#create",
		},
		"/admin/legacy": {
			Verb:             "GET|HEAD",
			URIPattern:       "/admin/legacy",
			ControllerAction: "redirect(301, /new)",
		},
		"/users/:id/preview": {
			Prefix:           "preview_user",
			Verb:             "HEAD|OPTIONS",
			URIPattern:       "/users/:id/preview",
			ControllerAction: `users#preview {id: /\d+/}`,
		},
		"/fallback": {
			Prefix:           "show",
			Verb:             "",
			URIPattern:       "/fallback",
			ControllerAction: "fallback#show",
		},
	}
	found := 0
	for _, entry := range result.Entries {
		expected, ok := want[entry.URIPattern]
		if !ok {
			continue
		}
		found++
		if entry != expected {
			t.Errorf("route %q = %#v, want %#v", entry.URIPattern, entry, expected)
		}
	}
	if found != len(want) {
		t.Errorf("found %d match routes, want %d", found, len(want))
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", result.Warnings)
	}
}

func TestParseStatic_InvalidMatchRoutesWarnAndContinue(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  match "/missing", to: "items#show"
  match "/empty", to: "items#show", via: []
  match "/unknown", to: "items#show", via: :trace
  match "/dynamic", to: "items#show", via: methods
  match "/mixed", to: "items#show", via: [:all, :get]
  match "/callable", to: ->(_env) { [200, {}, []] }, via: :get
  match "/valid", to: "items#show", via: :get
end
`)
	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 1 || result.Entries[0].URIPattern != "/valid" {
		t.Fatalf("valid match route was not preserved: %#v", result.Entries)
	}
	if len(result.Warnings) != 6 {
		t.Fatalf("got %d warnings, want 6: %#v", len(result.Warnings), result.Warnings)
	}
	joined := ""
	for _, warning := range result.Warnings {
		joined += warning.Message + "\n"
	}
	for _, fragment := range []string{"requires a static via", "empty via", "trace", "methods", "all", "dynamic route target"} {
		if !strings.Contains(joined, fragment) {
			t.Errorf("warnings missing %q: %s", fragment, joined)
		}
	}
}

func TestParseStatic_MountRoutes(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  mount GenericRack => "/legacy"
  namespace :admin do
    mount Generic::HTTPServer, at: "/service", as: :gateway
  end
  resources :users, only: [] do
    mount Generic::Engine, at: "tools", as: nil
  end
end
`)
	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 3 || len(result.Warnings) != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	want := []routes.RouteEntry{
		{Prefix: "generic_rack", Verb: "", URIPattern: "/legacy", ControllerAction: "GenericRack"},
		{Prefix: "admin_gateway", Verb: "", URIPattern: "/admin/service", ControllerAction: "Generic::HTTPServer"},
		{Prefix: "", Verb: "", URIPattern: "/users/:user_id/tools", ControllerAction: "Generic::Engine"},
	}
	for index, entry := range result.Entries {
		if entry != want[index] {
			t.Errorf("mount %d = %#v, want %#v", index, entry, want[index])
		}
	}
	table := routes.FormatTable(result.Entries)
	if !strings.Contains(table, "generic_rack") || !strings.Contains(table, "/legacy") || !strings.Contains(table, "GenericRack") {
		t.Fatalf("mount with blank verb was not formatted: %q", table)
	}
}

func TestParseStatic_MountReceiverTargets(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  mount GenericServer.server => "/socket"
  namespace :admin do
    mount Generic::Engine.routes, at: "/engine", as: :mounted_engine
  end
end
`)
	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 2 || len(result.Warnings) != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	want := []routes.RouteEntry{
		{Prefix: "", Verb: "", URIPattern: "/socket", ControllerAction: "GenericServer.server"},
		{Prefix: "admin_mounted_engine", Verb: "", URIPattern: "/admin/engine", ControllerAction: "Generic::Engine.routes"},
	}
	for index, entry := range result.Entries {
		if entry != want[index] {
			t.Errorf("mount %d = %#v, want %#v", index, entry, want[index])
		}
	}
}

func TestParseStatic_ConditionalMountsAreIncludedWithWarnings(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  mount Generic::Engine, at: "/enabled" if feature_enabled?
  mount GenericServer.server => "/fallback" unless disabled?({ reason: "if needed" })
  mount Generic::Quoted, at: "/path if available"
end
`)
	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 3 || len(result.Warnings) != 2 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Entries[0].URIPattern != "/enabled" ||
		result.Entries[1].URIPattern != "/fallback" ||
		result.Entries[2].URIPattern != "/path if available" {
		t.Fatalf("conditional parsing changed mount paths: %#v", result.Entries)
	}
	for _, warning := range result.Warnings {
		if warning.Message != "conditional mount is included without evaluating its condition" {
			t.Errorf("unexpected warning: %#v", warning)
		}
	}
}

func TestParseStatic_MountsInConcernsAndDrawnFiles(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  concern :mountable do
    mount GenericRack, at: "rack"
  end
  namespace :admin do
    concerns :mountable
    draw :extra
  end
end
`)
	routesDir := filepath.Join(filepath.Dir(path), "routes")
	mustWriteStaticRouteFile(t, filepath.Join(routesDir, "extra.rb"), `mount Generic::Engine => "/engine"`)

	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 2 || len(result.Warnings) != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Entries[0].Prefix != "admin_generic_rack" ||
		result.Entries[0].URIPattern != "/admin/rack" ||
		result.Entries[1].Prefix != "admin_generic_engine" ||
		result.Entries[1].URIPattern != "/admin/engine" {
		t.Fatalf("mount context was not preserved: %#v", result.Entries)
	}
}

func TestParseStatic_DynamicMountsWarnAndContinue(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  mount app_for(:generic), at: "/dynamic-target"
  mount Generic::Engine.routes(:isolated), at: "/routes-call"
  mount GenericRack, at: dynamic_path
  mount GenericRack, at: "/interpolated/#{segment}"
  mount GenericRack, at: "/approximate" if feature_enabled?
  mount GenericRack, at: "/malformed" if
  mount GenericRack,
    at: "/multiline"
  mount GenericRack, at: "/valid"
end
`)
	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 2 || len(result.Warnings) != 7 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Entries[0].URIPattern != "/approximate" ||
		result.Entries[1].URIPattern != "/valid" {
		t.Fatalf("valid and approximate mounts were not preserved: %#v", result.Entries)
	}
	if result.Warnings[4].Message != "conditional mount is included without evaluating its condition" {
		t.Fatalf("unexpected conditional mount warning: %#v", result.Warnings[4])
	}
	if !strings.Contains(result.Warnings[5].Message, "malformed postfix condition") {
		t.Fatalf("unexpected malformed condition warning: %#v", result.Warnings[5])
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

func TestParseStaticDetailed_PositionalScopePath(t *testing.T) {
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
	if len(result.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", result.Warnings)
	}
	if result.Entries[1].URIPattern != "/admin/health" ||
		result.Entries[1].ControllerAction != "health#show" {
		t.Errorf("unexpected scoped route: %#v", result.Entries[1])
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

func TestParseStatic_ResourceHelpersAppearOnFirstEnabledRoute(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  resources :users
  resources :photos, only: :create
  resources :records, only: :update
  resources :logs, only: :destroy
  resource :profile
  resource :session, only: :create
  resource :preference, only: :update
end
`)
	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", result.Warnings)
	}

	type routeKey struct {
		controller string
		action     string
		verb       string
	}
	got := make(map[routeKey]string)
	for _, entry := range result.Entries {
		controllerAction := splitCA(entry.ControllerAction)
		got[routeKey{controllerAction[0], controllerAction[1], entry.Verb}] = entry.Prefix
	}
	want := map[routeKey]string{
		{"users", "index", "GET"}:         "users",
		{"users", "create", "POST"}:       "",
		{"users", "show", "GET"}:          "user",
		{"users", "update", "PATCH"}:      "",
		{"users", "update", "PUT"}:        "",
		{"users", "destroy", "DELETE"}:    "",
		{"photos", "create", "POST"}:      "photos",
		{"records", "update", "PATCH"}:    "record",
		{"records", "update", "PUT"}:      "",
		{"logs", "destroy", "DELETE"}:     "log",
		{"profile", "show", "GET"}:        "profile",
		{"profile", "create", "POST"}:     "",
		{"profile", "update", "PATCH"}:    "",
		{"profile", "update", "PUT"}:      "",
		{"profile", "destroy", "DELETE"}:  "",
		{"session", "create", "POST"}:     "session",
		{"preference", "update", "PATCH"}: "preference",
		{"preference", "update", "PUT"}:   "",
	}
	for key, prefix := range want {
		if got[key] != prefix {
			t.Errorf("%#v prefix = %q, want %q", key, got[key], prefix)
		}
	}

	table := routes.FormatTable(result.Entries)
	if !strings.Contains(table, "POST    /users") ||
		!strings.Contains(table, "PATCH   /users/:id") {
		t.Fatalf("blank resource prefixes were not rendered: %q", table)
	}
	filtered, filterErr := routes.FilterEntries(result.Entries, []string{"users#create"})
	if filterErr != nil || len(filtered) != 1 || filtered[0].Prefix != "" {
		t.Fatalf("blank-prefix route was not filterable: %#v, %v", filtered, filterErr)
	}
}

func TestParseStatic_ResourceHelperSuppressionInConcernsAndDrawnFiles(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  concern :mutable do
    resource :profile, only: :update
  end
  namespace :admin do
    concerns :mutable
    draw :extra
  end
end
`)
	routesDir := filepath.Join(filepath.Dir(path), "routes")
	mustWriteStaticRouteFile(t, filepath.Join(routesDir, "extra.rb"), `resources :reports, only: :create`)

	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 3 || len(result.Warnings) != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	wantPrefixes := []string{"admin_profile", "", "admin_reports"}
	for index, prefix := range wantPrefixes {
		if result.Entries[index].Prefix != prefix {
			t.Errorf("route %d prefix = %q, want %q", index, result.Entries[index].Prefix, prefix)
		}
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
	if len(result.Entries) != 4 {
		t.Fatalf("got %d routes, want 4: %#v", len(result.Entries), result.Entries)
	}
	want := map[string]routes.RouteEntry{
		"profile":             {Prefix: "profile", Verb: "GET", URIPattern: "/users/profile", ControllerAction: "users#profile"},
		"show":                {Prefix: "health", Verb: "GET", URIPattern: "/health", ControllerAction: "health#show"},
		"update {id: /\\d+/}": {Prefix: "update_user", Verb: "PATCH", URIPattern: "/users/:id", ControllerAction: `users#update {id: /\d+/}`},
		"":                    {Prefix: "", Verb: "GET", URIPattern: "/legacy", ControllerAction: "redirect(301, /new)"},
	}
	for _, entry := range result.Entries {
		action := splitCA(entry.ControllerAction)[1]
		if entry != want[action] {
			t.Errorf("%s route = %#v, want %#v", action, entry, want[action])
		}
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", result.Warnings)
	}
}

func TestParseStatic_LiteralRedirects(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  namespace :admin do
    get "old", to: redirect("/new?from=old#details"), as: :legacy
    get "docs/:id" => redirect('/wiki/%{id}', status: 307)
    get "external", to: redirect("https://example.test/new")
    get "relative", to: redirect("current,page")
  end
end
`)

	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Warnings) != 0 || len(result.Entries) != 4 {
		t.Fatalf("unexpected result: %#v", result)
	}
	want := map[string]routes.RouteEntry{
		"/admin/old": {
			Prefix:           "admin_legacy",
			Verb:             "GET",
			URIPattern:       "/admin/old",
			ControllerAction: "redirect(301, /new?from=old#details)",
		},
		"/admin/docs/:id": {
			Verb:             "GET",
			URIPattern:       "/admin/docs/:id",
			ControllerAction: "redirect(307, /wiki/%{id})",
		},
		"/admin/external": {
			Verb:             "GET",
			URIPattern:       "/admin/external",
			ControllerAction: "redirect(301, https://example.test/new)",
		},
		"/admin/relative": {
			Verb:             "GET",
			URIPattern:       "/admin/relative",
			ControllerAction: "redirect(301, current,page)",
		},
	}
	for _, entry := range result.Entries {
		if entry != want[entry.URIPattern] {
			t.Errorf("redirect route = %#v, want %#v", entry, want[entry.URIPattern])
		}
	}
}

func TestParseStatic_RedirectPreservesResourceContextAndConstraintsWarning(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  resources :users, only: [] do
    get :legacy, on: :member, to: redirect("/users", status: 308), constraints: { id: /\d+/ }
  end
end
`)

	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 1 || len(result.Warnings) != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	entry := result.Entries[0]
	if entry.Prefix != "" || entry.Verb != "GET" ||
		entry.URIPattern != "/users/:id/legacy" ||
		entry.ControllerAction != `redirect(308, /users) {id: /\d+/}` {
		t.Fatalf("unexpected resource redirect: %#v", entry)
	}
}

func TestParseStatic_PathConstraints(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  get "/items/:section/:id", to: "items#show", constraints: { :id => /[0-9]+/i, section: /[a-z\/]{2,4}/mx }
end
`)

	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 1 || len(result.Warnings) != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	entry := result.Entries[0]
	if entry.ControllerAction != `items#show {id: /[0-9]+/i, section: /[a-z\/]{2,4}/mx}` {
		t.Fatalf("unexpected constrained route: %#v", entry)
	}
}

func TestParseStatic_PathConstraintsInConcernsAndDrawnFiles(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  concern :constrained do
    get "items/:token", to: "items#show", constraints: { token: /[a-z]+/ }
  end
  namespace :admin do
    concerns :constrained
    draw :extra
  end
end
`)
	routesDir := filepath.Join(filepath.Dir(path), "routes")
	mustWriteStaticRouteFile(t, filepath.Join(routesDir, "extra.rb"),
		`get "reports/:slug", to: "reports#show", constraints: { slug: /[a-z-]+/ }`)

	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 2 || len(result.Warnings) != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	want := map[string]string{
		"/admin/items/:token":  `admin/items#show {token: /[a-z]+/}`,
		"/admin/reports/:slug": `admin/reports#show {slug: /[a-z-]+/}`,
	}
	for _, entry := range result.Entries {
		if entry.ControllerAction != want[entry.URIPattern] {
			t.Errorf("route %q = %#v, want endpoint %q", entry.URIPattern, entry, want[entry.URIPattern])
		}
	}
}

func TestParseStatic_UnsupportedPathConstraintsWarnAndPreserveRoutes(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  get "/partial/:id", to: "items#show", constraints: { id: /[0-9]+/, host: /example/ }
  get "/string/:id", to: "items#show", constraints: { id: "numeric" }
  get "/object/:id", to: "items#show", constraints: ConstraintObject.new
  get "/callable/:id", to: "items#show", constraints: ->(request) { request.local? }
  get "/interpolated/:id", to: "items#show", constraints: { id: /#{pattern}/ }
  get "/malformed/:id", to: "items#show", constraints: { id: /[0-9]+/
  get "/valid/:id", to: "items#show", constraints: { id: /[0-9]+/ }
end
`)

	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 7 || len(result.Warnings) != 6 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Entries[0].ControllerAction != `items#show {id: /[0-9]+/}` ||
		result.Warnings[0].Message != "route constraints are only partially modeled" {
		t.Fatalf("unexpected partial constraint result: %#v, %#v", result.Entries[0], result.Warnings[0])
	}
	for _, entry := range result.Entries[1:6] {
		if strings.Contains(entry.ControllerAction, " {") {
			t.Errorf("unsupported constraint leaked into endpoint: %#v", entry)
		}
	}
	if result.Entries[6].ControllerAction != `items#show {id: /[0-9]+/}` {
		t.Fatalf("valid route after warnings was not modeled: %#v", result.Entries[6])
	}
	for _, warning := range result.Warnings[1:] {
		if warning.Message != "route constraints are not modeled" {
			t.Errorf("unexpected warning: %#v", warning)
		}
	}
}

func TestParseStatic_DynamicRedirectsWarnAndContinue(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  get "interpolated", to: redirect("https://#{request.host}/new")
  get "callable", to: redirect(GenericRedirect.new)
  get "options", to: redirect(path: "/new")
  get "status", to: redirect("/new", status: :temporary_redirect)
  get "block", to: redirect { |params| "/#{params[:id]}" }
  get "available", to: redirect("/new")
end
`)

	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 1 ||
		result.Entries[0].ControllerAction != "redirect(301, /new)" {
		t.Fatalf("valid redirect was not preserved: %#v", result.Entries)
	}
	if len(result.Warnings) != 5 {
		t.Fatalf("got %d warnings, want 5: %#v", len(result.Warnings), result.Warnings)
	}
	for _, warning := range result.Warnings {
		if !strings.Contains(warning.Message, "dynamic redirect target") {
			t.Errorf("unexpected warning: %#v", warning)
		}
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

func TestParseStatic_ScopeOptionsComposeIndependently(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  namespace :admin do
    scope path: :api, module: "v1", as: :api do
      get "reports", to: "reports#index"
      scope controller: :health, as: "ops" do
        get :status
      end
    end
  end
end
`)
	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 2 || len(result.Warnings) != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	want := []routes.RouteEntry{
		{Prefix: "admin_api_index", Verb: "GET", URIPattern: "/admin/api/reports", ControllerAction: "admin/v1/reports#index"},
		{Prefix: "admin_api_ops_status", Verb: "GET", URIPattern: "/admin/api/status", ControllerAction: "admin/v1/health#status"},
	}
	for index, entry := range result.Entries {
		if entry != want[index] {
			t.Errorf("route %d = %#v, want %#v", index, entry, want[index])
		}
	}
}

func TestParseStatic_ScopeContextFlowsIntoConcernsAndDrawnFiles(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  concern :visible do
    get "items", to: "items#index"
  end
  scope "/public", module: :api, as: :public do
    concerns :visible
    draw :extra
  end
end
`)
	routesDir := filepath.Join(filepath.Dir(path), "routes")
	mustWriteStaticRouteFile(t, filepath.Join(routesDir, "extra.rb"), `get "reports", to: "reports#index"`)

	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 2 || len(result.Warnings) != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Entries[0].URIPattern != "/public/items" ||
		result.Entries[0].ControllerAction != "api/items#index" ||
		result.Entries[0].Prefix != "public_index" ||
		result.Entries[1].URIPattern != "/public/reports" ||
		result.Entries[1].ControllerAction != "api/reports#index" ||
		result.Entries[1].Prefix != "public_index" {
		t.Fatalf("scope context was not inherited: %#v", result.Entries)
	}
}

func TestParseStatic_ControllerBlocks(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  controller :items do
    get "show"
    get "explicit", to: "other#index"
    controller "previews" do
      get "latest"
    end
  end
  namespace :admin do
    scope path: :api, module: :v1, as: :api do
      controller :items do
        get "status"
      end
    end
  end
  resources :users, only: [] do
    controller :previews do
      get "preview", on: :member
    end
  end
end
`)
	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 5 || len(result.Warnings) != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	want := []routes.RouteEntry{
		{Prefix: "show", Verb: "GET", URIPattern: "/show", ControllerAction: "items#show"},
		{Prefix: "index", Verb: "GET", URIPattern: "/explicit", ControllerAction: "other#index"},
		{Prefix: "latest", Verb: "GET", URIPattern: "/latest", ControllerAction: "previews#latest"},
		{Prefix: "admin_api_status", Verb: "GET", URIPattern: "/admin/api/status", ControllerAction: "admin/v1/items#status"},
		{Prefix: "preview_user", Verb: "GET", URIPattern: "/users/:id/preview", ControllerAction: "previews#preview"},
	}
	for index, entry := range result.Entries {
		if entry != want[index] {
			t.Errorf("route %d = %#v, want %#v", index, entry, want[index])
		}
	}
}

func TestParseStatic_ControllerContextFlowsIntoConcernsAndDrawnFiles(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  concern :visible do
    get "summary"
  end
  controller :reports do
    concerns :visible
    draw :extra
  end
end
`)
	routesDir := filepath.Join(filepath.Dir(path), "routes")
	mustWriteStaticRouteFile(t, filepath.Join(routesDir, "extra.rb"), `get "detail"`)

	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 2 || len(result.Warnings) != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Entries[0].ControllerAction != "reports#summary" ||
		result.Entries[1].ControllerAction != "reports#detail" {
		t.Fatalf("controller context was not inherited: %#v", result.Entries)
	}
}

func TestParseStatic_DynamicControllerBlocksWarnAndSkip(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  controller dynamic_controller do
    get "dynamic"
  end
  controller "#{controller_name}" do
    get "interpolated"
  end
  controller app.controller do
    get "callable"
  end
  controller :missing
  get "valid", to: "items#index"
end
`)
	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 1 || len(result.Warnings) != 4 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Entries[0].URIPattern != "/valid" {
		t.Fatalf("valid route after controller warnings was lost: %#v", result.Entries)
	}
	for _, warning := range result.Warnings {
		if !strings.Contains(warning.Message, "controller") {
			t.Errorf("unexpected warning: %#v", warning)
		}
	}
}

func TestParseStatic_UnsafeScopesWarnAndSkipOrApproximate(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  scope dynamic_path do
    get "dynamic", to: "items#index"
  end
  scope "/conflict", path: "/other" do
    get "conflict", to: "items#index"
  end
  scope "/interpolated/#{segment}" do
    get "interpolated", to: "items#index"
  end
  resources :users, only: [] do
    scope path: "tools", as: :tools do
      get "preview", to: "users#preview", on: :member
    end
  end
  scope path: "/public", defaults: { format: :json } do
    get "approximate", to: "items#index"
  end
  scope "/missing"
  get "valid", to: "items#index"
end
`)
	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 3 || len(result.Warnings) != 5 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Entries[0].URIPattern != "/tools/users/:id/preview" ||
		result.Entries[0].Prefix != "preview_tools_user" ||
		result.Entries[1].URIPattern != "/public/approximate" ||
		result.Entries[2].URIPattern != "/valid" {
		t.Fatalf("unsafe scopes leaked routes or valid routes were lost: %#v", result.Entries)
	}
	if result.Warnings[3].Message != "scope options are only partially modeled" {
		t.Fatalf("unexpected partial scope warning: %#v", result.Warnings[3])
	}
}

func TestParseStatic_DrawFormsPreserveOrder(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  get "before", to: "before#show"
  draw :users
  draw("reports")
  draw 'health'
  get "after", to: "after#show"
end
`)
	routesDir := filepath.Join(filepath.Dir(path), "routes")
	mustWriteStaticRouteFile(t, filepath.Join(routesDir, "users.rb"), `get "users", to: "users#index"`)
	mustWriteStaticRouteFile(t, filepath.Join(routesDir, "reports.rb"), `get "reports", to: "reports#index"`)
	mustWriteStaticRouteFile(t, filepath.Join(routesDir, "health.rb"), `get "health", to: "health#show"`)

	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", result.Warnings)
	}
	var targets []string
	for _, entry := range result.Entries {
		targets = append(targets, entry.ControllerAction)
	}
	want := []string{"before#show", "users#index", "reports#index", "health#show", "after#show"}
	if strings.Join(targets, ",") != strings.Join(want, ",") {
		t.Fatalf("targets = %#v, want %#v", targets, want)
	}
}

func TestParseStatic_DrawInheritsResourceContext(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  namespace :admin do
    resources :users, only: [] do
      draw :comments
    end
  end
end
`)
	mustWriteStaticRouteFile(t, filepath.Join(filepath.Dir(path), "routes", "comments.rb"), `
resources :comments, only: :index
`)

	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Warnings) != 0 || len(result.Entries) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	entry := result.Entries[0]
	if entry.URIPattern != "/admin/users/:user_id/comments" ||
		entry.ControllerAction != "admin/comments#index" ||
		entry.Prefix != "admin_user_comments" {
		t.Fatalf("drawn route did not inherit context: %#v", entry)
	}
}

func TestParseStatic_NestedAndRepeatedDraws(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  draw :shared
  draw :nested
  draw :shared
end
`)
	routesDir := filepath.Join(filepath.Dir(path), "routes")
	mustWriteStaticRouteFile(t, filepath.Join(routesDir, "shared.rb"), `get "shared", to: "shared#show"`)
	mustWriteStaticRouteFile(t, filepath.Join(routesDir, "nested.rb"), `draw(:leaf)`)
	mustWriteStaticRouteFile(t, filepath.Join(routesDir, "leaf.rb"), `get "leaf", to: "leaf#show"`)

	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Warnings) != 0 || len(result.Entries) != 3 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Entries[0].ControllerAction != "shared#show" ||
		result.Entries[1].ControllerAction != "leaf#show" ||
		result.Entries[2].ControllerAction != "shared#show" {
		t.Fatalf("unexpected draw order: %#v", result.Entries)
	}
}

func TestParseStatic_DrawFailuresWarnAndContinue(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  draw :missing
  draw dynamic_name
  draw "../outside"
  draw :cycle
  draw :unsupported
  get "available", to: "available#show"
end
`)
	routesDir := filepath.Join(filepath.Dir(path), "routes")
	mustWriteStaticRouteFile(t, filepath.Join(routesDir, "cycle.rb"), `draw :cycle`)
	unsupportedPath := filepath.Join(routesDir, "unsupported.rb")
	mustWriteStaticRouteFile(t, unsupportedPath, `
mount engine_for(:generic), at: "/engine"
`)

	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 1 || result.Entries[0].ControllerAction != "available#show" {
		t.Fatalf("valid routes were not preserved: %#v", result.Entries)
	}
	if len(result.Warnings) != 5 {
		t.Fatalf("got %d warnings, want 5: %#v", len(result.Warnings), result.Warnings)
	}
	messages := make([]string, len(result.Warnings))
	for i, warning := range result.Warnings {
		messages[i] = warning.Message
	}
	joined := strings.Join(messages, "\n")
	for _, fragment := range []string{"could not be read", "dynamic draw", "unsafe or unsupported", "cyclic draw", "dynamic mount target"} {
		if !strings.Contains(joined, fragment) {
			t.Errorf("warnings missing %q: %s", fragment, joined)
		}
	}
	last := result.Warnings[len(result.Warnings)-1]
	if last.Path != unsupportedPath || last.Line != 2 {
		t.Fatalf("drawn warning source = %#v, want %s:2", last, unsupportedPath)
	}
}

func TestParseStatic_DrawRejectsEscapingSymlink(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  draw :linked
end
`)
	outside := filepath.Join(filepath.Dir(path), "outside.rb")
	mustWriteStaticRouteFile(t, outside, `get "outside"`)
	routesDir := filepath.Join(filepath.Dir(path), "routes")
	if err := os.MkdirAll(routesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(routesDir, "linked.rb")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 0 || len(result.Warnings) != 1 ||
		!strings.Contains(result.Warnings[0].Message, "through a symlink") {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestParseStatic_ConcernDefinitionAndResourceExpansion(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  concern :commentable do
    resources :comments, only: :index
  end

  namespace :admin do
    resources :users, only: [] do
      concerns :commentable
    end
  end
end
`)

	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Warnings) != 0 || len(result.Entries) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	entry := result.Entries[0]
	if entry.URIPattern != "/admin/users/:user_id/comments" ||
		entry.ControllerAction != "admin/comments#index" ||
		entry.Prefix != "admin_user_comments" {
		t.Fatalf("concern did not inherit resource context: %#v", entry)
	}
}

func TestParseStatic_ConcernInvocationFormsAndInlineResources(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  concern :commentable do
    resources :comments, only: :index
  end
  concern :taggable do
    resources :tags, only: :index
  end

  resources :posts, only: [], concerns: [:commentable, :taggable]
  resource :profile, only: [], concerns: :taggable
  namespace :admin do
    concerns :commentable, :taggable
  end
end
`)

	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Warnings) != 0 || len(result.Entries) != 5 {
		t.Fatalf("unexpected result: %#v", result)
	}
	wantPaths := map[string]bool{
		"/posts/:post_id/comments": true,
		"/posts/:post_id/tags":     true,
		"/profile/tags":            true,
		"/admin/comments":          true,
		"/admin/tags":              true,
	}
	for _, entry := range result.Entries {
		if !wantPaths[entry.URIPattern] {
			t.Errorf("unexpected concern route: %#v", entry)
		}
	}
}

func TestParseStatic_ConcernRegistrySpansDrawnFiles(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  draw :definitions
  draw :usage
end
`)
	routesDir := filepath.Join(filepath.Dir(path), "routes")
	mustWriteStaticRouteFile(t, filepath.Join(routesDir, "definitions.rb"), `
concern :reportable do
  resources :reports, only: :index
end
`)
	mustWriteStaticRouteFile(t, filepath.Join(routesDir, "usage.rb"), `
namespace :admin do
  concerns :reportable
end
`)

	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Warnings) != 0 || len(result.Entries) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Entries[0].URIPattern != "/admin/reports" ||
		result.Entries[0].ControllerAction != "admin/reports#index" {
		t.Fatalf("unexpected cross-file concern route: %#v", result.Entries[0])
	}
}

func TestParseStatic_NestedConcernUsesLatestDefinition(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  concern :searchable do
    get "old", to: "search#old"
  end
  concern :searchable do
    get "new", to: "search#new"
  end
  concern :container do
    concerns :searchable
  end

  concerns [:container]
end
`)

	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Warnings) != 0 || len(result.Entries) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Entries[0].URIPattern != "/new" || result.Entries[0].ControllerAction != "search#new" {
		t.Fatalf("latest concern definition was not used: %#v", result.Entries)
	}
}

func TestParseStatic_ConcernFailuresWarnAndContinue(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  concern :external, GenericCallable.new
  concern dynamic_name do
    resources :ignored
  end
  concern :parameterized do |options|
    resources :comments, options
  end
  concern :recursive do
    concerns :recursive
  end
  concern :available do
    resources :reports, only: :index
  end

  concerns :external
  concerns :parameterized
  concerns :missing
  concerns :recursive
  concerns :available, only: [:index]
end
`)

	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 1 || result.Entries[0].ControllerAction != "reports#index" {
		t.Fatalf("valid concern routes were not preserved: %#v", result.Entries)
	}
	joined := ""
	for _, warning := range result.Warnings {
		joined += warning.Message + "\n"
	}
	for _, fragment := range []string{
		"callable or dynamic",
		"parameterized",
		"was not found",
		"cyclic route concern",
		"invocation options",
	} {
		if !strings.Contains(joined, fragment) {
			t.Errorf("warnings missing %q: %s", fragment, joined)
		}
	}
}

func TestParseStatic_ConcernBodyWarningKeepsSourceLine(t *testing.T) {
	path := writeRoutesFile(t, `
Rails.application.routes.draw do
  concern :mountable do
    mount engine_for(:generic), at: "/engine"
  end
  concerns :mountable
end
`)

	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("unexpected warnings: %#v", result.Warnings)
	}
	warning := result.Warnings[0]
	if warning.Path != path || warning.Line != 4 || !strings.Contains(warning.Message, "dynamic mount target") {
		t.Fatalf("unexpected concern warning source: %#v", warning)
	}
}

func TestParseStatic_UnterminatedConcernWarns(t *testing.T) {
	path := writeRoutesFile(t, `
concern :broken do
  resources :comments
`)

	result, err := routes.ParseStaticDetailed(path, pluralize.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 0 || len(result.Warnings) != 1 ||
		result.Warnings[0].Line != 2 ||
		!strings.Contains(result.Warnings[0].Message, "unterminated") {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func mustWriteStaticRouteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
