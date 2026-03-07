package routes_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/janstol/rails-kit/internal/routes"
)

const sampleRoutes = `                                  Prefix Verb   URI Pattern                    Controller#Action
                                    root GET    /                              home#index
                              GET    /users                             users#index
                              POST   /users                             users#create
                         new_user GET    /users/new                         users#new
                        edit_user GET    /users/:id/edit                    users#edit
                              GET    /users/:id                         users#show
                           DELETE /users/:id                         users#destroy`

const sampleRoutesWithNoise = `[DEPRECATION WARNING] Something something
Loading environment...
                                  Prefix Verb   URI Pattern                    Controller#Action
                                    root GET    /                              home#index
                              rails_info GET    /rails/info                    rails/info#properties
                              PATCH/PUT /users/:id                         users#update
                         new_user GET    /users/new                         users#new
                              GET    /users/:id                         users#show`

func TestCacheValid(t *testing.T) {
	dir := t.TempDir()

	// Create nested routes directory with a .rb file
	nestedDir := filepath.Join(dir, "config", "routes", "sub")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatal(err)
	}
	nestedFile := filepath.Join(nestedDir, "nested.rb")
	if err := os.WriteFile(nestedFile, []byte("# nested route"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create routes.rb
	routesRb := filepath.Join(dir, "config", "routes.rb")
	if err := os.MkdirAll(filepath.Dir(routesRb), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(routesRb, []byte("# routes"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create cache file with a time older than the source files
	cacheFile := filepath.Join(dir, "tmp", "routes_cache.txt")
	if err := os.MkdirAll(filepath.Dir(cacheFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cacheFile, []byte("cached output"), 0644); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-2 * time.Second)
	if err := os.Chtimes(cacheFile, past, past); err != nil {
		t.Fatal(err)
	}

	// Modify the nested file so it is newer than the cache
	if err := os.WriteFile(nestedFile, []byte("# updated nested route"), 0644); err != nil {
		t.Fatal(err)
	}

	// Cache should be invalid because nested file is newer
	out := routes.CacheValid(cacheFile, routesRb, filepath.Join(dir, "config", "routes"))
	if out {
		t.Error("expected cache to be invalid after nested file modification")
	}
}

func TestCacheValidNoRoutesDir(t *testing.T) {
	dir := t.TempDir()

	routesRb := filepath.Join(dir, "config", "routes.rb")
	if err := os.MkdirAll(filepath.Dir(routesRb), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(routesRb, []byte("# routes"), 0644); err != nil {
		t.Fatal(err)
	}

	cacheFile := filepath.Join(dir, "tmp", "routes_cache.txt")
	if err := os.MkdirAll(filepath.Dir(cacheFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cacheFile, []byte("cached output"), 0644); err != nil {
		t.Fatal(err)
	}
	// Make cache newer than routes.rb
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(cacheFile, future, future); err != nil {
		t.Fatal(err)
	}

	// config/routes/ dir does not exist — cache should still be valid
	valid := routes.CacheValid(cacheFile, routesRb, filepath.Join(dir, "config", "routes"))
	if !valid {
		t.Error("expected cache to be valid when routes dir does not exist")
	}
}

func TestCacheValidAfterDeletion(t *testing.T) {
	dir := t.TempDir()

	routesDir := filepath.Join(dir, "config", "routes")
	if err := os.MkdirAll(routesDir, 0755); err != nil {
		t.Fatal(err)
	}
	routeFile := filepath.Join(routesDir, "admin.rb")
	if err := os.WriteFile(routeFile, []byte("# admin routes"), 0644); err != nil {
		t.Fatal(err)
	}

	routesRb := filepath.Join(dir, "config", "routes.rb")
	if err := os.WriteFile(routesRb, []byte("# routes"), 0644); err != nil {
		t.Fatal(err)
	}

	cacheFile := filepath.Join(dir, "tmp", "routes_cache.txt")
	if err := os.MkdirAll(filepath.Dir(cacheFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cacheFile, []byte("cached output"), 0644); err != nil {
		t.Fatal(err)
	}

	// Backdate source files to -10s and the routesDir to -10s,
	// set cache to -5s so it appears newer than the files.
	veryPast := time.Now().Add(-10 * time.Second)
	slightlyPast := time.Now().Add(-5 * time.Second)

	for _, p := range []string{routesRb, routeFile, routesDir} {
		if err := os.Chtimes(p, veryPast, veryPast); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chtimes(cacheFile, slightlyPast, slightlyPast); err != nil {
		t.Fatal(err)
	}

	// Verify cache is valid before deletion
	valid := routes.CacheValid(cacheFile, routesRb, routesDir)
	if !valid {
		t.Fatal("expected cache to be valid before deletion")
	}

	// Delete the route file — directory mtime updates to now (after slightlyPast)
	if err := os.Remove(routeFile); err != nil {
		t.Fatal(err)
	}

	// Cache should now be invalid because the directory mtime changed
	valid = routes.CacheValid(cacheFile, routesRb, routesDir)
	if valid {
		t.Error("expected cache to be invalid after route file deletion")
	}
}

func TestCacheValidAfterNestedDeletion(t *testing.T) {
	dir := t.TempDir()

	routesDir := filepath.Join(dir, "config", "routes")
	subDir := filepath.Join(routesDir, "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	nestedFile := filepath.Join(subDir, "nested.rb")
	if err := os.WriteFile(nestedFile, []byte("# nested"), 0644); err != nil {
		t.Fatal(err)
	}

	routesRb := filepath.Join(dir, "config", "routes.rb")
	if err := os.WriteFile(routesRb, []byte("# routes"), 0644); err != nil {
		t.Fatal(err)
	}

	cacheFile := filepath.Join(dir, "tmp", "routes_cache.txt")
	if err := os.MkdirAll(filepath.Dir(cacheFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cacheFile, []byte("cached output"), 0644); err != nil {
		t.Fatal(err)
	}

	// Backdate all source files/dirs to -10s, set cache to -5s (newer)
	veryPast := time.Now().Add(-10 * time.Second)
	slightlyPast := time.Now().Add(-5 * time.Second)

	for _, p := range []string{routesRb, nestedFile, subDir, routesDir} {
		if err := os.Chtimes(p, veryPast, veryPast); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chtimes(cacheFile, slightlyPast, slightlyPast); err != nil {
		t.Fatal(err)
	}

	// Verify cache is valid before deletion
	valid := routes.CacheValid(cacheFile, routesRb, routesDir)
	if !valid {
		t.Fatal("expected cache to be valid before deletion")
	}

	// Delete nested.rb — updates sub/ mtime but NOT config/routes/ mtime
	if err := os.Remove(nestedFile); err != nil {
		t.Fatal(err)
	}

	// Cache should now be invalid because sub/ mtime changed
	valid = routes.CacheValid(cacheFile, routesRb, routesDir)
	if valid {
		t.Error("expected cache to be invalid after nested file deletion")
	}
}

func TestCacheValid_SameTimestamp(t *testing.T) {
	dir := t.TempDir()

	routesRb := filepath.Join(dir, "config", "routes.rb")
	if err := os.MkdirAll(filepath.Dir(routesRb), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(routesRb, []byte("# routes"), 0644); err != nil {
		t.Fatal(err)
	}

	cacheFile := filepath.Join(dir, "tmp", "routes_cache.txt")
	if err := os.MkdirAll(filepath.Dir(cacheFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cacheFile, []byte("cached output"), 0644); err != nil {
		t.Fatal(err)
	}

	// Set identical mtime on both files — cache should be considered invalid
	// because routes.rb was modified at the same tick as the cache was written.
	sameTime := time.Now().Add(-1 * time.Second)
	for _, p := range []string{routesRb, cacheFile} {
		if err := os.Chtimes(p, sameTime, sameTime); err != nil {
			t.Fatal(err)
		}
	}

	valid := routes.CacheValid(cacheFile, routesRb, filepath.Join(dir, "config", "routes"))
	if valid {
		t.Error("expected cache to be invalid when routes.rb mtime equals cache mtime")
	}
}

func TestCacheValidAfterNonRubyFileModification(t *testing.T) {
	dir := t.TempDir()

	routesDir := filepath.Join(dir, "config", "routes")
	if err := os.MkdirAll(routesDir, 0755); err != nil {
		t.Fatal(err)
	}
	routeFile := filepath.Join(routesDir, "admin.rb")
	if err := os.WriteFile(routeFile, []byte("# admin"), 0644); err != nil {
		t.Fatal(err)
	}
	nonRubyFile := filepath.Join(routesDir, "notes.txt")
	if err := os.WriteFile(nonRubyFile, []byte("initial"), 0644); err != nil {
		t.Fatal(err)
	}

	routesRb := filepath.Join(dir, "config", "routes.rb")
	if err := os.WriteFile(routesRb, []byte("# routes"), 0644); err != nil {
		t.Fatal(err)
	}

	cacheFile := filepath.Join(dir, "tmp", "routes_cache.txt")
	if err := os.MkdirAll(filepath.Dir(cacheFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cacheFile, []byte("cached output"), 0644); err != nil {
		t.Fatal(err)
	}

	veryPast := time.Now().Add(-10 * time.Second)
	slightlyPast := time.Now().Add(-5 * time.Second)
	for _, p := range []string{routesRb, routeFile, nonRubyFile, routesDir} {
		if err := os.Chtimes(p, veryPast, veryPast); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chtimes(cacheFile, slightlyPast, slightlyPast); err != nil {
		t.Fatal(err)
	}

	valid := routes.CacheValid(cacheFile, routesRb, routesDir)
	if !valid {
		t.Fatal("expected cache to be valid before non-ruby modification")
	}

	if err := os.WriteFile(nonRubyFile, []byte("updated"), 0644); err != nil {
		t.Fatal(err)
	}

	valid = routes.CacheValid(cacheFile, routesRb, routesDir)
	if !valid {
		t.Error("expected cache to remain valid after non-ruby file modification")
	}
}

func TestCacheValidAfterRoutesDirRemoved(t *testing.T) {
	dir := t.TempDir()

	routesDir := filepath.Join(dir, "config", "routes")
	if err := os.MkdirAll(routesDir, 0755); err != nil {
		t.Fatal(err)
	}
	routeFile := filepath.Join(routesDir, "admin.rb")
	if err := os.WriteFile(routeFile, []byte("# admin"), 0644); err != nil {
		t.Fatal(err)
	}

	routesRb := filepath.Join(dir, "config", "routes.rb")
	if err := os.WriteFile(routesRb, []byte("# routes"), 0644); err != nil {
		t.Fatal(err)
	}

	tmpDir := filepath.Join(dir, "tmp")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatal(err)
	}
	cacheFile := filepath.Join(tmpDir, "routes_cache.txt")
	if err := os.WriteFile(cacheFile, []byte("cached output"), 0644); err != nil {
		t.Fatal(err)
	}
	// Write flag file to simulate that config/routes/ existed at cache-write time.
	flagFile := filepath.Join(tmpDir, "routes_dir.flag")
	if err := os.WriteFile(flagFile, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	// Backdate source files and cache so cache appears newer.
	veryPast := time.Now().Add(-10 * time.Second)
	slightlyPast := time.Now().Add(-5 * time.Second)
	for _, p := range []string{routesRb, routeFile, routesDir} {
		if err := os.Chtimes(p, veryPast, veryPast); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{cacheFile, flagFile} {
		if err := os.Chtimes(p, slightlyPast, slightlyPast); err != nil {
			t.Fatal(err)
		}
	}

	// Verify cache is valid before removing the directory.
	valid := routes.CacheValid(cacheFile, routesRb, routesDir)
	if !valid {
		t.Fatal("expected cache to be valid before directory removal")
	}

	// Remove config/routes/ entirely.
	if err := os.RemoveAll(routesDir); err != nil {
		t.Fatal(err)
	}

	// Cache should be invalid because flag file says the directory existed before.
	valid = routes.CacheValid(cacheFile, routesRb, routesDir)
	if valid {
		t.Error("expected cache to be invalid after config/routes/ was removed")
	}
}

func TestRun(t *testing.T) {
	dir := t.TempDir()
	mustWriteRoutesTestFile(t, filepath.Join(dir, "config", "routes.rb"), "# routes")

	restorePath := stubBundle(t, sampleRoutes)
	defer restorePath()

	out, err := routes.Run(context.Background(), dir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != sampleRoutes {
		t.Fatalf("output mismatch:\n%s", out)
	}

	// Run must not write a cache file.
	cacheFile := filepath.Join(dir, "tmp", "routes_cache.txt")
	if _, err := os.Stat(cacheFile); err == nil {
		t.Fatal("Run must not write cache file")
	}
}

func TestRefresh(t *testing.T) {
	dir := t.TempDir()
	mustWriteRoutesTestFile(t, filepath.Join(dir, "config", "routes.rb"), "# routes")

	restorePath := stubBundle(t, sampleRoutes)
	defer restorePath()

	out, err := routes.Refresh(context.Background(), dir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != sampleRoutes {
		t.Fatalf("output mismatch:\n%s", out)
	}

	// Refresh must write a cache file.
	cacheFile := filepath.Join(dir, "tmp", "routes_cache.txt")
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		t.Fatalf("expected cache file to be written: %v", err)
	}
	if string(data) != sampleRoutes {
		t.Fatalf("cache content mismatch:\n%s", data)
	}
}

func TestCacheReturnsFreshOutputWhenTmpDirCreationFails(t *testing.T) {
	dir := t.TempDir()
	mustWriteRoutesTestFile(t, filepath.Join(dir, "config", "routes.rb"), "# routes")

	tmpPath := filepath.Join(dir, "tmp")
	if err := os.WriteFile(tmpPath, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}

	restorePath := stubBundle(t, sampleRoutes)
	defer restorePath()

	stderr := captureStderr(t)

	out, err := routes.Cache(context.Background(), dir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != sampleRoutes {
		t.Fatalf("output mismatch:\n%s", out)
	}
	if !strings.Contains(stderr(), "Warning: could not create routes cache directory:") {
		t.Fatalf("expected tmp dir warning, got: %q", stderr())
	}
	if _, statErr := os.Stat(filepath.Join(dir, "tmp", "routes_cache.txt")); statErr == nil || (!os.IsNotExist(statErr) && !strings.Contains(statErr.Error(), "not a directory")) {
		t.Fatalf("expected no cache file, stat err = %v", statErr)
	}
}

func TestCacheReturnsFreshOutputWhenCacheFileWriteFails(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "tmp", "routes_cache.txt")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-2 * time.Second)
	if err := os.Chtimes(cacheDir, past, past); err != nil {
		t.Fatal(err)
	}

	mustWriteRoutesTestFile(t, filepath.Join(dir, "config", "routes.rb"), "# routes")

	restorePath := stubBundle(t, sampleRoutes)
	defer restorePath()

	stderr := captureStderr(t)

	out, err := routes.Cache(context.Background(), dir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != sampleRoutes {
		t.Fatalf("output mismatch:\n%s", out)
	}
	if !strings.Contains(stderr(), "Warning: could not write routes cache:") {
		t.Fatalf("expected cache write warning, got: %q", stderr())
	}
	info, statErr := os.Stat(cacheDir)
	if statErr != nil {
		t.Fatalf("stat %s: %v", cacheDir, statErr)
	}
	if !info.IsDir() {
		t.Fatalf("expected %s to remain a directory", cacheDir)
	}
}

func TestFilterSkipsBootNoise(t *testing.T) {
	input := "[DEPRECATION WARNING] Something something\nLoading environment...\n" + sampleRoutes
	out, err := routes.Filter(input, []string{"users"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lines := strings.SplitN(out, "\n", 2)
	header := lines[0]
	if !strings.Contains(header, "Prefix") || !strings.Contains(header, "Verb") || !strings.Contains(header, "URI Pattern") {
		t.Errorf("header should be the routes header line, got: %q", header)
	}
	if strings.Contains(header, "DEPRECATION") || strings.Contains(header, "Loading") {
		t.Errorf("header must not be a boot noise line, got: %q", header)
	}
}

func TestFilter(t *testing.T) {
	t.Run("matching pattern", func(t *testing.T) {
		out, err := routes.Filter(sampleRoutes, []string{"users"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, "users#index") {
			t.Error("missing users#index")
		}
		if strings.Contains(out, "home#index") {
			t.Error("should not include home#index")
		}
	})

	t.Run("no match", func(t *testing.T) {
		_, err := routes.Filter(sampleRoutes, []string{"posts"})
		if err == nil {
			t.Error("expected error for no match")
		}
	})

	t.Run("multiple patterns", func(t *testing.T) {
		out, err := routes.Filter(sampleRoutes, []string{"new_user", "edit_user"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out, "new_user") {
			t.Error("missing new_user")
		}
		if !strings.Contains(out, "edit_user") {
			t.Error("missing edit_user")
		}
	})
}

func TestParseTable(t *testing.T) {
	entries, err := routes.ParseTable(sampleRoutesWithNoise)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 5 {
		t.Fatalf("expected 5 entries, got %d: %#v", len(entries), entries)
	}

	if entries[0].Prefix != "root" || entries[0].Verb != "GET" || entries[0].URIPattern != "/" || entries[0].ControllerAction != "home#index" {
		t.Fatalf("unexpected first entry: %#v", entries[0])
	}

	if entries[1].Prefix != "rails_info" || entries[1].ControllerAction != "rails/info#properties" {
		t.Fatalf("unexpected second entry: %#v", entries[1])
	}

	if entries[2].Prefix != "" || entries[2].Verb != "PATCH/PUT" || entries[2].URIPattern != "/users/:id" || entries[2].ControllerAction != "users#update" {
		t.Fatalf("unexpected blank-prefix entry: %#v", entries[2])
	}

	if entries[4].Prefix != "" || entries[4].Verb != "GET" || entries[4].ControllerAction != "users#show" {
		t.Fatalf("unexpected trailing blank-prefix entry: %#v", entries[4])
	}
}

func TestParseTableReturnsErrorWithoutHeader(t *testing.T) {
	_, err := routes.ParseTable("Loading app...\nno tabular routes here\n")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "standard tabular format") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func mustWriteRoutesTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func stubBundle(t *testing.T, output string) func() {
	t.Helper()
	binDir := t.TempDir()
	bundlePath := filepath.Join(binDir, "bundle")
	script := "#!/bin/sh\nprintf '%s' " + shellQuote(output) + "\n"
	if err := os.WriteFile(bundlePath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	prevPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", binDir+string(os.PathListSeparator)+prevPath); err != nil {
		t.Fatal(err)
	}

	return func() {
		_ = os.Setenv("PATH", prevPath)
	}
}

func captureStderr(t *testing.T) func() string {
	t.Helper()
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	t.Cleanup(func() {
		_ = w.Close()
		os.Stderr = oldStderr
	})

	return func() string {
		_ = w.Close()
		data, err := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}
