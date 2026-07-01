package graph

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/chrismo/crux/internal/rwx"
)

func loadFixtureGraph(t *testing.T) *Graph {
	t.Helper()
	data, err := os.ReadFile("../rwx/testdata/run_succeeded.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var run rwx.Run
	if err := json.Unmarshal(data, &run); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return Build(run)
}

func TestLayoutLayering(t *testing.T) {
	l := Layout(loadFixtureGraph(t))

	wantLayer := map[string]int{
		"code": 0, "go": 0, "~base-image": 0,
		"deps": 1, "~base-config": 1,
		"vet": 2, "test": 2, "build": 2,
	}
	for key, want := range wantLayer {
		if got := l.Pos[key].Layer; got != want {
			t.Errorf("layer(%q) = %d, want %d", key, got, want)
		}
	}
}

func TestLayoutOrderingDeterministic(t *testing.T) {
	l := Layout(loadFixtureGraph(t))

	want := [][]string{
		{"code", "go", "~base-image"},   // roots, ordered by key
		{"deps", "~base-config"},        // deps bary 0.5, ~base-config bary 2
		{"build", "test", "vet"},        // tie on parent deps -> ordered by key
	}
	if len(l.Layers) != len(want) {
		t.Fatalf("layer count = %d, want %d", len(l.Layers), len(want))
	}
	for i, w := range want {
		if !equalStrings(l.Layers[i], w) {
			t.Errorf("layer %d = %v, want %v", i, l.Layers[i], w)
		}
	}

	// Order field must agree with the position within its layer.
	for _, layer := range l.Layers {
		for order, key := range layer {
			if l.Pos[key].Order != order {
				t.Errorf("Pos[%q].Order = %d, want %d", key, l.Pos[key].Order, order)
			}
		}
	}
}

// A diamond with a long edge (A spans two layers to D) must not panic and must
// place D below its deepest dependency.
func TestLayoutLongEdge(t *testing.T) {
	g := &Graph{index: map[string]*Node{}}
	for _, k := range []string{"a", "b", "c", "d"} {
		n := &Node{Key: k}
		g.Nodes = append(g.Nodes, n)
		g.index[k] = n
	}
	g.Edges = []Edge{
		{From: "a", To: "b"},
		{From: "b", To: "c"},
		{From: "a", To: "d"},
		{From: "c", To: "d"},
	}
	l := Layout(g)
	if l.Pos["d"].Layer != 3 {
		t.Errorf("layer(d) = %d, want 3 (longest path a->b->c->d)", l.Pos["d"].Layer)
	}
}
