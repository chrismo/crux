package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/chrismo/crux/internal/graph"
	"github.com/chrismo/crux/internal/rwx"
)

// errFetch stands in for a transient CLI/network failure.
var errFetch = errors.New("rwx runs list: boom")

func inflightRun() rwx.Run {
	return rwx.Run{
		RunID:     "r1",
		Completed: false,
		Tasks: []rwx.Task{
			{Key: "a", Status: rwx.TaskStatus{Execution: "running"}},
			{Key: "b", Status: rwx.TaskStatus{Execution: "waiting"}},
		},
	}
}

func TestLivePollerStartsOnlyForInFlight(t *testing.T) {
	a := NewApp(nil, AppConfig{})
	if _, cmd := a.Update(runOpenedMsg{run: inflightRun()}); cmd == nil {
		t.Error("in-flight run should start polling")
	}
	b := NewApp(nil, AppConfig{})
	if _, cmd := b.Update(runOpenedMsg{run: loadRun(t, "run_succeeded.json")}); cmd != nil {
		t.Error("completed run should not poll")
	}
}

func TestPollMsgRefreshesInFlight(t *testing.T) {
	a := NewApp(nil, AppConfig{})
	m, _ := a.Update(runOpenedMsg{run: inflightRun()})
	a = m.(App)
	if _, cmd := a.Update(pollMsg{}); cmd == nil {
		t.Error("pollMsg should refresh while in-flight")
	}
}

// The run list has to keep itself current without a keypress (issue #1). The
// poll tick drives that, and a background refresh must not yank the cursor off
// the row the user is on or scroll them back to the top.
func TestListAutoRefreshOnPoll(t *testing.T) {
	runs := []rwx.RunSummary{
		{ID: "r1", Title: "one"}, {ID: "r2", Title: "two"}, {ID: "r3", Title: "three"},
	}
	a := NewApp(nil, AppConfig{})
	m, _ := a.Update(runsLoadedMsg{runs: runs})
	a = m.(App)
	a.selected = 2 // sitting on r3

	// A tick while on the list schedules a re-fetch.
	m, cmd := a.Update(listPollMsg{})
	if cmd == nil {
		t.Fatal("listPollMsg in list mode should issue a refresh + reschedule")
	}
	a = m.(App)

	// A new page arrives with a run prepended: selection follows r3, not the index.
	fresh := append([]rwx.RunSummary{{ID: "r0", Title: "brand new"}}, runs...)
	m, _ = a.Update(runsLoadedMsg{runs: fresh, refresh: true})
	a = m.(App)
	if got := a.selectedRun(); got == nil || got.ID != "r3" {
		t.Errorf("selection after refresh = %v, want r3", got)
	}
	if len(a.runs) != 4 {
		t.Errorf("runs = %d, want 4 (refresh replaces the page)", len(a.runs))
	}

	// A failed background refresh must not blank the list the user is reading.
	m, _ = a.Update(runsLoadedMsg{refresh: true, err: errFetch})
	if got := len(m.(App).runs); got != 4 {
		t.Errorf("runs after failed refresh = %d, want 4 (list preserved)", got)
	}

	// The tick keeps ticking while a run is open, so esc back to the list
	// resumes auto-refresh instead of going quiet forever.
	g := openGraph(t, "run_succeeded.json")
	if _, cmd := g.Update(listPollMsg{}); cmd == nil {
		t.Error("listPollMsg in graph mode should still reschedule the tick")
	}

	// Once the user has paged in a second page, a refresh (page one only) would
	// truncate the list under them — so it re-arms without fetching.
	p := NewApp(nil, AppConfig{Filter: rwx.ListFilter{Limit: 2}})
	m, _ = p.Update(runsLoadedMsg{runs: fresh}) // 4 runs > limit 2 == paged
	if _, cmd := m.(App).Update(listPollMsg{}); cmd == nil {
		t.Error("paged list should still re-arm the tick")
	}
	if got := len(m.(App).runs); got != 4 {
		t.Errorf("paged runs = %d, want 4 left intact", got)
	}
}

// Tab cycles all/mine/branch. The scopes that sit *outside* that cycle —
// --repository and --failed — must ride through it: rebuilding the filter from
// scratch would silently drop the user back to every repo, or to every result
// status, with nothing in the header to explain why the list grew.
func TestCycleScopeKeepsOrthogonalScopes(t *testing.T) {
	a := NewApp(nil, AppConfig{Filter: rwx.ListFilter{
		Limit: 30, Repositories: []string{"crux"}, ResultStatus: "failed",
	}})
	m, _ := a.cycleScope(1)
	got := m.(App).cfg.Filter
	if len(got.Repositories) != 1 || got.Repositories[0] != "crux" {
		t.Errorf("Repositories after cycleScope = %v, want [crux]", got.Repositories)
	}
	if got.ResultStatus != "failed" {
		t.Errorf("ResultStatus after cycleScope = %q, want failed", got.ResultStatus)
	}
	// And still there after a full lap back to where it started.
	for i := 0; i < 3; i++ {
		m, _ = m.(App).cycleScope(1)
	}
	if got := m.(App).cfg.Filter; len(got.Repositories) != 1 || got.ResultStatus != "failed" {
		t.Errorf("after a full cycle = %+v, want repo crux / status failed", got)
	}
}

