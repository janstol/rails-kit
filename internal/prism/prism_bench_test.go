package prism_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/janstol/rails-kit/internal/prism"
)

const benchTestdataApp = "../../testdata/app"

var benchRunner = prism.Runner{}

// benchParseFilesPaths returns n paths built by cycling the real fixtures
// under testdata/app, so the benchmark exercises real Ruby source rather
// than synthetic files, at whatever batch size n requires.
func benchParseFilesPaths(b *testing.B, n int) []string {
	b.Helper()
	var fixtures []string
	if err := filepath.WalkDir(benchTestdataApp, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".rb") {
			fixtures = append(fixtures, path)
		}
		return nil
	}); err != nil {
		b.Fatalf("walking %s: %v", benchTestdataApp, err)
	}
	if len(fixtures) == 0 {
		b.Skip("testdata/app fixtures not found")
	}

	paths := make([]string, n)
	for i := range paths {
		paths[i] = fixtures[i%len(fixtures)]
	}
	return paths
}

func BenchmarkParseFiles1(b *testing.B) {
	paths := benchParseFilesPaths(b, 1)

	b.ReportAllocs()
	for b.Loop() {
		if _, err := benchRunner.ParseFiles(context.Background(), paths); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseFiles8(b *testing.B) {
	paths := benchParseFilesPaths(b, 8)

	b.ReportAllocs()
	for b.Loop() {
		if _, err := benchRunner.ParseFiles(context.Background(), paths); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseFiles50(b *testing.B) {
	paths := benchParseFilesPaths(b, 50)

	b.ReportAllocs()
	for b.Loop() {
		if _, err := benchRunner.ParseFiles(context.Background(), paths); err != nil {
			b.Fatal(err)
		}
	}
}
