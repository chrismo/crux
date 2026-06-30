package graph

import (
	"sort"

	"github.com/chrismo/rwx-tui/internal/rwx"
)

// FailureInfo describes the failures in a run and their downstream blast radius.
type FailureInfo struct {
	Failed []string // failed node keys, in deterministic order
	blast  map[string]bool
}

// First returns the first failed node key (deterministic), or "" if none failed.
func (f *FailureInfo) First() string {
	if len(f.Failed) == 0 {
		return ""
	}
	return f.Failed[0]
}

// InBlast reports whether a node is downstream of a failure (and not itself a
// failed node).
func (f *FailureInfo) InBlast(key string) bool { return f.blast[key] }

// BlastKeys returns the blast-radius node keys in sorted order.
func (f *FailureInfo) BlastKeys() []string {
	keys := make([]string, 0, len(f.blast))
	for k := range f.blast {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// AnalyzeFailures finds failed nodes and the set of their descendants (the blast
// radius). Failed nodes themselves are excluded from the blast set.
func AnalyzeFailures(g *Graph) *FailureInfo {
	succs := map[string][]string{}
	for _, e := range g.Edges {
		succs[e.From] = append(succs[e.From], e.To)
	}

	var failed []string
	for _, n := range g.Nodes {
		if n.State == rwx.StateFailed {
			failed = append(failed, n.Key)
		}
	}
	sort.Strings(failed)

	blast := map[string]bool{}
	for _, f := range failed {
		for _, d := range descendants(f, succs) {
			blast[d] = true
		}
	}
	// A failed node is not part of its own blast radius.
	for _, f := range failed {
		delete(blast, f)
	}

	return &FailureInfo{Failed: failed, blast: blast}
}

// descendants returns all nodes reachable from start by following edges.
func descendants(start string, succs map[string][]string) []string {
	seen := map[string]bool{}
	var stack []string
	stack = append(stack, succs[start]...)
	for len(stack) > 0 {
		k := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[k] {
			continue
		}
		seen[k] = true
		stack = append(stack, succs[k]...)
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
