package schema_test

import (
	"os"
	"testing"

	"github.com/janstol/rails-kit/internal/schema"
)

func BenchmarkParse(b *testing.B) {
	if _, err := os.Stat(testdataSchema); err != nil {
		b.Skip("testdata schema.rb not found")
	}

	b.ReportAllocs()
	for b.Loop() {
		if _, err := schema.Parse(testdataSchema); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkListTables(b *testing.B) {
	if _, err := os.Stat(testdataSchema); err != nil {
		b.Skip("testdata schema.rb not found")
	}

	b.ReportAllocs()
	for b.Loop() {
		if _, err := schema.ListTables(testdataSchema); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkExtractTables(b *testing.B) {
	if _, err := os.Stat(testdataSchema); err != nil {
		b.Skip("testdata schema.rb not found")
	}
	names := []string{"users", "posts"}

	b.ReportAllocs()
	for b.Loop() {
		if _, err := schema.ExtractTables(testdataSchema, names); err != nil {
			b.Fatal(err)
		}
	}
}
