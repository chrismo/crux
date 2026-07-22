package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/chrismo/crux/internal/graph"
	"github.com/chrismo/crux/internal/rwx"
)

// ---- Graph view (a single run) -------------------------------------------

// Model renders a single run's graph headlessly (the --print path). It honors
// pin terms so print stays feature-equivalent to the interactive graph.
type Model struct {
	run     rwx.Run
	graph   *graph.Graph
	layout  *graph.LayoutData
	overlay graphOverlay
}

// NewModel builds the headless graph view from a fetched run, pre-pinning any
// substring terms and applying a graph filter (all empty = the full graph). As
// in the interactive view, a filter is a global finder that overrides the pins.
func NewModel(run rwx.Run, pinTerms []string, filter string) Model {
	g := graph.Build(run)
	pins := pinListFor(g, pinTerms)
	return Model{
		run:    run,
		graph:  g,
		layout: graph.Layout(g),
		overlay: graphOverlay{
			Focus:  focusSetOf(g, pins),
			Pinned: pinnedSetOf(pins),
			Filter: filter,
		},
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) View() string {
	return Screen(m.run, m.graph, m.layout, 0, m.overlay)
}

// graphOverlay carries the interactive overlays (cursor + focus/filter) into
// Screen. The zero value (used by --print) shows no overlays.
type graphOverlay struct {
	Selected string
	Focus    map[string]bool // visible cone (union of pinned anchors' cones)
	Pinned   map[string]bool // the pinned anchors themselves (for the pin marker)
	Filter   string
}

// Screen renders the full graph view (header, graph, legend) as a string. Pure,
// so it backs both View() and the headless --print path; ov is empty for
// --print, which has no cursor or filter.
func Screen(run rwx.Run, g *graph.Graph, l *graph.LayoutData, width int, ov graphOverlay) string {
	var b strings.Builder

	status := run.ResultStatus
	if !run.Completed {
		status = "in progress"
	}
	header := fmt.Sprintf("RWX run %s · %s · %s", short(run.RunID), run.DefinitionPath, status)
	b.WriteString(theme.Header.Render(header))
	b.WriteString("\n")

	cp := graph.CriticalPath(g)
	if line := CriticalPathLine(cp); line != "" {
		b.WriteString(theme.Special.Render(line))
		b.WriteString("\n")
	}

	fi := graph.AnalyzeFailures(g)
	if line := FailureLine(fi); line != "" {
		b.WriteString(theme.Failure.Render(line))
		b.WriteString("\n")
	}

	// Filter/focus collapse the graph to a visible set (rather than dimming);
	// path-preserving connectors stand in for folded-away chains.
	rg, rl, collapsed := g, l, map[[2]string]bool(nil)
	if visible := computeVisible(g, ov); visible != nil {
		b.WriteString(theme.Special.Render(filterHeader(ov, len(visible), len(g.Nodes))))
		b.WriteString("\n")
		if len(visible) == 0 {
			b.WriteString("\n")
			b.WriteString(theme.Faint.Render(Legend()))
			b.WriteString("\n")
			return b.String()
		}
		rg, collapsed = collapseGraph(g, visible)
		rl = graph.Layout(rg)
	}
	b.WriteString("\n")

	b.WriteString(RenderGraph(rg, rl, width, RenderOpts{
		Crit: cp, Failure: fi, Selected: ov.Selected, Pinned: ov.Pinned, Collapsed: collapsed,
	}))
	b.WriteString("\n")
	b.WriteString(theme.Faint.Render(Legend()))
	b.WriteString("\n")
	return b.String()
}

func short(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// HomeView renders the run-list landing (header, status line, list). runs is
// the already view-filtered slice; total is the full fetched count (for the
// "(n of N shown)" hint). scope is the fetch scope ("all"/"mine"/"branch"), def
// is the --definition scope, and filter is the active view-filter term.
func HomeView(runs []rwx.RunSummary, selected int, now time.Time, scope, def, filter string, total int) string {
	var b strings.Builder
	// No repo in the header: the list mixes repositories, so naming the first
	// run's repo mislabels the rest. Each row carries its own repo column now,
	// and an explicit --repository scope shows up in the status line.
	b.WriteString(theme.Header.Render("crux"))
	if s := listStatus(scope, def, filter, len(runs), total); s != "" {
		b.WriteString("  " + theme.Special.Render(s))
	}
	b.WriteString("\n\n")
	b.WriteString(RenderRunList(runs, selected, now))
	b.WriteString("\n")
	return b.String()
}

// listStatus is the header suffix describing the filter tiers: the fetch scope
// (empty for the default "all"), the --definition scope, and the typed view
// filter. The shown/total count is appended whenever a view filter (def or
// typed) is narrowing the list.
func listStatus(scope, def, filter string, shown, total int) string {
	var parts []string
	if scope != "" {
		parts = append(parts, scope)
	}
	if def != "" {
		parts = append(parts, "def: "+def)
	}
	if filter != "" {
		parts = append(parts, "filter: "+filter)
	}
	s := strings.Join(parts, " · ")
	if def != "" || filter != "" {
		s += fmt.Sprintf("  (%d of %d shown)", shown, total)
	}
	return s
}

// ---- App router (list <-> graph) -----------------------------------------

type appMode int

const (
	modeLoading appMode = iota
	modeList
	modeGraph
)

// AppConfig configures the root App.
type AppConfig struct {
	Run         string         // open this run directly, skipping the list
	Filter      rwx.ListFilter // server-side fetch filter for the run list
	Pins        []string       // substring terms to pre-pin on the first run opened
	GraphFilter string         // initial graph node filter (type-to-filter seed)
	ListFilter  string         // initial run-list view filter (type-to-filter seed)
	DefinitionFilter string    // --definition: persistent DefinitionPath-only list scope
	GithubBase       string    // https://github.com/{owner}/{repo} for ctrl+g commit links
}

// App is the root Bubble Tea model. It starts on the run list (the home) and
// opens a run's graph on selection; with AppConfig.Run set it opens that run
// directly.
type App struct {
	client *rwx.Client
	cfg    AppConfig
	now    func() time.Time

	mode    appMode
	hasList bool // a list exists to return to via esc

	keys    keyMap
	help    help.Model
	spinner spinner.Model

	width    int
	height   int
	viewport viewport.Model

	runs        []rwx.RunSummary
	selected    int    // index into the *visible* (view-filtered) runs
	defFilter   string // --definition scope: DefinitionPath-only, persists past esc
	listFilter  string // client-side view filter over the fetched runs (type-to-filter)
	nextCursor  string // pagination cursor for the next page ("" = no more)
	loadingMore bool

	run          rwx.Run
	runUrl       string    // cloud.rwx.com URL of the open run, carried from the list (ctrl+o)
	githubBase   string    // https://github.com/{owner}/{repo} for commit links (ctrl+g)
	graph        *graph.Graph
	layout       *graph.LayoutData
	selectedNode string    // key of the highlighted graph node
	xOffset      int       // horizontal pan offset for the graph viewport
	pins         []pin     // pinned anchors (a set); cones combine per pin.refine
	pinsSeeded   bool      // AppConfig.Pins have been applied (one-time, first run)
	history      []viewState // focus back-stack: esc pops one snapshot
	graphFilter  string      // live graph filter (type-to-filter); mirrors listFilter
	logsLoading  bool   // logs fetch in flight for the open detail pane
	detailOpen   bool   // detail pane shown for the selected node
	logsContent  string // fetched logs ("" = show task detail instead)

	openURL func(string) error // opens a URL in the browser (injected for tests)

	notice string // transient one-line footer note (e.g. "nothing to open"); cleared on the next key
	err    error
}

// NewApp builds the root model. The viewport is seeded with a sane default size
// so the first frame renders before the initial WindowSizeMsg arrives.
func NewApp(client *rwx.Client, cfg AppConfig) App {
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))
	sp.Style = theme.Running
	return App{
		graphFilter: cfg.GraphFilter,      // optional --graph-filter seed
		listFilter:  cfg.ListFilter,       // optional --list-filter seed
		defFilter:   cfg.DefinitionFilter, // optional --definition scope
		githubBase:  cfg.GithubBase,
		openURL:     openInBrowser,
		client:   client,
		cfg:      cfg,
		now:      time.Now,
		mode:     modeLoading,
		keys:     defaultKeyMap(),
		help:     help.New(),
		spinner:  sp,
		width:    80,
		height:   24,
		viewport: viewport.New(80, 23),
	}
}

