package ui

import (
	"strings"

	"github.com/chrismo/crux/internal/graph"
)

// computeVisible returns the set of node keys that survive the active
// filter/focus overlays, or nil when nothing is active (meaning "show all").
// Filter is a case-insensitive substring on the key; Focus (when non-nil)
// constrains to its cone. Both apply together.
func computeVisible(g *graph.Graph, ov graphOverlay) map[string]bool {
	if ov.Focus == nil && ov.Filter == "" {
		return nil
	}
	filter := strings.ToLower(ov.Filter)
	vis := make(map[string]bool)
	for _, n := range g.Nodes {
		if ov.Focus != nil && !ov.Focus[n.Key] {
			continue
		}
		if filter != "" && !strings.Contains(strings.ToLower(n.Key), filter) {
			continue
		}
		vis[n.Key] = true
	}
	return vis
}

// collapseGraph builds a graph over only the visible nodes, preserving reach/
// paths: two visible nodes get an edge when one reaches the other through zero
// or more hidden intermediates (stopping at the first visible node on each
// path). Edges that traverse at least one hidden node are returned in the
// collapsed set so the renderer can style them distinctly (a dashed connector
// that stands for a path folded away).
//
// The result Graph carries no index (its Node method is unused here); Layout
// and RenderGraph both operate off the exported Nodes/Edges only.
func collapseGraph(g *graph.Graph, visible map[string]bool) (*graph.Graph, map[[2]string]bool) {
	succs := make(map[string][]string, len(g.Nodes))
	for _, e := range g.Edges {
		succs[e.From] = append(succs[e.From], e.To)
	}

	edgeCollapsed := make(map[[2]string]bool) // false = direct wins
	for _, n := range g.Nodes {
		if !visible[n.Key] {
			continue
		}
		seen := make(map[string]bool)
		var walk func(cur string, viaHidden bool)
		walk = func(cur string, viaHidden bool) {
			for _, s := range succs[cur] {
				if visible[s] {
					key := [2]string{n.Key, s}
					if prev, ok := edgeCollapsed[key]; !ok {
						edgeCollapsed[key] = viaHidden
					} else if prev && !viaHidden {
						edgeCollapsed[key] = false // a direct path exists; prefer it
					}
					continue // don't descend past a visible node
				}
				if !seen[s] {
					seen[s] = true
					walk(s, true)
				}
			}
		}
		walk(n.Key, false)
	}

	nodes := make([]*graph.Node, 0, len(visible))
	for _, n := range g.Nodes {
		if visible[n.Key] {
			nodes = append(nodes, n)
		}
	}
	edges := make([]graph.Edge, 0, len(edgeCollapsed))
	collapsed := make(map[[2]string]bool)
	for k, c := range edgeCollapsed {
		edges = append(edges, graph.Edge{From: k[0], To: k[1]})
		if c {
			collapsed[k] = true
		}
	}
	return &graph.Graph{Nodes: nodes, Edges: edges}, collapsed
}
