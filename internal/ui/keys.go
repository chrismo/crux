package ui

import "github.com/charmbracelet/bubbles/key"

// keyMap holds the key bindings whose help labels the footer keybar renders (via
// bubbles/help). Movement/typing are matched by raw key type in handleKey and
// labeled inline in ShortHelp, so only the action keys that appear in the keybar
// live here.
type keyMap struct {
	Enter  key.Binding
	Back   key.Binding
	ToList key.Binding
	Quit   key.Binding
	Pin    key.Binding
	Logs   key.Binding
	Top    key.Binding
	Bottom key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Enter:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
		Back:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		ToList: key.NewBinding(key.WithKeys("backspace"), key.WithHelp("⌫", "list")),
		Quit:   key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Pin:    key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "pin")),
		Logs:   key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "logs")),
		Top:    key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "top")),
		Bottom: key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "bottom")),
	}
}

// modeHelp adapts the keymap to bubbles/help's KeyMap interface, tailoring the
// always-visible one-line keybar to the current mode. (There is no separate ?
// overlay — the keybar shows everything that applies.)
type modeHelp struct {
	keys   keyMap
	mode   appMode
	detail bool // graph detail/log pane open (a sub-state of modeGraph)
}

func (h modeHelp) ShortHelp() []key.Binding {
	// The graph detail/log pane only scrolls and peels back — none of the graph
	// nav/filter/pin/list keys apply, so it gets its own keybar.
	if h.mode == modeGraph && h.detail {
		return []key.Binding{
			key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑↓", "scroll")),
			h.keys.Logs,
			h.keys.Back,
			key.NewBinding(key.WithKeys("ctrl+o"), key.WithHelp("^o", "web")),
			key.NewBinding(key.WithKeys("ctrl+g"), key.WithHelp("^g", "commit")),
			key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("^C", "quit")),
		}
	}
	switch h.mode {
	case modeList:
		// Type-to-filter like the graph; Tab cycles the server-side fetch scope.
		return []key.Binding{
			key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑↓", "move")),
			key.NewBinding(key.WithKeys("runes"), key.WithHelp("type", "filter")),
			key.NewBinding(key.WithKeys("tab"), key.WithHelp("⇥", "scope")),
			h.keys.Enter,
			key.NewBinding(key.WithKeys("ctrl+o"), key.WithHelp("^o", "web")),
			key.NewBinding(key.WithKeys("ctrl+g"), key.WithHelp("^g", "commit")),
			key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("^R", "refresh")),
			key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("^C", "quit")),
		}
	case modeGraph:
		// Graph mode is type-to-filter: printable keys build the filter live, so
		// these labels describe the non-letter actions plus the typing hint.
		return []key.Binding{
			key.NewBinding(key.WithKeys("up", "down", "left", "right"), key.WithHelp("↑↓←→", "move")),
			key.NewBinding(key.WithKeys("runes"), key.WithHelp("type", "filter")),
			h.keys.Pin,
			h.keys.Enter,
			h.keys.Back,
			h.keys.ToList,
			key.NewBinding(key.WithKeys("ctrl+o"), key.WithHelp("^o", "web")),
			key.NewBinding(key.WithKeys("ctrl+g"), key.WithHelp("^g", "commit")),
			key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("^C", "quit")),
		}
	default:
		return []key.Binding{h.keys.Quit}
	}
}

// FullHelp is required by help.KeyMap but unused (no overlay); it mirrors
// ShortHelp so nothing is lost if a caller ever renders it.
func (h modeHelp) FullHelp() [][]key.Binding {
	return [][]key.Binding{h.ShortHelp()}
}
