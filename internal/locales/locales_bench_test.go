package locales_test

import (
	"os"
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
