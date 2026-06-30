package ui

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/chrismo/rwx-tui/internal/graph"
	"github.com/chrismo/rwx-tui/internal/rwx"
)

func fixtureGraph(t *testing.T) (*graph.Graph, *graph.LayoutData) {
	t.Helper()
	data, err := os.ReadFile("../rwx/testdata/run_succeeded.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var run rwx.Run
	if err := json.Unmarshal(data, &run); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	g := graph.Build(run)
	return g, graph.Layout(g)
}

func TestRenderGraphContainsAllNodes(t *testing.T) {
	g, l := fixtureGraph(t)
	out := RenderGraph(g, l, RenderOpts{})

	for _, key := range []string{"code", "go", "deps", "vet", "test", "build", "~base-image", "~base-config"} {
		if !strings.Contains(out, key) {
			t.Errorf("render missing node %q", key)
		}
	}
}

func TestRenderGraphOrdersLayersTopDown(t *testing.T) {
	g, l := fixtureGraph(t)
	out := RenderGraph(g, l, RenderOpts{})

	// A layer-0 root must render above a layer-2 leaf.
	iCode := strings.Index(out, "code")
	iVet := strings.Index(out, "vet")
	if iCode < 0 || iVet < 0 {
		t.Fatal("expected both code and vet in output")
	}
	if iCode > iVet {
		t.Errorf("code (layer 0) should render before vet (layer 2)")
	}
}

func TestRenderGraphUsesStateGlyphs(t *testing.T) {
	g, l := fixtureGraph(t)
	out := RenderGraph(g, l, RenderOpts{})

	if !strings.Contains(out, glyphFor(rwx.StateRan)) {
		t.Errorf("expected the 'ran' glyph %q in output", glyphFor(rwx.StateRan))
	}
	// This run had no failures, so the failed glyph must not appear.
	if strings.Contains(out, glyphFor(rwx.StateFailed)) {
		t.Errorf("unexpected 'failed' glyph in a fully-succeeded run")
	}
}

func TestRenderGraphMarksCriticalPath(t *testing.T) {
	g, l := fixtureGraph(t)
	cp := graph.CriticalPath(g)

	// Without a critical path, every box uses the rounded border.
	if strings.Contains(RenderGraph(g, l, RenderOpts{}), "┏") {
		t.Error("did not expect a thick border without a critical path")
	}
	// With one, critical nodes switch to a thick border.
	if !strings.Contains(RenderGraph(g, l, RenderOpts{Crit: cp}), "┏") {
		t.Error("expected a thick border for critical-path nodes")
	}
}

func failedGraph(t *testing.T) (*graph.Graph, *graph.LayoutData) {
	t.Helper()
	data, err := os.ReadFile("../rwx/testdata/run_failed.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var run rwx.Run
	if err := json.Unmarshal(data, &run); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	g := graph.Build(run)
	return g, graph.Layout(g)
}

func TestRenderGraphMarksBlastRadius(t *testing.T) {
	g, l := failedGraph(t)
	fi := graph.AnalyzeFailures(g)
	out := RenderGraph(g, l, RenderOpts{Failure: fi})

	if !strings.Contains(out, "↯") {
		t.Error("expected the blast-radius marker ↯ in a failed run")
	}
	if !strings.Contains(out, glyphFor(rwx.StateFailed)) {
		t.Error("expected the failed glyph for the failed node")
	}
}

func TestFailureLine(t *testing.T) {
	g, _ := failedGraph(t)
	got := FailureLine(graph.AnalyzeFailures(g))
	want := "failed: code · blast radius: build, deps, test, vet"
	if got != want {
		t.Errorf("FailureLine() = %q, want %q", got, want)
	}

	// A succeeded run produces no failure line.
	gs, _ := fixtureGraph(t)
	if got := FailureLine(graph.AnalyzeFailures(gs)); got != "" {
		t.Errorf("FailureLine() on success = %q, want empty", got)
	}
}

func TestCriticalPathLine(t *testing.T) {
	g, _ := fixtureGraph(t)
	got := CriticalPathLine(graph.CriticalPath(g))
	want := "critical path: code → deps → test · 20s"
	if got != want {
		t.Errorf("CriticalPathLine() = %q, want %q", got, want)
	}
}
