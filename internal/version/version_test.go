package version

import "testing"

func TestInfoPrefersExplicitValues(t *testing.T) {
	oldVersion, oldCommit, oldBuildDate := Version, Commit, BuildDate
	Version, Commit, BuildDate = "1.2.3", "abc1234", "2026-03-09T10:00:00Z"
	t.Cleanup(func() {
		Version, Commit, BuildDate = oldVersion, oldCommit, oldBuildDate
	})

	version, commit, buildDate := Info()
	if version != "1.2.3" {
		t.Fatalf("version = %q, want 1.2.3", version)
	}
	if commit != "abc1234" {
		t.Fatalf("commit = %q, want abc1234", commit)
	}
	if buildDate != "2026-03-09T10:00:00Z" {
		t.Fatalf("buildDate = %q, want 2026-03-09T10:00:00Z", buildDate)
	}
}
