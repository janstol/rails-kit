package gem

import (
	"os"
	"testing"
)

func BenchmarkParse(b *testing.B) {
	if _, err := os.Stat(testdataLockfile); err != nil {
		b.Skip("testdata Gemfile.lock not found")
	}

	b.ReportAllocs()
	for b.Loop() {
		if _, err := Parse(testdataLockfile); err != nil {
			b.Fatal(err)
		}
	}
}
