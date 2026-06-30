package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// The footer keybar is generated from the keymap, so labels can't drift from
// behavior. Verify the mode-aware short help and the ? full overlay.
func TestFooterKeybarByMode(t *testing.T) {
	a := NewApp(nil, AppConfig{})

	a.mode = modeList
	listFooter := a.footerView()
	for _, want := range []string{"open", "filter", "quit"} {
		if !strings.Contains(listFooter, want) {
			t.Errorf("list footer missing %q:\n%s", want, listFooter)
		}
	}

	a.mode = modeGraph
	graphFooter := a.footerView()
	if !strings.Contains(graphFooter, "back") {
		t.Errorf("graph footer missing %q:\n%s", "back", graphFooter)
	}

	a.showHelp = true
	full := a.footerView()
	for _, want := range []string{"refresh", "top", "bottom"} {
		if !strings.Contains(full, want) {
			t.Errorf("? overlay missing %q:\n%s", want, full)
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
	if !strings.Contains(out, "rwxtui") {
		t.Errorf("rendered view missing home header:\n%s", out)
	}
}

func TestMouseClickSelectsRow(t *testing.T) {
	a := NewApp(nil, AppConfig{})
	m, _ := a.Update(runsLoadedMsg{runs: loadRunList(t)})
	a = m.(App)
	m, _ = a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	a = m.(App)

	// Rows start after the header + blank line, so row index 2 is at Y=4.
	m, _ = a.Update(tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, Y: 4})
	a = m.(App)
	if a.selected != 2 {
		t.Errorf("click at Y=4 selected %d, want 2", a.selected)
	}

	// A wheel event must not panic and leaves selection unchanged.
	m, _ = a.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	a = m.(App)
	if a.selected != 2 {
		t.Errorf("wheel changed selection to %d, want 2", a.selected)
	}
}

func TestGraphSelectionNav(t *testing.T) {
	a := NewApp(nil, AppConfig{})
	m, _ := a.Update(runOpenedMsg{run: loadRun(t, "run_succeeded.json")})
	a = m.(App)
	m, _ = a.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	a = m.(App)

	send := func(s string) {
		m, _ := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
		a = m.(App)
	}

	// Layout layer 0 (sorted) is [code, go, ~base-image]; layer 1 is
	// [deps, ~base-config].
	if a.selectedNode != "code" {
		t.Fatalf("initial selection = %q, want code", a.selectedNode)
	}
	send("j") // down a layer
	if a.selectedNode != "deps" {
		t.Errorf("after down = %q, want deps", a.selectedNode)
	}
	send("k") // up a layer
	if a.selectedNode != "code" {
		t.Errorf("after up = %q, want code", a.selectedNode)
	}
	send("l") // right within layer
	if a.selectedNode != "go" {
		t.Errorf("after right = %q, want go", a.selectedNode)
	}
}
