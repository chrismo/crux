package graph

import "sort"

// CritPath is the critical path through the graph: the node chain (root -> leaf)
// with the greatest total weight, plus that total.
type CritPath struct {
	Keys  []string
	Total int
	set   map[string]bool
}

// Contains reports whether a node key lies on the critical path.
func (c *CritPath) Contains(key string) bool { return c.set[key] }

// CriticalPath computes the heaviest dependency chain. Weight is each node's
// real duration (ExecutionRuntimeSeconds, else CompletedRuntimeSeconds, captured
// in Node.DurationSeconds). If no node has timing data, it falls back to depth
// (every node weighted 1), yielding the longest chain by edge count.
func CriticalPath(g *Graph) *CritPath {
	anyTiming := false
	for _, n := range g.Nodes {
		if n.HasTiming {
			anyTiming = true
			break
		}
	}
	weight := func(n *Node) int {
		if anyTiming {
			return n.DurationSeconds
		}
		return 1
	}

	preds := map[string][]string{}
	indeg := map[string]int{}
	for _, n := range g.Nodes {
		indeg[n.Key] = 0
	}
	succs := map[string][]string{}
	for _, e := range g.Edges {
		preds[e.To] = append(preds[e.To], e.From)
		succs[e.From] = append(succs[e.From], e.To)
		indeg[e.To]++
	}

	// best[n] = heaviest chain ending at n; from[n] = chosen predecessor.
	best := map[string]int{}
	from := map[string]string{}

	for _, k := range topoOrder(g, succs, indeg) {
		w := weight(g.index[k])
		bestPred, bestPredScore := "", -1
		ps := append([]string(nil), preds[k]...)
		sort.Strings(ps) // deterministic tie-break
		for _, p := range ps {
			if best[p] > bestPredScore {
				bestPredScore, bestPred = best[p], p
			}
		}
		if bestPred == "" {
			best[k] = w
		} else {
			best[k] = w + bestPredScore
			from[k] = bestPred
		}
	}

	// End node = max best, tie-break by key for determinism.
	end, endScore := "", -1
	keys := make([]string, 0, len(best))
	for k := range best {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if best[k] > endScore {
			endScore, end = best[k], k
		}
	}

	// Reconstruct root -> leaf.
	var rev []string
	for k := end; k != ""; k = from[k] {
		rev = append(rev, k)
	}
	path := make([]string, len(rev))
	for i := range rev {
		path[i] = rev[len(rev)-1-i]
	}

	set := make(map[string]bool, len(path))
	for _, k := range path {
		set[k] = true
	}
	return &CritPath{Keys: path, Total: endScore, set: set}
}

// topoOrder returns a deterministic topological ordering of the graph's nodes.
func topoOrder(g *Graph, succs map[string][]string, indeg map[string]int) []string {
	in := map[string]int{}
	for k, d := range indeg {
		in[k] = d
	}
	var queue []string
	for _, n := range g.Nodes {
		if in[n.Key] == 0 {
			queue = append(queue, n.Key)
		}
	}
	sort.Strings(queue)

	var order []string
	for len(queue) > 0 {
		k := queue[0]
		queue = queue[1:]
		order = append(order, k)
		next := append([]string(nil), succs[k]...)
		sort.Strings(next)
		for _, s := range next {
			in[s]--
			if in[s] == 0 {
				queue = append(queue, s)
				sort.Strings(queue)
			}
		}
	}
	return order
}
