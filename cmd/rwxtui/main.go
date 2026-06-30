// Command rwxtui is a local TUI for monitoring RWX runs with a better Flow
// dependency-graph viewer. This is the skeleton: flag parsing plus a Bubble Tea
// placeholder. The data layer and graph viewer land in subsequent steps.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// options holds the parsed command-line configuration.
type options struct {
	branch     string // branch to resolve a run for (default: current git branch)
	definition string // .rwx definition path, required when a branch has several
	run        string // explicit run ID to open, bypassing resolution
	dir        string // checkout dir for the static-YAML fallback (default: cwd)
}

func parseFlags(args []string) (options, error) {
	fs := flag.NewFlagSet("rwxtui", flag.ContinueOnError)
	var o options
	fs.StringVar(&o.branch, "branch", "", "branch to resolve a run for (default: current git branch)")
	fs.StringVar(&o.definition, "definition", "", "RWX definition path (required when a branch has multiple)")
	fs.StringVar(&o.run, "run", "", "explicit run ID to open")
	fs.StringVar(&o.dir, "dir", ".", "checkout directory for the static-YAML fallback")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	return o, nil
}

type model struct {
	opts options
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() string {
	var b strings.Builder
	b.WriteString("rwxtui — RWX Flow viewer (skeleton)\n\n")
	b.WriteString(fmt.Sprintf("  branch:     %s\n", orDefault(m.opts.branch, "(current)")))
	b.WriteString(fmt.Sprintf("  definition: %s\n", orDefault(m.opts.definition, "(auto)")))
	b.WriteString(fmt.Sprintf("  run:        %s\n", orDefault(m.opts.run, "(resolve)")))
	b.WriteString(fmt.Sprintf("  dir:        %s\n", m.opts.dir))
	b.WriteString("\n  press q to quit\n")
	return b.String()
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func main() {
	opts, err := parseFlags(os.Args[1:])
	if err != nil {
		os.Exit(2)
	}
	if _, err := tea.NewProgram(model{opts: opts}).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "rwxtui:", err)
		os.Exit(1)
	}
}
