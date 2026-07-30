package related_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/janstol/rails-kit/internal/related"
)

func BenchmarkFind(b *testing.B) {
	if _, err := os.Stat(testdataRoot); err != nil {
		b.Skip("testdata root not found")
	}

	b.ReportAllocs()
	for b.Loop() {
		if _, err := related.Find(testdataRoot, defaultRelatedConfig(), "user", "users"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWalkMatchSegment(b *testing.B) {
	dir := filepath.Join(testdataRoot, "app")
	if _, err := os.Stat(dir); err != nil {
		b.Skip("testdata app dir not found")
	}

	b.ReportAllocs()
	for b.Loop() {
		if _, err := related.WalkMatchSegment(dir, "user"); err != nil {
			b.Fatal(err)
		}
	}
}