func TestRefreshPreservesSelection(t *testing.T) {
	a := openGraph(t, "run_succeeded.json")
	a.selectedNode = "test"
	m, _ := a.Update(runRefreshedMsg{run: loadRun(t, "run_succeeded.json")})
	a = m.(App)
	if a.selectedNode != "test" {
		t.Errorf("selection = %q, want test (preserved across refresh)", a.selectedNode)
	}
}

func TestPollIntervalBackoff(t *testing.T) {
	mk := func(n int, exec string) rwx.Run {
		r := rwx.Run{}
		for i := 0; i < n; i++ {
			r.Tasks = append(r.Tasks, rwx.Task{Status: rwx.TaskStatus{Execution: exec}})
		}
		return r
	}
	if got := pollInterval(mk(5, "running")); got != 2*time.Second {
		t.Errorf("many active = %v, want 2s", got)
	}
	if got := pollInterval(mk(1, "running")); got != 4*time.Second {
		t.Errorf("few active = %v, want 4s", got)
	}
	if got := pollInterval(mk(2, "finished")); got != 6*time.Second {
		t.Errorf("none active = %v, want 6s", got)
	}
}

// The footer keybar is generated from the keymap, so labels can't drift from
// behavior. It is mode-aware and there is no separate ? overlay.
func TestFooterKeybarByMode(t *testing.T) {
	a := NewApp(nil, AppConfig{})

	a.mode = modeList
	listFooter := a.footerView()
	for _, want := range []string{"move", "filter", "scope", "open", "web", "commit", "quit"} {
		if !strings.Contains(listFooter, want) {
			t.Errorf("list footer missing %q:\n%s", want, listFooter)
		}
	}
	// List-only actions must not leak graph bindings.
	if strings.Contains(listFooter, "pin") {
		t.Errorf("list footer should not show graph bindings:\n%s", listFooter)
	}

	a.mode = modeGraph
	graphFooter := a.footerView()
	for _, want := range []string{"back", "pin", "filter", "list", "web", "commit"} {
		if !strings.Contains(graphFooter, want) {
			t.Errorf("graph footer missing %q:\n%s", want, graphFooter)
		}
	}

	// The detail/log pane has its own keybar: scroll/logs/back only. It must not
	// advertise graph actions that do nothing there (list, pin, filter).
	a.detailOpen = true
	detailFooter := a.footerView()
	for _, want := range []string{"scroll", "logs", "back", "web", "commit"} {
		if !strings.Contains(detailFooter, want) {
			t.Errorf("detail footer missing %q:\n%s", want, detailFooter)
		}
	}
	for _, unwanted := range []string{"list", "pin", "filter"} {
		if strings.Contains(detailFooter, unwanted) {
			t.Errorf("detail footer should not show %q:\n%s", unwanted, detailFooter)
		}
	}
}

func TestResizeSizesViewportAndRenders(t *testing.T) {
	a := NewApp(nil, AppConfig{})

	// Simulate a loaded run list, then a window resize.
	m, _ := a.Update(runsLoadedMsg{runs: loadRunList(t)})
	a = m.(App)
	m, _ = a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	a = m.(App)

	if a.width != 100 || a.height != 30 {
		t.Errorf("size = %dx%d, want 100x30", a.width, a.height)
	}
	if a.viewport.Width != 100 {
		t.Errorf("viewport width = %d, want 100", a.viewport.Width)
	}
	// Viewport height is window minus the footer keybar.
	if a.viewport.Height >= 30 || a.viewport.Height < 1 {
		t.Errorf("viewport height = %d, want < 30 and >= 1", a.viewport.Height)
	}
	out := a.View()
	if !strings.Contains(out, "crux") {
		t.Errorf("rendered view missing home header:\n%s", out)
	}
}

