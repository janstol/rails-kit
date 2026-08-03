package concerns_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/janstol/rails-kit/internal/concerns"
)

// BenchmarkParse measures Parse end to end, including the per-call Prism
// parser construction. Unlike a long-lived CLI process, this benchmark pays
// the one-time WASM-compile cold start on every iteration, since Parse
// constructs and closes its own *prism.Parser rather than reusing one across
// calls -- so ns/op here reflects cold-start cost, not steady-state parse
// throughput.
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
