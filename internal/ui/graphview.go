// Package ui holds the Bubble Tea models and rendering for the Flow viewer.
package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/chrismo/crux/internal/graph"
	"github.com/chrismo/crux/internal/rwx"
)

// stateGlyphs maps a display state to its glyph. Color comes from the theme
// (theme.State); glyphs live here so the render layout stays stable.
var stateGlyphs = map[rwx.DisplayState]string{
	rwx.StateRan:      "✓",
	rwx.StateCacheHit: "⚡",
	rwx.StateRunning:  "●",
	rwx.StateWaiting:  "○",
	rwx.StateFailed:   "✗",
	rwx.StateSkipped:  "⊘",
	rwx.StatePending:  "·",
}

func glyphFor(s rwx.DisplayState) string {
	if g, ok := stateGlyphs[s]; ok {
		return g
	}
	return "?"
}

// RenderGraph renders the layered layout top-down onto a rune canvas: layer 0
// (roots) at the top, each layer a row of state-colored node boxes, with real
// connectors routed between each task and its `use:` dependencies. Critical-
// path nodes get a thick border; blast-radius nodes get a red border and a "↯"
// marker. width is the available terminal width (threaded for the upcoming
// fit/collapse work; the full graph is drawn today and the viewport clips).
type RenderOpts struct {
	Crit      *graph.CritPath    // critical path: thick border (may be nil)
	Failure   *graph.FailureInfo // failures + blast radius (may be nil)
	Selected  string             // selected node key: double border + reverse ("" = none)
	Pinned    map[string]bool    // pinned anchor nodes: 📌 marker + accent border
	Collapsed map[[2]string]bool // edges (from,to) that fold away hidden nodes: dashed
}

// pinMark is the prefix shown on a pinned anchor node so it stands out.
const pinMark = "📌 "

// Box geometry constants. Boxes are 3 rows tall; gapV rows of routing space sit
// between layers, and gapH columns between sibling boxes.
const (
	boxHeight = 3
	gapV      = 2
	gapH      = 3
	layerStep = boxHeight + gapV
)

type nodeBox struct {
	x, y, w int
}

func (b nodeBox) cx() int   { return b.x + b.w/2 }
func (b nodeBox) top() int  { return b.y }
func (b nodeBox) bot() int  { return b.y + boxHeight - 1 }

// borderSet is a box-drawing corner/edge set for a node border style.
type borderSet struct{ tl, tr, bl, br, h, v rune }

var (
	roundedBorder = borderSet{'╭', '╮', '╰', '╯', '─', '│'}
	thickBorder   = borderSet{'┏', '┓', '┗', '┛', '━', '┃'}
	doubleBorder  = borderSet{'╔', '╗', '╚', '╝', '═', '║'}
)

func RenderGraph(g *graph.Graph, l *graph.LayoutData, width int, opts RenderOpts) string {
	_ = width // reserved for fit/pan in a later phase
	if len(l.Layers) == 0 {
		return ""
	}

	// Index by key so this works on both the original graph and a collapsed one
	// (which carries no internal index).
	nodeByKey := make(map[string]*graph.Node, len(g.Nodes))
	for _, n := range g.Nodes {
		nodeByKey[n.Key] = n
	}

	// 1. Geometry: place each layer as a row, boxes left to right.
	geo := make(map[string]nodeBox, len(g.Nodes))
	canvasW := 0
	for lv, layer := range l.Layers {
		x, y := 0, lv*layerStep
		for i, key := range layer {
			w := lipgloss.Width(labelFor(nodeByKey[key], inBlast(opts, key), opts.Pinned[key])) + 4 // 2 pad + 2 border
			geo[key] = nodeBox{x: x, y: y, w: w}
			x += w
			if i < len(layer)-1 {
				x += gapH
			}
		}
		if x > canvasW {
			canvasW = x
		}
	}
	canvasH := (len(l.Layers)-1)*layerStep + boxHeight
	cv := newCanvas(canvasW, canvasH)

	// 2. Connectors first, so boxes cleanly cover any crossings.
	drawConnectors(cv, g, geo, opts.Collapsed)

	// 3. Boxes.
	for _, n := range g.Nodes {
		drawBox(cv, n, geo[n.Key], opts)
	}

	// 4. Port markers where edges meet a box border (drawn over the border).
	drawPorts(cv, g, geo)

	return cv.String() + "\n"
}