// bodyContent is the scrollable body for the current mode (the same pure-render
// output that --print uses, fed into the viewport).
func (a App) bodyContent() string {
	switch a.mode {
	case modeList:
		return HomeView(a.visibleRuns(), a.selected, a.now(), FetchLabel(a.cfg.Filter), a.defFilter, a.listFilter, len(a.runs))
	case modeGraph:
		if a.detailOpen {
			switch {
			case a.logsLoading:
				return a.spinner.View() + " loading logs…"
			case a.logsContent != "":
				return a.logsContent
			}
			return RenderDetail(a.run.FindTask(a.selectedNode))
		}
		return Screen(a.run, a.graph, a.layout, a.width, a.currentOverlay())
	default:
		return ""
	}
}

// refresh re-feeds the viewport from the current state.
func (a *App) refresh() {
	a.viewport.SetContent(a.bodyContent())
}

// firstNode returns the top-left node key of a layout, or "".
func firstNode(l *graph.LayoutData) string {
	if l == nil || len(l.Layers) == 0 || len(l.Layers[0]) == 0 {
		return ""
	}
	return l.Layers[0][0]
}

// currentOverlay describes the active cursor/filter/pins for rendering and for
// deriving the visible set.
func (a *App) currentOverlay() graphOverlay {
	return graphOverlay{
		Selected: a.selectedNode,
		Focus:    a.focusSet(),
		Pinned:   a.pinnedSet(),
		Filter:   a.graphFilter,
	}
}

