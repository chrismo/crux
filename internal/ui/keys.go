package ui

import "github.com/charmbracelet/bubbles/key"

// keyMap is the single source of truth for key bindings. The footer keybar is
// generated from it (via bubbles/help), so the labels can't drift from behavior.
type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Left    key.Binding
	Right   key.Binding
	Enter   key.Binding
	Back    key.Binding
	Quit    key.Binding
	Filter  key.Binding
	Isolate key.Binding
	Logs    key.Binding
	Refresh key.Binding
	Top     key.Binding
	Bottom  key.Binding
	All     key.Binding
	Mine    key.Binding
	Branch  key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Left:    key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "left")),
		Right:   key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "right")),
		Enter:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
		Back:    key.NewBinding(key.WithKeys("esc", "backspace"), key.WithHelp("esc", "back")),
		Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Filter:  key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter (collapse)")),
		Isolate: key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "isolate (collapse)")),
		Logs:    key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "logs")),
		Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		Top:     key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "top")),
		Bottom:  key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "bottom")),
		All:     key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "all")),
		Mine:    key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "mine")),
		Branch:  key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "branch")),
	}
}

// modeHelp adapts the keymap to bubbles/help's KeyMap interface, tailoring the
// always-visible one-line keybar to the current mode. (There is no separate ?
// overlay — the keybar shows everything that applies.)
type modeHelp struct {
	keys keyMap
	mode appMode
}

func (h modeHelp) ShortHelp() []key.Binding {
	switch h.mode {
	case modeList:
		return []key.Binding{h.keys.Up, h.keys.Down, h.keys.Enter, h.keys.All, h.keys.Mine, h.keys.Branch, h.keys.Refresh, h.keys.Quit}
	case modeGraph:
		return []key.Binding{h.keys.Up, h.keys.Down, h.keys.Left, h.keys.Right, h.keys.Enter, h.keys.Isolate, h.keys.Filter, h.keys.Back, h.keys.Quit}
	default:
		return []key.Binding{h.keys.Quit}
	}
}

// FullHelp is required by help.KeyMap but unused (no overlay); it mirrors
// ShortHelp so nothing is lost if a caller ever renders it.
func (h modeHelp) FullHelp() [][]key.Binding {
	return [][]key.Binding{h.ShortHelp()}
}
