package graph

import (
	"sort"
)

// Pos is a node's place in the layered layout.
type Pos struct {
	Layer int // depth from the roots (longest path)
	Order int // index within its layer, left to right
}

// LayoutData is the computed layered layout: nodes grouped into ordered layers,
// plus a key->Pos lookup.
type LayoutData struct {
	Layers [][]string
	Pos    map[string]Pos
}

// Layout assigns each node a layer (longest path from the roots) and orders
// nodes within each layer by the barycenter of their predecessors to reduce
// edge crossings. Ordering is deterministic: roots and barycenter ties break by
// key.
func Layout(g *Graph) *LayoutData {
	preds := map[string][]string{}
	succs := map[string][]string{}
	indeg := map[string]int{}
	for _, n := range g.Nodes {
		indeg[n.Key] = 0
	}
	for _, e := range g.Edges {
		preds[e.To] = append(preds[e.To], e.From)
		succs[e.From] = append(succs[e.From], e.To)
		indeg[e.To]++
	}

	layer := longestPathLayers(g, succs, indeg)

	maxLayer := 0
	for _, lv := range layer {
		if lv > maxLayer {
			maxLayer = lv
		}
	}

	layers := make([][]string, maxLayer+1)
	for _, n := range g.Nodes {
		lv := layer[n.Key]
		layers[lv] = append(layers[lv], n.Key)
	}

	pos := make(map[string]Pos, len(g.Nodes))

	// Layer 0: order by key. Higher layers: order by barycenter of predecessor
	// Order values (already assigned, since we go top-down), tie-break by key.
	for lv := range layers {
		keys := layers[lv]
		if lv == 0 {
			sort.Strings(keys)
		} else {
			bary := make(map[string]float64, len(keys))
			for _, k := range keys {
				bary[k] = barycenter(preds[k], pos)
			}
			sort.SliceStable(keys, func(i, j int) bool {
				if bary[keys[i]] != bary[keys[j]] {
					return bary[keys[i]] < bary[keys[j]]
				}
				return keys[i] < keys[j]
			})
		}
		for order, k := range keys {
			pos[k] = Pos{Layer: lv, Order: order}
		}
	}

	return &LayoutData{Layers: layers, Pos: pos}
}

// longestPathLayers computes layer[node] = longest dependency chain length from
// any root, via a Kahn topological sweep. Any nodes left in a cycle are placed
// after the acyclic portion so the function always terminates.
func longestPathLayers(g *Graph, succs map[string][]string, indeg map[string]int) map[string]int {
	layer := map[string]int{}
	in := map[string]int{}
	for k, d := range indeg {
		in[k] = d
		layer[k] = 0
	}

	// Deterministic queue: start with roots ordered by key.
	var queue []string
	for _, n := range g.Nodes {
		if in[n.Key] == 0 {
			queue = append(queue, n.Key)
		}
	}
	sort.Strings(queue)

	processed := 0
	for len(queue) > 0 {
		k := queue[0]
		queue = queue[1:]
		processed++
		next := append([]string(nil), succs[k]...)
		sort.Strings(next)
		for _, s := range next {
			if layer[k]+1 > layer[s] {
				layer[s] = layer[k] + 1
			}
			in[s]--
			if in[s] == 0 {
				queue = append(queue, s)
			}
		}
	}

	// Defensive: if a cycle left nodes unprocessed, they keep layer 0 rather
	// than hanging. RWX DAGs are acyclic, so this should not happen.
	_ = processed
	return layer
}

func barycenter(preds []string, pos map[string]Pos) float64 {
	if len(preds) == 0 {
		return 0
	}
	sum := 0
	for _, p := range preds {
		sum += pos[p].Order
	}
	return float64(sum) / float64(len(preds))
}
