package ui

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/chrismo/crux/internal/graph"
	"github.com/chrismo/crux/internal/rwx"
)

var updateGolden = flag.Bool("update", false, "update golden files")

// TestMain forces the no-color (Ascii) profile so rendered output is
// deterministic across environments (local TTY, CI, pipes) — the golden files
// below capture plain text, which is exactly the --print parity surface.
func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.Ascii)
	os.Exit(m.Run())
}

func goldenCheck(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name)
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s (regenerate with `go test ./internal/ui -update`): %v", name, err)
	}
	if got != string(want) {
		t.Errorf("%s mismatch — run `go test ./internal/ui -update` if intended.\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func loadRun(t *testing.T, fixture string) rwx.Run {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "rwx", "testdata", fixture))
	if err != nil {
		t.Fatalf("read %s: %v", fixture, err)
	}
	var run rwx.Run
	if err := json.Unmarshal(data, &run); err != nil {
		t.Fatalf("unmarshal %s: %v", fixture, err)
	}
	return run
}

// These goldens pin the --print body output. Any intentional change to a pure
// renderer must be accompanied by `-update`; an unintentional one fails here.
func TestGoldenScreenSucceeded(t *testing.T) {
	run := loadRun(t, "run_succeeded.json")
	g := graph.Build(run)
	goldenCheck(t, "screen_succeeded.txt", Screen(run, g, graph.Layout(g), 0, graphOverlay{}))
}

func TestGoldenScreenFailed(t *testing.T) {
	run := loadRun(t, "run_failed.json")
	g := graph.Build(run)
	goldenCheck(t, "screen_failed.txt", Screen(run, g, graph.Layout(g), 0, graphOverlay{}))
}

// The sample-DAG goldens exercise the richly-connected mock build: connectors,
// critical path, and (in the failed variant) blast radius across many layers.
func TestGoldenSampleDagSucceeded(t *testing.T) {
	run := loadRun(t, "sample_dag_succeeded.json")
	g := graph.Build(run)
	goldenCheck(t, "sample_dag_succeeded.txt", Screen(run, g, graph.Layout(g), 0, graphOverlay{}))
}

func TestGoldenSampleDagFailed(t *testing.T) {
	run := loadRun(t, "sample_dag_failed.json")
	g := graph.Build(run)
	goldenCheck(t, "sample_dag_failed.txt", Screen(run, g, graph.Layout(g), 0, graphOverlay{}))
}

// Filter collapses the graph to matching nodes with path-preserving
// connectors. "g" keeps a cross-layer slice (go-deps, proto-gen, lint-go,
// integration, ...) whose intermediate build-* nodes are hidden, so the
// go-deps/proto-gen -> integration links render as dashed collapsed edges.
func TestGoldenSampleDagFiltered(t *testing.T) {
	run := loadRun(t, "sample_dag_failed.json")
	g := graph.Build(run)
	goldenCheck(t, "sample_dag_filter_g.txt", Screen(run, g, graph.Layout(g), 0, graphOverlay{Filter: "g"}))
}

// Pinning an anchor collapses to its focus cone and marks the anchor with 📌.
func TestGoldenSampleDagPinned(t *testing.T) {
	run := loadRun(t, "sample_dag_failed.json")
	g := graph.Build(run)
	ov := graphOverlay{Focus: graph.Focus(g, "go-deps"), Pinned: map[string]bool{"go-deps": true}}
	goldenCheck(t, "sample_dag_pinned.txt", Screen(run, g, graph.Layout(g), 0, ov))
}

func TestGoldenHome(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "rwx", "testdata", "runs_list.json"))
	if err != nil {
		t.Fatalf("read runs_list.json: %v", err)
	}
	var rl rwx.RunList
	if err := json.Unmarshal(data, &rl); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	now := time.Date(2026, 6, 30, 21, 0, 0, 0, time.UTC)
	goldenCheck(t, "home.txt", HomeView(rl.Runs, 0, now, ""))
}
