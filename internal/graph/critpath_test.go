package graph

import "testing"

func TestCriticalPathByDuration(t *testing.T) {
	g := loadFixtureGraph(t)
	cp := CriticalPath(g)

	wantKeys := []string{"code", "deps", "test"} // 5 + 2 + 13 = 20
	if !equalStrings(cp.Keys, wantKeys) {
		t.Errorf("Keys = %v, want %v", cp.Keys, wantKeys)
	}
	if cp.Total != 20 {
		t.Errorf("Total = %d, want 20", cp.Total)
	}
	if !cp.Contains("test") {
		t.Error("Contains(test) = false, want true")
	}
	if cp.Contains("vet") {
		t.Error("Contains(vet) = true; vet is not on the critical path")
	}
}

// With no timing data anywhere, the critical path falls back to the longest
// chain by depth (each node weighted 1).
func TestCriticalPathDepthFallback(t *testing.T) {
	g := &Graph{index: map[string]*Node{}}
	for _, k := range []string{"a", "b", "c", "d"} {
		n := &Node{Key: k, HasTiming: false}
		g.Nodes = append(g.Nodes, n)
		g.index[k] = n
	}
	g.Edges = []Edge{
		{From: "a", To: "b"},
		{From: "b", To: "c"},
		{From: "a", To: "d"},
	}
	cp := CriticalPath(g)
	want := []string{"a", "b", "c"}
	if !equalStrings(cp.Keys, want) {
		t.Errorf("Keys = %v, want %v", cp.Keys, want)
	}
	if cp.Total != 3 {
		t.Errorf("Total = %d, want 3 (depth)", cp.Total)
	}
}