func TestGraphSelectionNav(t *testing.T) {
	a := NewApp(nil, AppConfig{})
	m, _ := a.Update(runOpenedMsg{run: loadRun(t, "run_succeeded.json")})
	a = m.(App)
	m, _ = a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	a = m.(App)

	send := func(kt tea.KeyType) {
		m, _ := a.Update(tea.KeyMsg{Type: kt})
		a = m.(App)
	}

	// Graph nav is arrow-key only (letters type into the filter). Layout layer 0
	// (sorted) is [code, go, ~base-image]; layer 1 is [deps, ~base-config].
	if a.selectedNode != "code" {
		t.Fatalf("initial selection = %q, want code", a.selectedNode)
	}
	send(tea.KeyDown)
	if a.selectedNode != "deps" {
		t.Errorf("after down = %q, want deps", a.selectedNode)
	}
	send(tea.KeyUp)
	if a.selectedNode != "code" {
		t.Errorf("after up = %q, want code", a.selectedNode)
	}
	send(tea.KeyRight)
	if a.selectedNode != "go" {
		t.Errorf("after right = %q, want go", a.selectedNode)
	}
}

func TestListPaginationAppends(t *testing.T) {
	a := NewApp(nil, AppConfig{Filter: rwx.ListFilter{Limit: 30}})
	runs := loadRunList(t)
	m, _ := a.Update(runsLoadedMsg{runs: runs, cursor: "CURSOR"})
	a = m.(App)
	if a.nextCursor != "CURSOR" || len(a.runs) != len(runs) {
		t.Fatalf("initial page: cursor=%q n=%d", a.nextCursor, len(a.runs))
	}

	// Pressing down at the last row with a cursor requests the next page.
	a.selected = len(a.runs) - 1
	m, _ = a.Update(tea.KeyMsg{Type: tea.KeyDown})
	a = m.(App)
	if !a.loadingMore {
		t.Error("down at bottom should set loadingMore")
	}

	// The appended page grows the list and clears the cursor.
	m, _ = a.Update(runsLoadedMsg{runs: runs[:2], cursor: "", append: true})
	a = m.(App)
	if len(a.runs) != len(runs)+2 {
		t.Errorf("after append n=%d, want %d", len(a.runs), len(runs)+2)
	}
	if a.nextCursor != "" || a.loadingMore {
		t.Errorf("append should clear cursor (%q) and loadingMore (%v)", a.nextCursor, a.loadingMore)
	}
}

// Tab cycles the server-side fetch scope (all → mine → branch) with a re-fetch.
func TestListTabCyclesScope(t *testing.T) {
	a := NewApp(nil, AppConfig{Filter: rwx.ListFilter{Limit: 30}})
	m, _ := a.Update(runsLoadedMsg{runs: loadRunList(t)})
	a = m.(App)
	if a.listScope() != "all" {
		t.Fatalf("initial scope = %q, want all", a.listScope())
	}

	m, _ = a.Update(tea.KeyMsg{Type: tea.KeyTab})
	a = m.(App)
	if !a.cfg.Filter.Mine {
		t.Errorf("Tab should cycle to mine: %+v", a.cfg.Filter)
	}
	if a.mode != modeLoading {
		t.Error("scope change should trigger a reload (modeLoading)")
	}
}

// Typing narrows the run list client-side; esc clears it.
func TestListTypeToFilter(t *testing.T) {
	a := NewApp(nil, AppConfig{Filter: rwx.ListFilter{Limit: 30}})
	m, _ := a.Update(runsLoadedMsg{runs: loadRunList(t)})
	a = m.(App)
	total := len(a.runs)

	pressRunes(&a, "prime") // matches only the prime-cache run's definition
	if got := len(a.visibleRuns()); got == 0 || got >= total {
		t.Errorf("filter 'prime' should narrow rows: %d of %d", got, total)
	}
	if a.listFilter != "prime" {
		t.Errorf("listFilter = %q, want prime", a.listFilter)
	}

	sendType(&a, tea.KeyEsc) // clears the filter
	if a.listFilter != "" {
		t.Errorf("esc should clear the list filter, got %q", a.listFilter)
	}
	if len(a.visibleRuns()) != total {
		t.Errorf("cleared filter should show all %d rows", total)
	}
}

func openGraph(t *testing.T, fixture string) App {
	t.Helper()
	a := NewApp(nil, AppConfig{})
	m, _ := a.Update(runOpenedMsg{run: loadRun(t, fixture)})
	a = m.(App)
	m, _ = a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return m.(App)
}

func TestGraphPinToggle(t *testing.T) {
	a := openGraph(t, "run_failed.json")
	if a.selectedNode != "code" {
		t.Fatalf("initial selection = %q, want code", a.selectedNode)
	}

	pin := func() {
		m, _ := a.Update(tea.KeyMsg{Type: tea.KeySpace})
		a = m.(App)
	}

	pin() // pin code's cone (space)
	if len(a.pins) != 1 || a.pins[0].key != "code" {
		t.Fatalf("pins = %v, want [code]", a.pins)
	}
	fs := a.focusSet()
	if !fs["code"] || !fs["deps"] {
		t.Errorf("focus cone should include code's cone: %v", fs)
	}
	if fs["go"] {
		t.Errorf("focus cone should exclude go (unrelated): %v", fs)
	}

	pin() // toggle the same node off (unpin)
	if len(a.pins) != 0 {
		t.Errorf("pins not cleared on second space: %v", a.pins)
	}
	if a.focusSet() != nil {
		t.Errorf("focusSet should be nil with no pins")
	}
}

