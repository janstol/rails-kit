package cmd

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// Opt-in, delta-based startup assertion.
//
// go test produces no binary of its own, so this test is gated on
// RAILS_KIT_BENCH_BIN holding a path to a prebuilt rails-kit binary (see
// `just bench`). Left unset, it skips — keeping `go test ./...` and
// `go test -race ./...` green and non-flaky on shared CI runners.
//
// `version` does essentially no work; its wall time is almost entirely
// process startup (exec + dyld + Go runtime init). Measuring other commands'
// wall time *relative to that floor* isolates real work from startup noise,
// which a flat threshold cannot do.
const (
	startupRuns    = 20
	startupDelta   = 5 * time.Millisecond
	startupCeiling = 25 * time.Millisecond
)

func TestStartupBudget(t *testing.T) {
	bin := os.Getenv("RAILS_KIT_BENCH_BIN")
	if bin == "" {
		t.Skip("RAILS_KIT_BENCH_BIN not set; skipping opt-in startup budget check (see `just bench`)")
	}

	fixtureRoot, err := filepath.Abs("../testdata")
	if err != nil {
		t.Fatal(err)
	}

	versionFloor := medianRunTime(t, bin, []string{"version"})
	t.Logf("version (startup floor): median=%s over %d runs", versionFloor, startupRuns)

	cases := []struct {
		name string
		args []string
	}{
		{name: "schema", args: []string{"--root", fixtureRoot, "schema"}},
		{name: "about", args: []string{"--root", fixtureRoot, "about"}},
		{name: "routes --static", args: []string{"--root", fixtureRoot, "routes", "--static"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			median := medianRunTime(t, bin, tc.args)
			delta := median - versionFloor
			t.Logf("%s: median=%s delta-over-floor=%s", tc.name, median, delta)

			if median > startupCeiling {
				t.Errorf("%s: median wall time %s exceeds absolute ceiling %s", tc.name, median, startupCeiling)
			}
			if delta > startupDelta {
				t.Errorf("%s: delta over version floor %s exceeds budget %s", tc.name, delta, startupDelta)
			}
		})
	}
}

// medianRunTime runs bin with args startupRuns times and returns the median
// wall time. Median rather than mean, because process startup has a long
// right tail (scheduler jitter, page faults) that a mean would overweight.
func medianRunTime(t *testing.T, bin string, args []string) time.Duration {
	t.Helper()

	durations := make([]time.Duration, 0, startupRuns)
	for range startupRuns {
		cmd := exec.Command(bin, args...)
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		start := time.Now()
		if err := cmd.Run(); err != nil {
			t.Fatalf("running %s %v: %v", bin, args, err)
		}
		durations = append(durations, time.Since(start))
	}

	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	return durations[len(durations)/2]
}