func inBlast(opts RenderOpts, key string) bool {
	return opts.Failure != nil && opts.Failure.InBlast(key)
}

// labelFor is the text inside a node box (glyph, key, timing, blast marker,
// and a pin marker when the node is a pinned anchor).
func labelFor(n *graph.Node, onBlast, pinned bool) string {
	label := glyphFor(n.State) + " " + n.Key
	if n.HasTiming && n.DurationSeconds > 0 {
		label += fmt.Sprintf(" (%ds)", n.DurationSeconds)
	}
	if onBlast {
		label += " ↯"
	}
	if pinned {
		label = pinMark + label
	}
	return label
}

func drawBox(cv *canvas, n *graph.Node, b nodeBox, opts RenderOpts) {
	onCrit := opts.Crit != nil && opts.Crit.Contains(n.Key)
	onBlast := inBlast(opts, n.Key)
	selected := opts.Selected != "" && n.Key == opts.Selected
	pinned := opts.Pinned[n.Key]

	bs := roundedBorder
	switch {
	case selected:
		bs = doubleBorder
	case onCrit:
		bs = thickBorder
	}

	fg := theme.State(n.State).GetForeground()
	borderColor := fg
	if onBlast {
		borderColor = theme.Failure.GetForeground()
	}
	if pinned {
		borderColor = theme.Special.GetForeground() // pins win on border color so they're findable
	}
	bold := onCrit || selected || pinned
	bst := cellStyle{fg: borderColor, bold: bold}
	lst := cellStyle{fg: fg, bold: bold, reverse: selected}

	x, y, w := b.x, b.y, b.w
	cv.set(x, y, bs.tl, bst)
	cv.hline(x+1, x+w-2, y, bs.h, bst)
	cv.set(x+w-1, y, bs.tr, bst)

	cv.set(x, y+1, bs.v, bst)
	cv.text(x+1, y+1, " "+labelFor(n, onBlast, pinned)+" ", lst)
	cv.set(x+w-1, y+1, bs.v, bst)

	cv.set(x, y+2, bs.bl, bst)
	cv.hline(x+1, x+w-2, y+2, bs.h, bst)
	cv.set(x+w-1, y+2, bs.br, bst)
}

// Direction bits for connector cells; ORed so junctions/crossings resolve to
// the right box-drawing rune.
const (
	dirN uint8 = 1 << iota
	dirE
	dirS
	dirW
)

// drawConnectors routes an orthogonal line for every edge: down from the
// parent's bottom-center, across a bus row just above the child, then down into
// the child's top-center. Overlapping segments merge via direction masks.
// Collapsed edges (those that stand in for a path through hidden nodes) have
// their straight runs drawn dashed so a folded-away chain reads differently
// from a direct dependency.
func drawConnectors(cv *canvas, g *graph.Graph, geo map[string]nodeBox, collapsed map[[2]string]bool) {
	masks := map[[2]int]uint8{}
	solid := map[[2]int]bool{}  // a non-collapsed edge touched this cell
	dashed := map[[2]int]bool{} // a collapsed edge touched this cell
	mark := func(x, y int, isCollapsed bool) {
		if isCollapsed {
			dashed[[2]int{x, y}] = true
		} else {
			solid[[2]int{x, y}] = true
		}
	}
	addV := func(x, ya, yb int, isCollapsed bool) {
		if ya > yb {
			ya, yb = yb, ya
		}
		for y := ya; y < yb; y++ {
			masks[[2]int{x, y}] |= dirS
			masks[[2]int{x, y + 1}] |= dirN
			mark(x, y, isCollapsed)
			mark(x, y+1, isCollapsed)
		}
	}
	addH := func(xa, xb, y int, isCollapsed bool) {
		if xa > xb {
			xa, xb = xb, xa
		}
		for x := xa; x < xb; x++ {
			masks[[2]int{x, y}] |= dirE
			masks[[2]int{x + 1, y}] |= dirW
			mark(x, y, isCollapsed)
			mark(x+1, y, isCollapsed)
		}
	}

	for _, e := range g.Edges {
		from, ok1 := geo[e.From]
		to, ok2 := geo[e.To]
		if !ok1 || !ok2 {
			continue
		}
		c := collapsed[[2]string{e.From, e.To}]
		fcx, tcx := from.cx(), to.cx()
		startY := from.bot() + 1 // first row below the parent box
		busY := to.top() - 1     // row just above the child box
		if busY < startY {
			busY = startY
		}
		addV(fcx, startY, busY, c)
		addH(fcx, tcx, busY, c)
		addV(tcx, busY, to.top()-1, c)
		masks[[2]int{fcx, startY}] |= dirN       // up toward parent port
		masks[[2]int{tcx, to.top() - 1}] |= dirS // down toward child port
	}

	st := cellStyle{fg: theme.Muted.GetForeground()}
	for pt, m := range masks {
		r := maskRune(m)
		if dashed[pt] && !solid[pt] {
			r = dashRune(r)
		}
		cv.set(pt[0], pt[1], r, st)
	}
}