// A second pin narrows the view to the intersection of cones: pinning a node
// with several parents, then one of those parents, drops the sibling parents.
func TestPinsIntersect(t *testing.T) {
	run := loadRun(t, "sample_dag_failed.json")
	g := graph.Build(run)
	a := &App{graph: g, layout: graph.Layout(g)}

	// integration has three parents: build-api, build-worker, build-web.
	a.togglePin("integration")
	fs := a.focusSet()
	for _, p := range []string{"build-api", "build-worker", "build-web"} {
		if !fs[p] {
			t.Fatalf("cone(integration) should include parent %s: %v", p, fs)
		}
	}

	// build-api is already visible, so pinning it refines (intersects) — the
	// sibling parents drop out.
	a.togglePin("build-api")
	if len(a.pins) != 2 || a.pins[1].refine != true {
		t.Fatalf("second pin of a visible node should refine: %+v", a.pins)
	}
	fs = a.focusSet()
	if !fs["integration"] || !fs["build-api"] {
		t.Errorf("pinned anchors must stay visible: %v", fs)
	}
	if fs["build-worker"] || fs["build-web"] {
		t.Errorf("intersection should hide sibling parents build-worker/build-web: %v", fs)
	}
}

// Pinning a node from outside the current pin view (found via the global
// filter) adds it via union — both cones stay visible.
func TestPinsUnionWhenAddedFromElsewhere(t *testing.T) {
	run := loadRun(t, "sample_dag_failed.json")
	g := graph.Build(run)
	a := &App{graph: g, layout: graph.Layout(g)}

	a.togglePin("go-deps") // Go branch
	if a.focusSet()["node-deps"] {
		t.Fatal("precondition: node-deps should be outside go-deps' cone")
	}
	a.togglePin("node-deps") // outside the current view → union (add)
	if len(a.pins) != 2 || a.pins[1].refine != false {
		t.Fatalf("pinning a node outside the view should add (union): %+v", a.pins)
	}
	fs := a.focusSet()
	if !fs["build-api"] || !fs["build-web"] {
		t.Errorf("union should keep both cones (build-api + build-web): %v", fs)
	}
}

// The filter is a global finder: while active it searches the whole graph,
// overriding the pin view, and pinning clears it (snapping back to the pins).
func TestFilterFindsOutsidePinsThenPinClearsIt(t *testing.T) {
	a := openGraph(t, "sample_dag_failed.json")
	a.togglePin("go-deps") // pin the Go branch

	// node-deps is outside go-deps' cone; the global filter still finds it.
	a.graphFilter = "node-deps"
	vis := computeVisible(a.graph, a.currentOverlay())
	if !vis["node-deps"] {
		t.Fatalf("active filter should find node-deps globally: %v", vis)
	}

	// Pin it: the filter clears and the view returns to the pins (now unioned).
	a.selectedNode = "node-deps"
	m, _ := a.Update(tea.KeyMsg{Type: tea.KeySpace})
	a = m.(App)
	if a.graphFilter != "" {
		t.Errorf("pinning should clear the filter, got %q", a.graphFilter)
	}
	fs := a.focusSet()
	if !fs["go-deps"] || !fs["node-deps"] {
		t.Errorf("both pins should be visible after adding from the finder: %v", fs)
	}
}

// Pins persist across trips out to the run list and into another run — so an
// elaborate pin set survives navigating between runs.
func TestPinsPersistAcrossRuns(t *testing.T) {
	a := NewApp(nil, AppConfig{})
	a.hasList = true
	open := func(fixture string) {
		m, _ := a.Update(runOpenedMsg{run: loadRun(t, fixture)})
		a = m.(App)
		m, _ = a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
		a = m.(App)
	}

	open("run_failed.json")
	m, _ := a.Update(tea.KeyMsg{Type: tea.KeySpace}) // pin the selected node
	a = m.(App)
	if len(a.pins) != 1 {
		t.Fatalf("expected 1 pin, got %v", a.pins)
	}
	pinned := a.pins[0].key

	// backspace returns to the run list; pins survive.
	m, _ = a.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	a = m.(App)
	if a.mode != modeList {
		t.Fatalf("backspace should return to the list, mode=%v", a.mode)
	}
	if len(a.pins) != 1 || a.pins[0].key != pinned {
		t.Fatalf("pins should persist at the list, got %v", a.pins)
	}

	// Opening another run keeps the pins.
	open("run_succeeded.json")
	if len(a.pins) != 1 || a.pins[0].key != pinned {
		t.Fatalf("pins should persist into the next run, got %v", a.pins)
	}
}

