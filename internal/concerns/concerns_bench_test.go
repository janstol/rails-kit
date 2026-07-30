package concerns_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/janstol/rails-kit/internal/concerns"
)

func BenchmarkParse(b *testing.B) {
	path := filepath.Join(testdataRoot, "app/models/concerns/searchable.rb")
	if _, err := os.Stat(path); err != nil {
		b.Skip("testdata/app/models/concerns/searchable.rb not found")
	}

	b.ReportAllocs()
	for b.Loop() {
		if _, err := concerns.Parse(path, "app/models/concerns/searchable.rb", "model"); err != nil {
			b.Fatal(err)
		}
	}
}
