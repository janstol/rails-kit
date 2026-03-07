package railsroot_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/janstol/rails-kit/internal/railsroot"
)

func TestFindFrom(t *testing.T) {
	t.Run("finds root at given dir", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "config"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "config", "application.rb"), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
		got, err := railsroot.FindFrom(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != dir {
			t.Errorf("got %q, want %q", got, dir)
		}
	})

	t.Run("finds root from subdirectory", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "config"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "config", "application.rb"), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
		sub := filepath.Join(dir, "app", "models")
		if err := os.MkdirAll(sub, 0755); err != nil {
			t.Fatal(err)
		}
		got, err := railsroot.FindFrom(sub)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != dir {
			t.Errorf("got %q, want %q", got, dir)
		}
	})

	t.Run("returns error when not found", func(t *testing.T) {
		dir := t.TempDir()
		_, err := railsroot.FindFrom(dir)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}
