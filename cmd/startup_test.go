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

// `routes --static` and `model` parse via the Prism AST (go-ruby-prism, WASM via wazero).
// Each process pays a one-time ~100-150 ms WASM-compile cold start on first
// parse. For routes this buys an 8-17x per-parse throughput win; for model it
// buys structurally reliable Ruby parsing in place of the regex line scanner.
// That cold start dominates the per-process wall time this test measures
// (every `cmd.Run` is a fresh process), so the tight schema/about budget does
// not fit. The Prism budget is sized to accommodate the cold start with
// headroom for slower runners and jitter; it is a coarse "did startup blow up
// to multiple seconds" guard, not a fine-grained regression detector — that
// lives in the parser benchmarks' ns/op.
const (
	prismStartupDelta   = 400 * time.Millisecond
	prismStartupCeiling = 500 * time.Millisecond
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
		name    string
		args    []string
		ceiling time.Duration
		delta   time.Duration
	}{
		{name: "schema", args: []string{"--root", fixtureRoot, "schema"}, ceiling: startupCeiling, delta: startupDelta},
		{name: "about", args: []string{"--root", fixtureRoot, "about"}, ceiling: startupCeiling, delta: startupDelta},
		// concerns (no arg) only lists filenames -- it must never construct a
		// Prism parser, so it stays on the tight budget alongside schema/about.
		{name: "concerns", args: []string{"--root", fixtureRoot, "concerns"}, ceiling: startupCeiling, delta: startupDelta},
		{name: "routes --static", args: []string{"--root", fixtureRoot, "routes", "--static"}, ceiling: prismStartupCeiling, delta: prismStartupDelta},
		{name: "model", args: []string{"--root", fixtureRoot, "model", "user"}, ceiling: prismStartupCeiling, delta: prismStartupDelta},
		{name: "concerns searchable", args: []string{"--root", fixtureRoot, "concerns", "searchable"}, ceiling: prismStartupCeiling, delta: prismStartupDelta},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			median := medianRunTime(t, bin, tc.args)
			delta := median - versionFloor
			t.Logf("%s: median=%s delta-over-floor=%s", tc.name, median, delta)

			if median > tc.ceiling {
				t.Errorf("%s: median wall time %s exceeds absolute ceiling %s", tc.name, median, tc.ceiling)
			}
			if delta > tc.delta {
				t.Errorf("%s: delta over version floor %s exceeds budget %s", tc.name, delta, tc.delta)
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
