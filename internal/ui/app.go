package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/chrismo/rwx-tui/internal/graph"
	"github.com/chrismo/rwx-tui/internal/rwx"
)

// Model is the root Bubble Tea model for viewing a single run's graph.
type Model struct {
	run    rwx.Run
	graph  *graph.Graph
	layout *graph.LayoutData
}

// NewModel builds the root model from a fetched run.
func NewModel(run rwx.Run) Model {
	g := graph.Build(run)
	return Model{run: run, graph: g, layout: graph.Layout(g)}
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
	return Screen(m.run, m.graph, m.layout)
}

// Screen renders the full view (header, graph, legend) as a string. Pure, so it
// backs both View() and the headless --print path.
func Screen(run rwx.Run, g *graph.Graph, l *graph.LayoutData) string {
	var b strings.Builder

	status := run.ResultStatus
	if !run.Completed {
		status = "in progress"
	}
	header := fmt.Sprintf("RWX run %s · %s · %s", short(run.RunID), run.DefinitionPath, status)
	b.WriteString(lipgloss.NewStyle().Bold(true).Render(header))
	b.WriteString("\n")

	cp := graph.CriticalPath(g)
	if line := CriticalPathLine(cp); line != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("5")).Render(line))
		b.WriteString("\n")
	}

	fi := graph.AnalyzeFailures(g)
	if line := FailureLine(fi); line != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render(line))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	b.WriteString(RenderGraph(g, l, RenderOpts{Crit: cp, Failure: fi}))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Faint(true).Render(Legend()))
	b.WriteString("\n")
	return b.String()
}

func short(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