// --pin seeds substring-matched pins once, on the first run opened.
func TestSeedPinsFromConfig(t *testing.T) {
	a := NewApp(nil, AppConfig{Pins: []string{"deps"}})
	m, _ := a.Update(runOpenedMsg{run: loadRun(t, "sample_dag_failed.json")})
	a = m.(App)
	m, _ = a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	a = m.(App)

	// "deps" matches go-deps, node-deps, py-deps — all pinned.
	got := map[string]bool{}
	for _, p := range a.pins {
		got[p.key] = true
	}
	for _, want := range []string{"go-deps", "node-deps", "py-deps"} {
		if !got[want] {
			t.Errorf("--pin deps should have pinned %s; pins=%v", want, a.pins)
		}
	}
	if !a.pinsSeeded {
		t.Error("pinsSeeded should be set after the first run open")
	}
}

func pressRunes(a *App, s string) {
	m, _ := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
	*a = m.(App)
}

func sendType(a *App, kt tea.KeyType) {
	m, _ := a.Update(tea.KeyMsg{Type: kt})
	*a = m.(App)
}

// esc undoes the last focus action via the history stack: pinning from a filter,
// then esc, pops the pre-pin snapshot — restoring the filter and dropping the pin.
func TestEscUndoesPinAndRestoresFilter(t *testing.T) {
	a := openGraph(t, "sample_dag_failed.json")

	pressRunes(&a, "build")
	if a.graphFilter != "build" {
		t.Fatalf("filter = %q, want build", a.graphFilter)
	}

	sendType(&a, tea.KeySpace) // pin; filter clears to the pin view
	if len(a.pins) != 1 || a.graphFilter != "" {
		t.Fatalf("after pin: pins=%v filter=%q", a.pins, a.graphFilter)
	}

	sendType(&a, tea.KeyEsc) // undo the pin
	if len(a.pins) != 0 {
		t.Fatalf("esc should undo the pin, pins=%v", a.pins)
	}
	if a.graphFilter != "build" {
		t.Errorf("esc should restore the filter, got %q", a.graphFilter)
	}
}

// space is a pure toggle: unpinning one of several pins just removes it and
// never resurrects a filter (that would override the surviving pins). Only esc
// (history) brings filters back.
func TestUnpinWithOtherPinsKeepsView(t *testing.T) {
	a := openGraph(t, "sample_dag_failed.json")

	pressRunes(&a, "web") // filter to *-web nodes
	a.selectedNode = "lint-web"
	sendType(&a, tea.KeySpace) // pin lint-web (filter clears)
	a.selectedNode = "node-deps"
	sendType(&a, tea.KeySpace) // pin node-deps
	if len(a.pins) != 2 {
		t.Fatalf("expected 2 pins, got %v", a.pins)
	}

	// Unpin lint-web with node-deps still pinned: no filter must come back.
	a.selectedNode = "lint-web"
	sendType(&a, tea.KeySpace)
	if len(a.pins) != 1 || a.pins[0].key != "node-deps" {
		t.Fatalf("expected only node-deps pinned, got %v", a.pins)
	}
	if a.graphFilter != "" {
		t.Errorf("unpin must not resurrect a filter, got %q", a.graphFilter)
	}
	if vis := computeVisible(a.graph, a.currentOverlay()); !vis["node-deps"] {
		t.Errorf("node-deps should still be visible in its pin view: %v", vis)
	}
}

// esc with a live filter cancels the finder (not a history step); a second esc
// then walks the focus history.
func TestEscCancelsLiveFilterBeforeHistory(t *testing.T) {
	a := openGraph(t, "sample_dag_failed.json")

	a.selectedNode = "go-deps"
	sendType(&a, tea.KeySpace) // pin go-deps (history has the pre-pin snapshot)
	pressRunes(&a, "web")      // start a new finder search
	if a.graphFilter != "web" {
		t.Fatalf("filter = %q, want web", a.graphFilter)
	}

	sendType(&a, tea.KeyEsc) // cancels the live filter, leaves the pin
	if a.graphFilter != "" {
		t.Errorf("first esc should clear the live filter, got %q", a.graphFilter)
	}
	if len(a.pins) != 1 {
		t.Fatalf("first esc should not touch pins, got %v", a.pins)
	}

	sendType(&a, tea.KeyEsc) // now pops history: undoes the pin
	if len(a.pins) != 0 {
		t.Errorf("second esc should undo the pin, got %v", a.pins)
	}
}