// pin is a pinned anchor node. refine records how its cone combines with the
// pins before it: true = intersect (the node was already visible, so it narrows
// the view — the nested "hide the siblings" case), false = union (the node was
// brought in from elsewhere via the global filter, so it adds a second area).
// The first pin's refine is unused (it's the base).
type pin struct {
	key    string
	refine bool
}

// viewState is a snapshot of the graph focus: the filter, the pin set, and the
// cursor. Every focus mutation pushes the prior state; esc pops one. This is the
// whole "undo" mechanism — pins stay a plain set you edit forward, and history
// is the stack you walk backward, so the two never entangle.
type viewState struct {
	filter   string
	pins     []pin
	selected string
}

// maxHistory caps the back-stack; older snapshots are dropped.
const maxHistory = 50

// pushHistory records the current focus state before a mutation.
func (a *App) pushHistory() {
	a.history = append(a.history, viewState{
		filter:   a.graphFilter,
		pins:     append([]pin(nil), a.pins...),
		selected: a.selectedNode,
	})
	if len(a.history) > maxHistory {
		a.history = a.history[len(a.history)-maxHistory:]
	}
}

// popHistory restores the most recent snapshot. Reports whether one existed.
func (a *App) popHistory() bool {
	if len(a.history) == 0 {
		return false
	}
	s := a.history[len(a.history)-1]
	a.history = a.history[:len(a.history)-1]
	a.graphFilter = s.filter
	a.pins = s.pins
	a.selectedNode = s.selected
	return true
}

// focusSetOf combines pinned anchors' cones per each pin's refine flag (nil when
// empty). refine=true intersects (narrow), false unions (add). Anchors are
// always kept visible so a pin never vanishes. Shared by the App and the
// headless --print path so both render pins identically.
func focusSetOf(g *graph.Graph, pins []pin) map[string]bool {
	if len(pins) == 0 || g == nil {
		return nil
	}
	set := make(map[string]bool)
	for k := range graph.Focus(g, pins[0].key) {
		set[k] = true
	}
	for _, p := range pins[1:] {
		cone := graph.Focus(g, p.key)
		if p.refine {
			for k := range set {
				if !cone[k] {
					delete(set, k)
				}
			}
		} else {
			for k := range cone {
				set[k] = true
			}
		}
	}
	for _, p := range pins { // anchors always stay visible
		set[p.key] = true
	}
	return set
}

// pinnedSetOf is the set of pinned anchor keys (for rendering the pin marker).
func pinnedSetOf(pins []pin) map[string]bool {
	if len(pins) == 0 {
		return nil
	}
	m := make(map[string]bool, len(pins))
	for _, p := range pins {
		m[p.key] = true
	}
	return m
}

// pinListFor resolves substring terms to an ordered pin list using the adaptive
// refine/add rule (a match already visible in the running pin view refines;
// otherwise it adds). Shared by --pin seeding and the print path.
func pinListFor(g *graph.Graph, terms []string) []pin {
	if g == nil {
		return nil
	}
	var pins []pin
	pinned := func(k string) bool {
		for _, p := range pins {
			if p.key == k {
				return true
			}
		}
		return false
	}
	for _, term := range terms {
		lt := strings.ToLower(strings.TrimSpace(term))
		if lt == "" {
			continue
		}
		for _, n := range g.Nodes {
			if strings.Contains(strings.ToLower(n.Key), lt) && !pinned(n.Key) {
				pins = append(pins, pin{key: n.Key, refine: focusSetOf(g, pins)[n.Key]})
			}
		}
	}
	return pins
}

func (a *App) focusSet() map[string]bool  { return focusSetOf(a.graph, a.pins) }
func (a *App) pinnedSet() map[string]bool { return pinnedSetOf(a.pins) }

// togglePin adds or removes a node from the pin set — a pure forward edit; back-
// out is handled by history (esc), not here. A newly-pinned node refines
// (intersects) when it's already inside the pin view and adds (unions) otherwise.
func (a *App) togglePin(key string) {
	if key == "" {
		return
	}
	for i, p := range a.pins {
		if p.key == key {
			a.pins = append(a.pins[:i], a.pins[i+1:]...)
			return
		}
	}
	a.pins = append(a.pins, pin{key: key, refine: a.focusSet()[key]})
}

// seedPins applies AppConfig.Pins once, when the first run opens. Each term is a
// case-insensitive substring; every node it matches is pinned (via the adaptive
// refine/add rule), so `--pin api` pins every api* node and a critical-path term
// narrows to that path. Pins persist across later runs, so this runs once.
func (a *App) seedPins() {
	if a.pinsSeeded || a.graph == nil {
		return
	}
	a.pinsSeeded = true
	a.pins = append(a.pins, pinListFor(a.graph, a.cfg.Pins)...)
	a.clampSelectionToVisible()
}

