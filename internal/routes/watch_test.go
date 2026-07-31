package routes_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/janstol/rails-kit/internal/routes"
)

func TestFingerprintStableWithoutChanges(t *testing.T) {
	dir := t.TempDir()
	routesRb := filepath.Join(dir, "config", "routes.rb")
	routesDir := filepath.Join(dir, "config", "routes")
	if err := os.MkdirAll(routesDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(routesRb, []byte("# routes"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(routesDir, "admin.rb"), []byte("# admin"), 0644); err != nil {
		t.Fatal(err)
	}

	fp1 := routes.Fingerprint(routesRb, routesDir)
	fp2 := routes.Fingerprint(routesRb, routesDir)
	if fp1 != fp2 {
		t.Fatalf("fingerprint changed without modification: %q != %q", fp1, fp2)
	}
}

func TestFingerprintChangesOnModify(t *testing.T) {
	dir := t.TempDir()
	routesRb := filepath.Join(dir, "config", "routes.rb")
	routesDir := filepath.Join(dir, "config", "routes")
	if err := os.MkdirAll(routesDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(routesRb, []byte("# routes"), 0644); err != nil {
		t.Fatal(err)
	}

	before := routes.Fingerprint(routesRb, routesDir)

	newTime := time.Now().Add(1 * time.Hour)
	if err := os.Chtimes(routesRb, newTime, newTime); err != nil {
		t.Fatal(err)
	}

	after := routes.Fingerprint(routesRb, routesDir)
	if before == after {
		t.Fatal("expected fingerprint to change after routes.rb mtime change")
	}
}

func TestFingerprintChangesOnAdd(t *testing.T) {
	dir := t.TempDir()
	routesRb := filepath.Join(dir, "config", "routes.rb")
	routesDir := filepath.Join(dir, "config", "routes")
	if err := os.MkdirAll(routesDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(routesRb, []byte("# routes"), 0644); err != nil {
		t.Fatal(err)
	}

	before := routes.Fingerprint(routesRb, routesDir)

	if err := os.WriteFile(filepath.Join(routesDir, "admin.rb"), []byte("# admin"), 0644); err != nil {
		t.Fatal(err)
	}

	after := routes.Fingerprint(routesRb, routesDir)
	if before == after {
		t.Fatal("expected fingerprint to change after adding a route file")
	}
}

func TestFingerprintChangesOnDelete(t *testing.T) {
	dir := t.TempDir()
	routesRb := filepath.Join(dir, "config", "routes.rb")
	routesDir := filepath.Join(dir, "config", "routes")
	if err := os.MkdirAll(routesDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(routesRb, []byte("# routes"), 0644); err != nil {
		t.Fatal(err)
	}
	adminFile := filepath.Join(routesDir, "admin.rb")
	if err := os.WriteFile(adminFile, []byte("# admin"), 0644); err != nil {
		t.Fatal(err)
	}

	before := routes.Fingerprint(routesRb, routesDir)

	if err := os.Remove(adminFile); err != nil {
		t.Fatal(err)
	}

	after := routes.Fingerprint(routesRb, routesDir)
	if before == after {
		t.Fatal("expected fingerprint to change after deleting a route file")
	}
}

func TestFingerprintChangesOnNewSubdir(t *testing.T) {
	dir := t.TempDir()
	routesRb := filepath.Join(dir, "config", "routes.rb")
	routesDir := filepath.Join(dir, "config", "routes")
	if err := os.MkdirAll(routesDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(routesRb, []byte("# routes"), 0644); err != nil {
		t.Fatal(err)
	}

	before := routes.Fingerprint(routesRb, routesDir)

	if err := os.MkdirAll(filepath.Join(routesDir, "admin"), 0755); err != nil {
		t.Fatal(err)
	}

	after := routes.Fingerprint(routesRb, routesDir)
	if before == after {
		t.Fatal("expected fingerprint to change after adding a new subdirectory")
	}
}

func TestFingerprintWorksWithoutRoutesDir(t *testing.T) {
	dir := t.TempDir()
	routesRb := filepath.Join(dir, "config", "routes.rb")
	if err := os.MkdirAll(filepath.Dir(routesRb), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(routesRb, []byte("# routes"), 0644); err != nil {
		t.Fatal(err)
	}

	routesDir := filepath.Join(dir, "config", "routes")
	fp1 := routes.Fingerprint(routesRb, routesDir)
	fp2 := routes.Fingerprint(routesRb, routesDir)
	if fp1 != fp2 {
		t.Fatal("expected stable fingerprint when config/routes/ is absent")
	}

	if err := os.MkdirAll(routesDir, 0755); err != nil {
		t.Fatal(err)
	}
	fp3 := routes.Fingerprint(routesRb, routesDir)
	if fp1 == fp3 {
		t.Fatal("expected fingerprint to change when config/routes/ appears")
	}
}

// triggerRender changes routesRb's mtime repeatedly (each time to a distinct,
// strictly-increasing value) until a render is observed on renderCh or the
// retry budget is exhausted. This avoids a race against Watch's internal
// goroutine capturing its baseline fingerprint.
func triggerRender(t *testing.T, routesRb string, renderCh <-chan struct{}) bool {
	t.Helper()
	for i := 1; i <= 100; i++ {
		newTime := time.Now().Add(time.Duration(i) * time.Hour)
		if err := os.Chtimes(routesRb, newTime, newTime); err != nil {
			t.Fatal(err)
		}
		select {
		case <-renderCh:
			return true
		case <-time.After(20 * time.Millisecond):
		}
	}
	return false
}

func TestWatchRendersOnChange(t *testing.T) {
	dir := t.TempDir()
	routesRb := filepath.Join(dir, "config", "routes.rb")
	routesDir := filepath.Join(dir, "config", "routes")
	if err := os.MkdirAll(routesDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(routesRb, []byte("# routes"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	renderCh := make(chan struct{}, 100)
	render := func() error {
		select {
		case renderCh <- struct{}{}:
		default:
		}
		return nil
	}

	watchDone := make(chan error, 1)
	go func() {
		watchDone <- routes.Watch(ctx, routesRb, routesDir, 5*time.Millisecond, render, nil)
	}()

	if !triggerRender(t, routesRb, renderCh) {
		t.Fatal("expected Watch to render after routes.rb changed")
	}

	cancel()
	select {
	case err := <-watchDone:
		if err != nil {
			t.Fatalf("Watch returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Watch did not return promptly after context cancellation")
	}
}

func TestWatchReturnsPromptlyOnContextCancel(t *testing.T) {
	dir := t.TempDir()
	routesRb := filepath.Join(dir, "config", "routes.rb")
	routesDir := filepath.Join(dir, "config", "routes")
	if err := os.MkdirAll(filepath.Dir(routesRb), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(routesRb, []byte("# routes"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	watchDone := make(chan error, 1)
	go func() {
		watchDone <- routes.Watch(ctx, routesRb, routesDir, time.Second, func() error { return nil }, nil)
	}()

	cancel()

	select {
	case err := <-watchDone:
		if err != nil {
			t.Fatalf("Watch returned error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Watch did not return promptly after context cancellation")
	}
}

func TestWatchKeepsPollingAfterRenderError(t *testing.T) {
	dir := t.TempDir()
	routesRb := filepath.Join(dir, "config", "routes.rb")
	routesDir := filepath.Join(dir, "config", "routes")
	if err := os.MkdirAll(routesDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(routesRb, []byte("# routes"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	var renderCount, errCount int
	renderCh := make(chan struct{}, 100)
	render := func() error {
		mu.Lock()
		renderCount++
		mu.Unlock()
		select {
		case renderCh <- struct{}{}:
		default:
		}
		return fmt.Errorf("boom")
	}
	onErr := func(error) {
		mu.Lock()
		errCount++
		mu.Unlock()
	}

	watchDone := make(chan error, 1)
	go func() {
		watchDone <- routes.Watch(ctx, routesRb, routesDir, 5*time.Millisecond, render, onErr)
	}()

	// Two separate changes should each produce a render (and onErr call)
	// despite render always returning an error.
	for i := 0; i < 2; i++ {
		if !triggerRender(t, routesRb, renderCh) {
			t.Fatalf("expected render #%d after change", i+1)
		}
	}

	cancel()
	select {
	case err := <-watchDone:
		if err != nil {
			t.Fatalf("Watch returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Watch did not return promptly after context cancellation")
	}

	mu.Lock()
	defer mu.Unlock()
	if renderCount < 2 {
		t.Fatalf("expected at least 2 renders despite errors, got %d", renderCount)
	}
	if errCount < 2 {
		t.Fatalf("expected onErr called at least twice, got %d", errCount)
	}
}