// Pins accumulate, and esc pops only the most recent one (not all of them).
func TestPinsAccumulateAndEscPopsLast(t *testing.T) {
	a := openGraph(t, "run_failed.json")
	pin := func() {
		m, _ := a.Update(tea.KeyMsg{Type: tea.KeySpace})
		a = m.(App)
	}
	esc := func() {
		m, _ := a.Update(tea.KeyMsg{Type: tea.KeyEsc})
		a = m.(App)
	}

	pin()
	first := a.selectedNode
	a.moveSelection(1, 0) // move to a different visible node
	second := a.selectedNode
	if second == first {
		t.Fatalf("expected selection to move off %q for a distinct second pin", first)
	}
	pin()
	if len(a.pins) != 2 {
		t.Fatalf("expected 2 accumulated pins, got %v", a.pins)
	}

	esc() // pops only the last pin
	if len(a.pins) != 1 || a.pins[0].key != first {
		t.Fatalf("esc should pop the last pin, leaving [%s], got %v", first, a.pins)
	}
	esc() // pops the remaining pin
	if len(a.pins) != 0 {
		t.Fatalf("esc should pop the remaining pin, got %v", a.pins)
	}
	esc() // nothing left: must NOT leave the grid
	if a.mode != modeGraph {
		t.Errorf("esc with nothing to dismiss should stay in the grid, mode=%v", a.mode)
	}
}

func TestDetailPaneAndLogs(t *testing.T) {
	a := openGraph(t, "run_failed.json") // selection starts on "code"

	// enter opens the detail pane for the selected node.
	m, _ := a.Update(tea.KeyMsg{Type: tea.KeyEnter})
	a = m.(App)
	if !a.detailOpen {
		t.Fatal("enter did not open the detail pane")
	}
	if !strings.Contains(a.bodyContent(), "status:") {
		t.Errorf("detail body missing status:\n%s", a.bodyContent())
	}

	// A logs result replaces the body with the log content.
	m, _ = a.Update(logsLoadedMsg{content: "line one\nline two"})
	a = m.(App)
	if !strings.Contains(a.bodyContent(), "line two") {
		t.Errorf("logs not shown in body:\n%s", a.bodyContent())
	}

	// esc peels one layer: from logs back to the node detail (pane stays open).
	m, _ = a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a = m.(App)
	if !a.detailOpen || a.logsContent != "" {
		t.Errorf("esc from logs should return to detail (open=%v logs=%q)", a.detailOpen, a.logsContent)
	}
	if !strings.Contains(a.bodyContent(), "status:") {
		t.Errorf("after esc from logs, detail should show:\n%s", a.bodyContent())
	}

	// A second esc closes the detail pane back to the graph.
	m, _ = a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a = m.(App)
	if a.detailOpen {
		t.Errorf("second esc should close the detail pane")
	}
}

// The detail/log pane must scroll by keyboard (the only scroll mechanism now
// that mouse tracking is off): with a long log open, Down moves the viewport.
func TestLogPaneKeyboardScroll(t *testing.T) {
	a := openGraph(t, "run_failed.json")
	m, _ := a.Update(tea.KeyMsg{Type: tea.KeyEnter}) // open detail
	a = m.(App)
	m, _ = a.Update(logsLoadedMsg{content: strings.Repeat("log line\n", 200)})
	a = m.(App)

	before := a.viewport.YOffset
	m, _ = a.Update(tea.KeyMsg{Type: tea.KeyDown})
	a = m.(App)
	if a.viewport.YOffset <= before {
		t.Errorf("Down should scroll the log pane, YOffset %d -> %d", before, a.viewport.YOffset)
	}
}

// Graph mode is type-to-filter: printable keys build the filter live (no /),
// backspace deletes, and esc clears it.
func TestGraphFilterTyping(t *testing.T) {
	a := openGraph(t, "run_succeeded.json")

	press := func(s string) {
		m, _ := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
		a = m.(App)
	}

	press("v")
	press("e")
	press("t")
	if got := a.graphFilter; got != "vet" {
		t.Errorf("filter value = %q, want vet", got)
	}

	// backspace deletes the last character.
	m, _ := a.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	a = m.(App)
	if got := a.graphFilter; got != "ve" {
		t.Errorf("backspace should delete last char, got %q", got)
	}

	// esc clears the filter entirely.
	m, _ = a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a = m.(App)
	if a.graphFilter != "" {
		t.Errorf("esc should clear the filter, got %q", a.graphFilter)
	}
}

// nodeColumn locates the box's LEFT BORDER, not the inner text, so panning left
// keeps the whole box on screen (regression: only the text, not the border, was
// brought into view).
func TestNodeColumnReturnsBoxLeftBorder(t *testing.T) {
	line := "───│ build │" // 3 connector cells, then the box "│ build │"
	if got := nodeColumn(line, "build"); got != 3 {
		t.Errorf("nodeColumn = %d, want 3 (the box's left border '│')", got)
	}
	if got := nodeColumn(line, "absent"); got != -1 {
		t.Errorf("nodeColumn(absent) = %d, want -1", got)
	}
}