// activeLayout is the layout the graph is currently drawn with: the collapsed
// layout when a filter/focus is active, else the full layout. Cursor movement
// walks this, so the selection only ever lands on a visible node.
func (a *App) activeLayout() *graph.LayoutData {
	if a.graph == nil {
		return a.layout
	}
	visible := computeVisible(a.graph, a.currentOverlay())
	if visible == nil {
		return a.layout
	}
	if len(visible) == 0 {
		return &graph.LayoutData{}
	}
	rg, _ := collapseGraph(a.graph, visible)
	return graph.Layout(rg)
}

// clampSelectionToVisible keeps the cursor on a visible node after the
// filter/focus set changes (a filtered-out selection jumps to the first
// visible node).
func (a *App) clampSelectionToVisible() {
	lay := a.activeLayout()
	if lay == nil {
		return
	}
	if _, ok := lay.Pos[a.selectedNode]; a.selectedNode == "" || !ok {
		a.selectedNode = firstNode(lay)
		a.xOffset = 0
	}
}

// moveSelection moves the graph cursor by dLayer (rows) and dOrder (columns
// within a layer), clamped to the currently-visible layout.
func (a *App) moveSelection(dLayer, dOrder int) {
	lay := a.activeLayout()
	if lay == nil || a.selectedNode == "" {
		return
	}
	pos, ok := lay.Pos[a.selectedNode]
	if !ok {
		a.selectedNode = firstNode(lay) // selection was hidden; land on a visible node
		return
	}
	if dLayer != 0 {
		target := pos.Layer + dLayer
		if target < 0 || target >= len(lay.Layers) {
			return
		}
		row := lay.Layers[target]
		idx := pos.Order
		if idx >= len(row) {
			idx = len(row) - 1
		}
		a.selectedNode = row[idx]
	}
	if dOrder != 0 {
		row := lay.Layers[pos.Layer]
		idx := pos.Order + dOrder
		if idx < 0 || idx >= len(row) {
			return
		}
		a.selectedNode = row[idx]
	}
}

// ensureSelectedVisible scrolls the viewport so the selected node's box is in
// view both vertically and horizontally. Best-effort: locates the node's label
// line and column in the rendered body.
func (a *App) ensureSelectedVisible() {
	if a.selectedNode == "" {
		return
	}
	lines := strings.Split(a.bodyContent(), "\n")
	target, col := -1, -1
	for i, ln := range lines {
		if c := nodeColumn(ln, a.selectedNode); c >= 0 {
			target, col = i, c
			break
		}
	}
	if target < 0 {
		return
	}
	a.scrollRowIntoView(target)

	// Horizontal pan: keep the whole selected box (left border to right border)
	// within view. Matters only for the wide unfiltered graph; the collapsed
	// view usually fits. col is the box's left border; the right border sits one
	// cell past the "│ key │" span.
	if a.viewport.Width > 0 && col >= 0 {
		colRight := col + lipgloss.Width(" "+a.selectedNode+" ") + 1
		left := a.xOffset
		right := left + a.viewport.Width - 1
		switch {
		case col < left:
			a.xOffset = col
		case colRight > right:
			a.xOffset = colRight - a.viewport.Width + 1
		}
		if a.xOffset < 0 {
			a.xOffset = 0
		}
		a.viewport.SetXOffset(a.xOffset)
	}
}

// scrollRowIntoView scrolls the viewport vertically the minimum amount to bring
// the given content line into view.
func (a *App) scrollRowIntoView(target int) {
	top := a.viewport.YOffset
	bottom := top + a.viewport.Height - 1
	switch {
	case target < top:
		a.viewport.SetYOffset(target)
	case target > bottom:
		a.viewport.SetYOffset(target - a.viewport.Height + 1)
	}
}

// ensureListRowVisible scrolls the run list so the selected row follows the
// cursor past the initially-visible window. HomeView renders a header line and a
// blank separator before the rows, so run i lands on content line i+2.
func (a *App) ensureListRowVisible() {
	const headerLines = 2
	a.scrollRowIntoView(headerLines + a.selected)
}

// nodeColumn returns the display column of the left border of node key's box in
// a rendered line, or -1 if not present. The box is drawn "│ key │", so the
// border sits one cell left of the " key " pad span. Panning to this column
// keeps the whole box (border included) on screen, not just the inner text. It
// strips ANSI styling so the column is measured in visible cells, not bytes.
func nodeColumn(line, key string) int {
	plain := ansi.Strip(line)
	marker := " " + key + " "
	i := strings.Index(plain, marker)
	if i < 0 {
		return -1
	}
	border := lipgloss.Width(plain[:i]) - 1 // one cell left of the leading pad
	if border < 0 {
		border = 0
	}
	return border
}

