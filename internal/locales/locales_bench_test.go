package locales_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/janstol/rails-kit/internal/locales"
)

func BenchmarkLoad(b *testing.B) {
	if _, err := os.Stat(testdataLocales); err != nil {
		b.Skip("testdata locales dir not found")
	}

	b.ReportAllocs()
	for b.Loop() {
		if _, err := locales.Load(testdataLocales); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLoadManyFiles measures Load against a synthetic locales directory
// with ~100 YAML files, since testdata's 3 real locale files are too few to
// show the fan-out win from parallelizing the read+unmarshal phase.
func BenchmarkLoadManyFiles(b *testing.B) {
	dir := writeManyLocaleFiles(b, 100)

	b.ReportAllocs()
	for b.Loop() {
		if _, err := locales.Load(dir); err != nil {
			b.Fatal(err)
		}
	}
}

func writeManyLocaleFiles(b *testing.B, fileCount int) string {
	b.Helper()
	dir := b.TempDir()
	for i := range fileCount {
		content := fmt.Sprintf("en:\n  file_%d:\n    greeting: \"hello %d\"\n    farewell: \"bye %d\"\n", i, i, i)
		path := filepath.Join(dir, fmt.Sprintf("locale_%03d.yml", i))
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			b.Fatal(err)
		}
	}
	return dir
}
