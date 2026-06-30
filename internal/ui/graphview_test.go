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
	out := RenderGraph(g, l)

	for _, key := range []string{"code", "go", "deps", "vet", "test", "build", "~base-image", "~base-config"} {
		if !strings.Contains(out, key) {
			t.Errorf("render missing node %q", key)
		}
	}
}

func TestRenderGraphOrdersLayersTopDown(t *testing.T) {
	g, l := fixtureGraph(t)
	out := RenderGraph(g, l)

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
	out := RenderGraph(g, l)

	if !strings.Contains(out, glyphFor(rwx.StateRan)) {
		t.Errorf("expected the 'ran' glyph %q in output", glyphFor(rwx.StateRan))
	}
	// This run had no failures, so the failed glyph must not appear.
	if strings.Contains(out, glyphFor(rwx.StateFailed)) {
		t.Errorf("unexpected 'failed' glyph in a fully-succeeded run")
	}
}