// resize sizes the viewport to the window minus the footer keybar.
func (a *App) resize() {
	footerH := lipgloss.Height(a.footerView())
	h := a.height - footerH
	if h < 1 {
		h = 1
	}
	a.viewport.Width = a.width
	a.viewport.Height = h
}

// footerView renders the mode-aware one-line keybar.
func (a App) footerView() string {
	return a.help.View(modeHelp{keys: a.keys, mode: a.mode, detail: a.detailOpen})
}

type runsLoadedMsg struct {
	runs   []rwx.RunSummary
	cursor string
	append bool
	// refresh marks a background auto-refresh rather than a user-initiated
	// load: it replaces the page in place, keeping the cursor and scroll
	// position where the reader left them.
	refresh bool
	err     error
}

type runOpenedMsg struct {
	run rwx.Run
	err error
}

func listRunsCmd(c *rwx.Client, f rwx.ListFilter, appendPage bool) tea.Cmd {
	return func() tea.Msg {
		rl, err := c.ListRuns(context.Background(), f)
		return runsLoadedMsg{runs: rl.Runs, cursor: rl.Pagination.NextCursor, append: appendPage, err: err}
	}
}

// refreshListCmd re-fetches the first page for the auto-refresh tick.
func refreshListCmd(c *rwx.Client, f rwx.ListFilter) tea.Cmd {
	return func() tea.Msg {
		f.Cursor = "" // always the newest page; paged-in extras are re-fetched on demand
		rl, err := c.ListRuns(context.Background(), f)
		return runsLoadedMsg{runs: rl.Runs, cursor: rl.Pagination.NextCursor, refresh: true, err: err}
	}
}

type logsLoadedMsg struct {
	content string
	err     error
}

func fetchLogsCmd(c *rwx.Client, runID, taskKey string) tea.Cmd {
	return func() tea.Msg {
		s, err := c.Logs(context.Background(), runID, taskKey)
		return logsLoadedMsg{content: s, err: err}
	}
}

func openRunCmd(c *rwx.Client, id string) tea.Cmd {
	return func() tea.Msg {
		r, err := c.Results(context.Background(), id)
		return runOpenedMsg{run: r, err: err}
	}
}

// pollMsg fires on the poll interval while an in-flight run is open.
type pollMsg struct{}

// listPollMsg drives the run list's auto-refresh. It is a separate tick from
// pollMsg so the two chains never consume each other's ticks: this one runs for
// the whole session (re-arming even while a run is open, so esc back to the list
// resumes refreshing) and is the only chain the list starts.
type listPollMsg struct{}

func listPollTick(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return listPollMsg{} })
}

// listPollInterval refreshes faster while runs are still moving, and backs off
// once the whole page has settled.
func listPollInterval(runs []rwx.RunSummary) time.Duration {
	for _, r := range runs {
		if r.Status.Execution == "in_progress" || r.Status.Execution == "waiting" {
			return 5 * time.Second
		}
	}
	return 15 * time.Second
}

// runRefreshedMsg carries a poll refresh of the open run (unlike runOpenedMsg it
// preserves the cursor and scroll position).
type runRefreshedMsg struct {
	run rwx.Run
	err error
}

func refreshRunCmd(c *rwx.Client, id string) tea.Cmd {
	return func() tea.Msg {
		r, err := c.Results(context.Background(), id)
		return runRefreshedMsg{run: r, err: err}
	}
}

func pollTick(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return pollMsg{} })
}

// pollInterval backs off as fewer tasks are active.
func pollInterval(run rwx.Run) time.Duration {
	active := 0
	for _, t := range run.Tasks {
		switch t.Status.Execution {
		case "running", "ready", "waiting":
			active++
		}
	}
	switch {
	case active > 4:
		return 2 * time.Second
	case active > 0:
		return 4 * time.Second
	default:
		return 6 * time.Second
	}
}

// reloadList applies a new filter and reloads the run list from the first page.
func (a App) reloadList(f rwx.ListFilter) (tea.Model, tea.Cmd) {
	a.cfg.Filter = f
	a.nextCursor = ""
	a.err = nil
	a.mode = modeLoading
	return a, tea.Batch(listRunsCmd(a.client, f, false), a.spinner.Tick)
}

// visibleRuns is the fetched runs narrowed by the client-side view filter (a
// case-insensitive substring over title, definition path, and branch). This is
// the "view filter" tier; the fetch filter (a.cfg.Filter) decides what's loaded.
func (a *App) visibleRuns() []rwx.RunSummary {
	// Two stacked view filters: the persistent --definition scope (DefinitionPath
	// only), then the interactive type-to-filter within it.
	return FilterRunList(FilterByDefinition(a.runs, a.defFilter), a.listFilter)
}

// selectedRun returns the currently-selected visible run, or nil if none.
func (a *App) selectedRun() *rwx.RunSummary {
	vis := a.visibleRuns()
	if a.selected < 0 || a.selected >= len(vis) {
		return nil
	}
	return &vis[a.selected]
}

