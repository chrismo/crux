package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textinput"

	"github.com/chrismo/crux/internal/graph"
)

func fixtureApp(t *testing.T, filter string) *App {
	t.Helper()
	run := loadRun(t, "sample_dag_failed.json")
	g := graph.Build(run)
	a := &App{graph: g, layout: graph.Layout(g), filterInput: textinput.New()}
	if filter != "" {
		a.filterInput.SetValue(filter)
	}
	return a
}

// Under an active filter the cursor must only ever land on visible nodes, no
// matter which direction it is moved.
func TestMoveSelectionStaysVisibleUnderFilter(t *testing.T) {
	a := fixtureApp(t, "g")
	visible := computeVisible(a.graph, a.currentOverlay())
	a.clampSelectionToVisible()
	if a.selectedNode == "" {
		t.Fatal("no selection after clamp")
	}
	for _, mv := range [][2]int{{1, 0}, {1, 0}, {0, 1}, {0, 1}, {-1, 0}, {0, -1}, {1, 0}, {0, 1}} {
		a.moveSelection(mv[0], mv[1])
		if a.selectedNode != "" && !visible[a.selectedNode] {
			t.Fatalf("moveSelection(%d,%d) landed on hidden node %q", mv[0], mv[1], a.selectedNode)
		}
	}
}

// Applying a filter that hides the current selection snaps it to a visible node.
func TestClampSelectionResetsHidden(t *testing.T) {
	a := fixtureApp(t, "g")
	a.selectedNode = "build-api" // has no 'g' → hidden under the filter
	a.clampSelectionToVisible()
	visible := computeVisible(a.graph, a.currentOverlay())
	if !visible[a.selectedNode] {
		t.Fatalf("clamp left selection on hidden node %q", a.selectedNode)
	}
}

// Without a filter, movement walks the full layout (down changes layer).
func TestMoveSelectionUnfiltered(t *testing.T) {
	a := fixtureApp(t, "")
	a.selectedNode = firstNode(a.layout)
	start := a.selectedNode
	a.moveSelection(1, 0)
	if a.selectedNode == start {
		t.Errorf("expected selection to move to a deeper layer, stayed on %q", start)
	}
	if _, ok := a.layout.Pos[a.selectedNode]; !ok {
		t.Errorf("selection %q not in full layout", a.selectedNode)
	}
}

func TestNodeColumn(t *testing.T) {
	line := "│ ✗ go-deps (1s) │   │ ⚡ proto-gen (3s) │"
	goCol := nodeColumn(line, "go-deps")
	protoCol := nodeColumn(line, "proto-gen")
	if goCol < 0 || protoCol < 0 {
		t.Fatalf("expected both labels found, got go=%d proto=%d", goCol, protoCol)
	}
	if goCol >= protoCol {
		t.Errorf("go-deps (%d) should be left of proto-gen (%d)", goCol, protoCol)
	}
	if got := nodeColumn(line, "absent"); got != -1 {
		t.Errorf("nodeColumn(absent) = %d, want -1", got)
	}
}
