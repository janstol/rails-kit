package routes_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/janstol/rails-kit/internal/pluralize"
	"github.com/janstol/rails-kit/internal/routes"
)

func BenchmarkParseStaticDetailed(b *testing.B) {
	path := filepath.Join("..", "..", "testdata", "config", "routes.rb")
	if _, err := os.Stat(path); err != nil {
		b.Skip("testdata/config/routes.rb not found")
	}
	p := pluralize.Default()

	b.ReportAllocs()
	for b.Loop() {
		if _, err := routes.ParseStaticDetailed(path, p); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParseStaticDetailedLarge measures ParseStaticDetailed against a
// synthetic routes.rb with several hundred resources, since the fixture app's
// 17-line routes.rb is too small to reveal per-call overhead that only shows
// up at real-app scale.
func BenchmarkParseStaticDetailedLarge(b *testing.B) {
	path := writeLargeRoutesFile(b, 300)
	p := pluralize.Default()

	b.ReportAllocs()
	for b.Loop() {
		if _, err := routes.ParseStaticDetailed(path, p); err != nil {
			b.Fatal(err)
		}
	}
}

func writeLargeRoutesFile(b *testing.B, resourceCount int) string {
	b.Helper()
	var sb strings.Builder
	sb.WriteString("Rails.application.routes.draw do\n")
	sb.WriteString("  root to: \"home#index\"\n\n")
	for i := range resourceCount {
		name := fmt.Sprintf("resource_%d", i)
		fmt.Fprintf(&sb, "  resources :%s do\n", name)
		sb.WriteString("    member do\n")
		sb.WriteString("      get :preview\n")
		sb.WriteString("    end\n")
		sb.WriteString("    collection do\n")
		sb.WriteString("      get :search\n")
		sb.WriteString("    end\n")
		sb.WriteString("  end\n\n")
	}
	sb.WriteString("end\n")

	dir := b.TempDir()
	path := filepath.Join(dir, "routes.rb")
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		b.Fatal(err)
	}
	return path
}