// selectRunByID moves the selection to the run with the given ID, so an
// auto-refresh keeps the reader on the row they were looking at even when newer
// runs push it down. Falls back to clamping when the run is gone (or filtered
// out) rather than jumping to the top.
func (a *App) selectRunByID(id string) {
	if id != "" {
		for i, r := range a.visibleRuns() {
			if r.ID == id {
				a.selected = i
				return
			}
		}
	}
	a.clampListSelection()
}

// clampListSelection keeps the selection within the visible rows after the view
// filter changes.
func (a *App) clampListSelection() {
	if n := len(a.visibleRuns()); a.selected >= n {
		a.selected = n - 1
	}
	if a.selected < 0 {
		a.selected = 0
	}
}

// listScope names the current fetch scope from the active fetch filter.
func (a *App) listScope() string { return ScopeLabel(a.cfg.Filter) }

// cycleScope advances the fetch scope (all → mine → branch → all, or reverse)
// and re-fetches. "branch" uses the selected run's branch; if there isn't one it
// is skipped so the cycle never lands on an empty branch.
func (a App) cycleScope(dir int) (tea.Model, tea.Cmd) {
	order := []string{"all", "mine", "branch"}
	cur := 0
	for i, s := range order {
		if s == a.listScope() {
			cur = i
		}
	}
	// Start from the live filter and change only what the cycle owns, rather
	// than rebuilding it from scratch: the scopes outside the cycle (the
	// --repository name, --failed's result status) have to ride along, or Tab
	// silently widens the list with nothing in the header to explain why.
	base := a.cfg.Filter
	base.Cursor = "" // a new scope always starts at page one
	for i := 1; i <= len(order); i++ {
		next := order[((cur+dir*i)%len(order)+len(order))%len(order)]
		f := base
		f.Mine, f.Branch = false, "" // the cycle's own fields, reset each step
		switch next {
		case "all":
			return a.reloadList(f)
		case "mine":
			f.Mine = true
			return a.reloadList(f)
		case "branch":
			if r := a.selectedRun(); r != nil && r.Branch != "" {
				f.Branch = r.Branch
				return a.reloadList(f)
			}
			// no branch to scope to — keep cycling past it
		}
	}
	return a, nil
}

func (a App) Init() tea.Cmd {
	var fetch tea.Cmd
	if a.cfg.Run != "" {
		fetch = openRunCmd(a.client, a.cfg.Run)
	} else {
		fetch = listRunsCmd(a.client, a.cfg.Filter, false)
	}
	// The list auto-refresh chain starts once, here, and re-arms itself for the
	// rest of the session — starting it anywhere else risks two chains ticking.
	return tea.Batch(fetch, a.spinner.Tick, listPollTick(listPollInterval(nil)))
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = m.Width, m.Height
		a.resize()
		a.refresh()
		return a, nil
	case spinner.TickMsg:
		if a.mode != modeLoading && !a.logsLoading {
			return a, nil // stop animating once loaded / logs fetched
		}
		var cmd tea.Cmd
		a.spinner, cmd = a.spinner.Update(m)
		return a, cmd
	case runsLoadedMsg:
		a.loadingMore = false
		// A background refresh must never disturb the reader. On a transient
		// fetch error keep the list that's on screen rather than blanking it,
		// and never drag the user out of a run they've opened.
		if m.refresh && (m.err != nil || a.mode != modeList) {
			a.err = m.err
			return a, nil
		}
		a.err = m.err
		switch {
		case m.append:
			a.runs = append(a.runs, m.runs...)
		case m.refresh:
			// Follow the selected run by ID: rows shift as new runs land, so
			// holding the index would slide the cursor onto a different run.
			sel := ""
			if r := a.selectedRun(); r != nil {
				sel = r.ID
			}
			a.runs = m.runs
			a.selectRunByID(sel)
		default:
			a.runs = m.runs
			a.selected = 0
		}
		a.nextCursor = m.cursor
		a.hasList = true
		a.mode = modeList
		if a.selected >= len(a.runs) {
			a.selected = 0
		}
		a.resize()
		a.refresh()
		if !m.append && !m.refresh {
			a.viewport.GotoTop()
		}
		return a, nil
	case listPollMsg:
		// Always re-arm: the chain outlives a visit to the graph view, so
		// returning to the list resumes auto-refresh with no extra plumbing.
		next := listPollTick(listPollInterval(a.runs))
		// Skip the fetch (but keep ticking) when refreshing would take something
		// away: a refresh returns page one only, so once the user has paged in
		// more runs it would silently truncate what they scrolled to.
		paged := a.cfg.Filter.Limit > 0 && len(a.runs) > a.cfg.Filter.Limit
		if a.mode != modeList || a.client == nil || a.loadingMore || paged {
			return a, next
		}
		return a, tea.Batch(refreshListCmd(a.client, a.cfg.Filter), next)
	case runOpenedMsg:
		a.err = m.err
		if m.err == nil {
			a.run = m.run
			a.graph = graph.Build(m.run)
			a.layout = graph.Layout(a.graph)
			a.selectedNode = firstNode(a.layout)
			a.mode = modeGraph
			a.seedPins() // one-time --pin application (persists across later runs)
		} else if a.hasList {
			a.mode = modeList // stay usable: drop back to the list on error
		}
		a.resize()
		a.refresh()
		a.viewport.GotoTop()
		if m.err == nil && !a.run.Completed {
			return a, pollTick(pollInterval(a.run)) // live-poll until it finishes
		}
		return a, nil
	case pollMsg:
		if a.mode == modeGraph && !a.run.Completed {
			return a, refreshRunCmd(a.client, a.run.RunID)
		}
		return a, nil
	case browserOpenedMsg:
		// Surface a failed launch instead of leaving the user staring at a TUI
		// that looks like it ignored the key.
		if m.err != nil {
			a.err = fmt.Errorf("open browser: %w", m.err)
		}
		return a, nil
	case runRefreshedMsg:
		if m.err == nil {
			a.run = m.run
			a.graph = graph.Build(m.run)
			a.layout = graph.Layout(a.graph)
			if a.graph.Node(a.selectedNode) == nil {
				a.selectedNode = firstNode(a.layout) // selection vanished; reset
			}
			a.refresh() // preserve scroll position (no GotoTop)
		}
		if a.mode == modeGraph && !a.run.Completed {
			return a, pollTick(pollInterval(a.run)) // keep polling
		}
		return a, nil
	case logsLoadedMsg:
		a.logsLoading = false
		switch {
		case m.err != nil:
			a.logsContent = "error fetching logs: " + m.err.Error()
		case strings.TrimSpace(m.content) == "":
			a.logsContent = "(no logs)"
		default:
			a.logsContent = m.content
		}
		a.refresh()
		a.viewport.GotoTop()
		return a, nil
	case tea.KeyMsg:
		model, cmd := a.handleKey(m)
		a = model.(App)
		a.refresh()
		// Keep the cursor in view: graph nav pans to the selected node, list nav
		// scrolls to the selected row.
		switch {
		case a.mode == modeGraph && !a.detailOpen:
			a.ensureSelectedVisible()
		case a.mode == modeList:
			a.ensureListRowVisible()
		}
		return a, cmd
	}
	return a, nil
}