// dashRune swaps a straight box-drawing run for its dashed variant; junctions
// and corners are left solid (there are no dashed forms for them).
func dashRune(r rune) rune {
	switch r {
	case '│':
		return '┊'
	case '─':
		return '┈'
	}
	return r
}

// drawPorts marks where an edge meets a box: a "┬" on the parent's bottom
// border and a "┴" on the child's top border. Drawn after boxes so they sit on
// top of the border runes.
func drawPorts(cv *canvas, g *graph.Graph, geo map[string]nodeBox) {
	st := cellStyle{fg: theme.Muted.GetForeground()}
	for _, e := range g.Edges {
		from, ok1 := geo[e.From]
		to, ok2 := geo[e.To]
		if !ok1 || !ok2 {
			continue
		}
		cv.set(from.cx(), from.bot(), '┬', st)
		cv.set(to.cx(), to.top(), '┴', st)
	}
}

func maskRune(m uint8) rune {
	switch m {
	case dirN | dirS, dirN, dirS:
		return '│'
	case dirE | dirW, dirE, dirW:
		return '─'
	case dirN | dirE:
		return '└'
	case dirN | dirW:
		return '┘'
	case dirS | dirE:
		return '┌'
	case dirS | dirW:
		return '┐'
	case dirN | dirE | dirS:
		return '├'
	case dirN | dirW | dirS:
		return '┤'
	case dirE | dirS | dirW:
		return '┬'
	case dirE | dirN | dirW:
		return '┴'
	case dirN | dirE | dirS | dirW:
		return '┼'
	}
	return '·'
}

// FailureLine summarizes a run's failures and blast radius as one line, or "" if
// nothing failed.
func FailureLine(fi *graph.FailureInfo) string {
	if fi == nil || len(fi.Failed) == 0 {
		return ""
	}
	line := "failed: " + strings.Join(fi.Failed, ", ")
	blast := fi.BlastKeys()
	if len(blast) == 0 {
		line += " · no downstream impact"
	} else {
		line += " · blast radius: " + strings.Join(blast, ", ")
	}
	return line
}

// filterHeader describes the active collapse: which overlays are on and how
// many of the run's nodes remain visible.
func filterHeader(ov graphOverlay, visible, total int) string {
	var parts []string
	if ov.Filter != "" {
		parts = append(parts, "filter: "+ov.Filter)
	}
	if len(ov.Pinned) > 0 {
		pins := make([]string, 0, len(ov.Pinned))
		for k := range ov.Pinned {
			pins = append(pins, k)
		}
		sort.Strings(pins)
		parts = append(parts, "📌 "+strings.Join(pins, ", "))
	}
	head := strings.Join(parts, " · ")
	if visible == 0 {
		return head + "  (no matches)"
	}
	return fmt.Sprintf("%s  (%d of %d shown)", head, visible, total)
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
		parts = append(parts, fmt.Sprintf("%s %s", glyphFor(s), s))
	}
	return strings.Join(parts, "   ")
}
