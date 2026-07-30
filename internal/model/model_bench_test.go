package model_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/janstol/rails-kit/internal/model"
)

func BenchmarkParse(b *testing.B) {
	path := testdataRoot + "/app/models/user.rb"
	if _, err := os.Stat(path); err != nil {
		b.Skip("testdata/app/models/user.rb not found")
	}

	b.ReportAllocs()
	for b.Loop() {
		if _, err := model.Parse(path, testdataRoot, "app/models"); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParseLarge measures Parse against a synthetic model with several
// hundred macro lines (associations, validations, scopes, callbacks), since
// the fixture app's user.rb is too small to reveal per-call overhead that
// only shows up at real-app scale.
func BenchmarkParseLarge(b *testing.B) {
	root, relPath := writeLargeModelFile(b, 300)

	b.ReportAllocs()
	for b.Loop() {
		if _, err := model.Parse(filepath.Join(root, relPath), root, "app/models"); err != nil {
			b.Fatal(err)
		}
	}
}

func writeLargeModelFile(b *testing.B, macroCount int) (root, relPath string) {
	b.Helper()
	var sb strings.Builder
	sb.WriteString("class BigModel < ApplicationRecord\n")
	sb.WriteString("  include Searchable\n\n")
	for i := range macroCount {
		fmt.Fprintf(&sb, "  has_many :assoc_%d, dependent: :destroy\n", i)
		fmt.Fprintf(&sb, "  validates :field_%d, presence: true, uniqueness: true\n", i)
		fmt.Fprintf(&sb, "  scope :scope_%d, -> { where(active: true) }\n", i)
		fmt.Fprintf(&sb, "  before_save :callback_%d\n", i)
	}
	sb.WriteString("end\n")

	root = b.TempDir()
	relPath = filepath.Join("app", "models", "big_model.rb")
	path := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		b.Fatal(err)
	}
	return root, relPath
}