func (a App) handleKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ctrl+c is the only unconditional quit: everywhere else "q" is a filter
	// character (type-to-filter), so quitting can't steal it.
	if k.Type == tea.KeyCtrlC {
		return a, tea.Quit
	}
	a.notice = "" // transient: any keypress clears the previous note

	switch a.mode {
	case modeList:
		// Type-to-filter, like the graph: printable keys build the view filter;
		// Tab cycles the server-side fetch scope. Actions live on non-letter keys.
		switch k.Type {
		case tea.KeyUp:
			if a.selected > 0 {
				a.selected--
			}
		case tea.KeyDown:
			if a.selected < len(a.visibleRuns())-1 {
				a.selected++
			} else if a.nextCursor != "" && !a.loadingMore {
				// At the bottom with more pages: fetch and append the next page.
				a.loadingMore = true
				f := a.cfg.Filter
				f.Cursor = a.nextCursor
				return a, listRunsCmd(a.client, f, true)
			}
		case tea.KeyHome:
			a.selected = 0
		case tea.KeyEnd:
			a.selected = len(a.visibleRuns()) - 1
		case tea.KeyEnter:
			if r := a.selectedRun(); r != nil {
				a.err = nil
				a.mode = modeLoading
				a.runUrl = r.RunUrl // carry the web URL so ctrl+o works in the graph too
				return a, tea.Batch(openRunCmd(a.client, r.ID), a.spinner.Tick)
			}
		case tea.KeyCtrlO:
			if r := a.selectedRun(); r != nil {
				if r.RunUrl != "" {
					return a, openURLCmd(a.openURL, r.RunUrl)
				}
				a.notice = "no rwx page for this run"
			}
		case tea.KeyCtrlG:
			if r := a.selectedRun(); r != nil {
				if url := commitURL(a.githubBase, r.CommitSha); url != "" {
					return a, openURLCmd(a.openURL, url)
				}
				a.notice = commitNotice(a.githubBase, r.CommitSha)
			}
		case tea.KeyTab:
			return a.cycleScope(1)
		case tea.KeyShiftTab:
			return a.cycleScope(-1)
		case tea.KeyCtrlR:
			f := a.cfg.Filter
			f.Cursor = ""
			return a.reloadList(f)
		case tea.KeyEsc:
			a.listFilter = ""
			a.clampListSelection()
		case tea.KeyBackspace:
			if r := []rune(a.listFilter); len(r) > 0 {
				a.listFilter = string(r[:len(r)-1])
				a.clampListSelection()
			}
		case tea.KeyRunes:
			a.listFilter += string(k.Runes)
			a.clampListSelection()
		}
	case modeGraph:
		// ctrl+o (cloud.rwx.com page) and ctrl+g (the run's commit on GitHub) are
		// run-scoped, so they work the same in the graph and the detail/logs pane.
		if k.Type == tea.KeyCtrlO {
			if a.runUrl != "" {
				return a, openURLCmd(a.openURL, a.runUrl)
			}
			a.notice = "no rwx page for this run"
			return a, nil
		}
		if k.Type == tea.KeyCtrlG {
			if url := commitURL(a.githubBase, a.run.CommitSha); url != "" {
				return a, openURLCmd(a.openURL, url)
			}
			a.notice = commitNotice(a.githubBase, a.run.CommitSha)
			return a, nil
		}
		// When the detail pane is open it captures Back/Logs; other keys are
		// inert until it closes.
		if a.detailOpen {
			switch {
			case key.Matches(k, a.keys.Back):
				// esc peels one layer: logs -> node detail -> graph.
				if a.logsContent != "" || a.logsLoading {
					a.logsContent = ""
					a.logsLoading = false
					a.viewport.GotoTop()
				} else {
					a.detailOpen = false
				}
				return a, nil
			case key.Matches(k, a.keys.Logs):
				if a.selectedNode != "" {
					a.logsLoading = true
					a.logsContent = ""
					return a, tea.Batch(fetchLogsCmd(a.client, a.run.RunID, a.selectedNode), a.spinner.Tick)
				}
				return a, nil
			case key.Matches(k, a.keys.Top):
				a.viewport.GotoTop()
				return a, nil
			case key.Matches(k, a.keys.Bottom):
				a.viewport.GotoBottom()
				return a, nil
			default:
				// Keyboard-scroll the pane (Up/Down/PgUp/PgDn/etc.) — the only way
				// to scroll a long detail/log pane now that mouse tracking is off.
				var cmd tea.Cmd
				a.viewport, cmd = a.viewport.Update(k)
				return a, cmd
			}
		}
		// Graph mode is type-to-filter: printable keys build the filter live and
		// the collapse machinery narrows the view. The few actions live on
		// non-letter keys so no letter is stolen from the filter.
		switch k.Type {
		case tea.KeyUp:
			a.moveSelection(-1, 0)
		case tea.KeyDown:
			a.moveSelection(1, 0)
		case tea.KeyLeft:
			a.moveSelection(0, -1)
		case tea.KeyRight:
			a.moveSelection(0, 1)
		case tea.KeyEnter:
			if a.selectedNode != "" {
				a.detailOpen = true
				a.logsContent = ""
				a.logsLoading = false
				// The detail pane is a fresh, narrow render; drop the graph's
				// horizontal pan (and start at the top) so it isn't scrolled
				// off-screen. a.xOffset is kept so closing restores the graph pan.
				a.viewport.SetXOffset(0)
				a.viewport.GotoTop()
			}
		case tea.KeySpace:
			// Toggle a pin (a forward focus edit). Snapshot first so esc can undo
			// it, then clear the finder filter to commit to the pin view.
			a.pushHistory()
			a.togglePin(a.selectedNode)
			a.graphFilter = ""
			a.clampSelectionToVisible()
		case tea.KeyEsc:
			// One uniform back-out: cancel the live finder if one's being typed,
			// otherwise pop the focus history (undo the last pin/unpin, restoring
			// its filter + cursor). Never leaves the grid.
			switch {
			case a.graphFilter != "":
				a.graphFilter = ""
				a.clampSelectionToVisible()
			case a.popHistory():
				a.clampSelectionToVisible()
			}
		case tea.KeyBackspace:
			if a.graphFilter != "" {
				r := []rune(a.graphFilter)
				a.graphFilter = string(r[:len(r)-1])
				a.clampSelectionToVisible()
			} else if a.hasList { // nothing to delete: go back to the run list
				a.err = nil
				a.mode = modeList
			}
		case tea.KeyRunes:
			a.graphFilter += string(k.Runes)
			a.clampSelectionToVisible()
		}
	}
	return a, nil
}

func (a App) View() string {
	if a.err != nil && a.mode == modeLoading {
		return theme.Failure.Render(fmt.Sprintf("error: %v", a.err)) + "\n\npress q to quit\n"
	}
	switch a.mode {
	case modeLoading:
		return a.spinner.View() + " " + theme.Faint.Render("loading…")
	case modeList, modeGraph:
		footer := a.footerView()
		switch {
		case a.err != nil:
			footer = theme.Failure.Render(fmt.Sprintf("error: %v", a.err)) + "\n" + footer
		case a.notice != "":
			footer = theme.Special.Render(a.notice) + "\n" + footer
		}
		return lipgloss.JoinVertical(lipgloss.Left, a.viewport.View(), footer)
	}
	return ""
}