// Opening the detail pane drops the graph's horizontal pan so the narrow detail
// render isn't left off-screen (regression: it inherited the graph's xOffset).
func TestOpeningDetailResetsHorizontalScroll(t *testing.T) {
	a := openGraph(t, "sample_dag_failed.json") // wide graph, window 120x40
	a.viewport.SetXOffset(60)                   // simulate a far-right pan

	m, _ := a.Update(tea.KeyMsg{Type: tea.KeyEnter}) // open detail on selection
	a = m.(App)
	if !a.detailOpen {
		t.Fatal("enter should open the detail pane")
	}
	// With a stale xOffset the viewport cuts the left off; the task key (the
	// detail header) would be missing.
	if view := a.viewport.View(); !strings.Contains(view, a.selectedNode) {
		t.Errorf("detail pane scrolled off — %q not visible:\n%s", a.selectedNode, view)
	}
}

// ctrl+o on a list row opens that run's cloud.rwx.com page.
func TestCtrlOOpensRunUrlFromList(t *testing.T) {
	var opened string
	a := NewApp(nil, AppConfig{Filter: rwx.ListFilter{Limit: 30}})
	a.openURL = func(u string) error { opened = u; return nil }
	m, _ := a.Update(runsLoadedMsg{runs: loadRunList(t)})
	a = m.(App)

	_, cmd := a.Update(tea.KeyMsg{Type: tea.KeyCtrlO}) // selection defaults to row 0
	if cmd == nil {
		t.Fatal("ctrl+o should return a command to open the browser")
	}
	cmd() // run the fire-and-forget opener
	if want := loadRunList(t)[0].RunUrl; opened != want {
		t.Errorf("opened %q, want %q", opened, want)
	}
}

// Opening a run from the list carries its RunUrl so ctrl+o works in the graph
// too (the rwx results payload has no URL of its own).
func TestOpeningRunFromListCarriesRunUrl(t *testing.T) {
	a := NewApp(nil, AppConfig{Filter: rwx.ListFilter{Limit: 30}})
	m, _ := a.Update(runsLoadedMsg{runs: loadRunList(t)})
	a = m.(App)

	m, _ = a.Update(tea.KeyMsg{Type: tea.KeyEnter}) // open row 0
	a = m.(App)
	if want := loadRunList(t)[0].RunUrl; a.runUrl != want {
		t.Errorf("runUrl = %q, want %q", a.runUrl, want)
	}
}

// In the graph, ctrl+o opens the carried run URL.
func TestCtrlOOpensRunUrlFromGraph(t *testing.T) {
	var opened string
	a := openGraph(t, "sample_dag_failed.json")
	a.openURL = func(u string) error { opened = u; return nil }
	a.runUrl = "https://cloud.rwx.com/mint/clabs/runs/abc"

	_, cmd := a.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	if cmd == nil {
		t.Fatal("ctrl+o should open the browser in graph mode")
	}
	cmd()
	if opened != a.runUrl {
		t.Errorf("graph ctrl+o opened %q, want %q", opened, a.runUrl)
	}
}

// ctrl+g on a list row opens that run's commit on GitHub (base from the git
// remote, sha from the run).
func TestCtrlGOpensCommitFromList(t *testing.T) {
	var opened string
	a := NewApp(nil, AppConfig{Filter: rwx.ListFilter{Limit: 30}, GithubBase: "https://github.com/chrismo/crux"})
	a.openURL = func(u string) error { opened = u; return nil }
	m, _ := a.Update(runsLoadedMsg{runs: []rwx.RunSummary{{ID: "r1", CommitSha: "abc123"}}})
	a = m.(App)

	_, cmd := a.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	if cmd == nil {
		t.Fatal("ctrl+g should open the commit page")
	}
	cmd()
	if want := "https://github.com/chrismo/crux/commit/abc123"; opened != want {
		t.Errorf("opened %q, want %q", opened, want)
	}
}

// In the graph, ctrl+g uses the open run's own commit sha.
func TestCtrlGOpensCommitFromGraph(t *testing.T) {
	var opened string
	a := openGraph(t, "sample_dag_failed.json")
	a.openURL = func(u string) error { opened = u; return nil }
	a.githubBase = "https://github.com/chrismo/crux"
	a.run.CommitSha = "def456"

	_, cmd := a.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	if cmd == nil {
		t.Fatal("ctrl+g should open the commit page in graph mode")
	}
	cmd()
	if want := "https://github.com/chrismo/crux/commit/def456"; opened != want {
		t.Errorf("opened %q, want %q", opened, want)
	}
}

