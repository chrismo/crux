// Package ui holds the Bubble Tea models and rendering for the Flow viewer.
package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/chrismo/rwx-tui/internal/graph"
	"github.com/chrismo/rwx-tui/internal/rwx"
)

// stateStyle maps a display state to a glyph and a color. Colors are applied via
// lipgloss; in a non-color terminal (tests, pipes) only the glyph/text shows.
type stateStyle struct {
	glyph string
	color lipgloss.Color
}

var stateStyles = map[rwx.DisplayState]stateStyle{
	rwx.StateRan:      {glyph: "✓", color: lipgloss.Color("2")},  // green
	rwx.StateCacheHit: {glyph: "⚡", color: lipgloss.Color("6")},  // cyan
	rwx.StateRunning:  {glyph: "●", color: lipgloss.Color("3")},  // yellow
	rwx.StateWaiting:  {glyph: "○", color: lipgloss.Color("8")},  // gray
	rwx.StateFailed:   {glyph: "✗", color: lipgloss.Color("1")},  // red
	rwx.StateSkipped:  {glyph: "⊘", color: lipgloss.Color("8")},  // gray
	rwx.StatePending:  {glyph: "·", color: lipgloss.Color("8")},  // gray
}

func glyphFor(s rwx.DisplayState) string {
	if st, ok := stateStyles[s]; ok {
		return st.glyph
	}
	return "?"
}

// RenderGraph renders the layered layout top-down: layer 0 (roots) at the top,
// each layer a row of state-colored node cells. Edge routing is a follow-up;
// this v1 conveys structure via layering and state via color/glyph. Nodes on
// the critical path (cp; may be nil) get a thick border to stand out.
func RenderGraph(g *graph.Graph, l *graph.LayoutData, cp *graph.CritPath) string {
	rows := make([]string, 0, len(l.Layers))
	for _, layer := range l.Layers {
		cells := make([]string, 0, len(layer))
		for _, key := range layer {
			onCrit := cp != nil && cp.Contains(key)
			cells = append(cells, renderCell(g.Node(key), onCrit))
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, cells...))
	}
	// A simple top-down flow cue between layer rows; true edge routing is a
	// follow-up.
	return strings.Join(rows, "\n   │\n") + "\n"
}

func renderCell(n *graph.Node, onCrit bool) string {
	st, ok := stateStyles[n.State]
	if !ok {
		st = stateStyle{glyph: "?", color: lipgloss.Color("8")}
	}
	label := fmt.Sprintf("%s %s", st.glyph, n.Key)
	if n.HasTiming && n.DurationSeconds > 0 {
		label += fmt.Sprintf(" (%ds)", n.DurationSeconds)
	}
	border := lipgloss.RoundedBorder()
	if onCrit {
		border = lipgloss.ThickBorder()
	}
	box := lipgloss.NewStyle().
		Border(border).
		BorderForeground(st.color).
		Foreground(st.color).
		Bold(onCrit).
		Padding(0, 1).
		MarginRight(2)
	return box.Render(label)
}

// CriticalPathLine summarizes the critical path as a one-line chain with total.
func CriticalPathLine(cp *graph.CritPath) string {
	if cp == nil || len(cp.Keys) == 0 {
		return ""
	}
	return fmt.Sprintf("critical path: %s · %ds", strings.Join(cp.Keys, " → "), cp.Total)
}

// Legend returns a one-line key of state glyphs for a footer.
func Legend() string {
	order := []rwx.DisplayState{
		rwx.StateRan, rwx.StateCacheHit, rwx.StateRunning,
		rwx.StateWaiting, rwx.StateFailed, rwx.StateSkipped, rwx.StatePending,
	}
	parts := make([]string, 0, len(order))
	for _, s := range order {
		parts = append(parts, fmt.Sprintf("%s %s", stateStyles[s].glyph, s))
	}
	return strings.Join(parts, "   ")
}
