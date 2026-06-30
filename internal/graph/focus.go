package graph

// Focus returns the subgraph relevant to key: key plus all of its ancestors
// (nodes it transitively depends on) and descendants (nodes that transitively
// depend on it). Returns nil if key is not in the graph. Used to isolate a
// node's dependency cone in the viewer.
func Focus(g *Graph, key string) map[string]bool {
	if g.index[key] == nil {
		return nil
	}
	succs := map[string][]string{}
	preds := map[string][]string{}
	for _, e := range g.Edges {
		succs[e.From] = append(succs[e.From], e.To)
		preds[e.To] = append(preds[e.To], e.From)
	}
	set := map[string]bool{key: true}
	// descendants() walks whichever adjacency it's given: succs forward
	// (descendants), preds backward (ancestors).
	for _, d := range descendants(key, succs) {
		set[d] = true
	}
	for _, a := range descendants(key, preds) {
		set[a] = true
	}
	return set
}