// ctrl+g no-ops (no command) when there's no GitHub base, and explains why via a
// footer notice instead of silently doing nothing.
func TestCtrlGNoopWithoutGithubBase(t *testing.T) {
	a := NewApp(nil, AppConfig{Filter: rwx.ListFilter{Limit: 30}}) // no GithubBase
	m, _ := a.Update(runsLoadedMsg{runs: []rwx.RunSummary{{ID: "r1", CommitSha: "abc123"}}})
	a = m.(App)
	m, cmd := a.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	a = m.(App)
	if cmd != nil {
		t.Error("ctrl+g with no GitHub base should not return a command")
	}
	if a.notice == "" {
		t.Error("ctrl+g with nothing to open should set an explanatory notice")
	}
}

// A run with no commit (CLI-triggered) sets the "no commit" notice, not silence.
func TestCtrlGNoticeWhenRunHasNoCommit(t *testing.T) {
	a := NewApp(nil, AppConfig{Filter: rwx.ListFilter{Limit: 30}, GithubBase: "https://github.com/chrismo/crux"})
	m, _ := a.Update(runsLoadedMsg{runs: []rwx.RunSummary{{ID: "r1", CommitSha: ""}}})
	a = m.(App)
	m, cmd := a.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	a = m.(App)
	if cmd != nil {
		t.Error("ctrl+g on a run with no commit should not return a command")
	}
	if !strings.Contains(a.notice, "commit") {
		t.Errorf("expected a 'no commit' notice, got %q", a.notice)
	}
}

// The notice is transient: the next keypress clears it.
func TestNoticeClearsOnNextKey(t *testing.T) {
	a := NewApp(nil, AppConfig{Filter: rwx.ListFilter{Limit: 30}, GithubBase: "https://github.com/chrismo/crux"})
	m, _ := a.Update(runsLoadedMsg{runs: []rwx.RunSummary{{ID: "r1", CommitSha: ""}}})
	a = m.(App)
	m, _ = a.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	a = m.(App)
	if a.notice == "" {
		t.Fatal("precondition: ctrl+g should have set a notice")
	}
	m, _ = a.Update(tea.KeyMsg{Type: tea.KeyDown})
	a = m.(App)
	if a.notice != "" {
		t.Errorf("notice should clear on the next key, still %q", a.notice)
	}
}

// ctrl+o is a no-op (no command) when there's no URL to open.
func TestCtrlONoopWithoutUrl(t *testing.T) {
	a := openGraph(t, "sample_dag_failed.json") // opened via runOpenedMsg: no runUrl carried
	if a.runUrl != "" {
		t.Fatalf("precondition: runUrl should be empty, got %q", a.runUrl)
	}
	if _, cmd := a.Update(tea.KeyMsg{Type: tea.KeyCtrlO}); cmd != nil {
		t.Error("ctrl+o with no URL should not return a command")
	}
}

// --definition sets a persistent definition-path scope: it narrows the list,
// typing narrows further WITHIN it, and esc clears the typed filter but keeps
// the scope.
func TestDefinitionScopeNarrowsList(t *testing.T) {
	a := NewApp(nil, AppConfig{Filter: rwx.ListFilter{Limit: 30}, DefinitionFilter: "prime"})
	m, _ := a.Update(runsLoadedMsg{runs: loadRunList(t)})
	a = m.(App)
	if got := len(a.visibleRuns()); got != 1 {
		t.Fatalf("def scope 'prime' should show 1 run, got %d of %d", got, len(a.runs))
	}

	pressRunes(&a, "zzz") // narrows within the def scope; matches nothing
	if got := len(a.visibleRuns()); got != 0 {
		t.Errorf("typing should stack on the def scope, got %d", got)
	}

	sendType(&a, tea.KeyEsc) // clears the typed filter, keeps the def scope
	if a.defFilter != "prime" {
		t.Errorf("esc should keep the def scope, got %q", a.defFilter)
	}
	if got := len(a.visibleRuns()); got != 1 {
		t.Errorf("after esc the def scope should still show 1 run, got %d", got)
	}
}

// The run list scrolls to keep the selection visible past the initially-shown
// window (regression: list nav never followed the cursor, so only the top rows
// that fit were reachable).
func TestListScrollsToFollowSelection(t *testing.T) {
	a := NewApp(nil, AppConfig{Filter: rwx.ListFilter{Limit: 30}})
	m, _ := a.Update(runsLoadedMsg{runs: loadRunList(t)})
	a = m.(App)
	m, _ = a.Update(tea.WindowSizeMsg{Width: 100, Height: 6}) // only a few rows fit
	a = m.(App)
	if a.viewport.YOffset != 0 {
		t.Fatalf("precondition: YOffset should start at 0, got %d", a.viewport.YOffset)
	}

	for i := 0; i < len(a.runs)-1; i++ { // move to the last row
		m, _ = a.Update(tea.KeyMsg{Type: tea.KeyDown})
		a = m.(App)
	}
	if a.viewport.YOffset == 0 {
		t.Errorf("list did not scroll to follow selection; YOffset still 0 with %d runs in a height-6 viewport", len(a.runs))
	}
}
