// Package monitor implements the --monitor live-log TUI using bubbletea.
package monitor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	logFile       = ".ralph/ralph.log"
	iterationFile = ".ralph/iteration.txt"
	maxLines      = 50
	pollInterval  = 500 * time.Millisecond
)

type model struct {
	lines    []string
	iter     string
	logPath  string
	iterPath string
	offset   int64 // bytes already read
	err      error
}

type tickMsg time.Time
type linesMsg struct {
	lines []string
	iter  string
	pos   int64
}

func (m model) Init() tea.Cmd {
	return tea.Batch(tick(), loadLines(m.logPath, m.iterPath, 0))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC || msg.String() == "q" {
			return m, tea.Quit
		}
	case linesMsg:
		m.lines = msg.lines
		m.iter = msg.iter
		m.offset = msg.pos
		return m, tick()
	case tickMsg:
		return m, loadLines(m.logPath, m.iterPath, m.offset)
	}
	return m, nil
}

var (
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
	lineStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

func (m model) View() string {
	var sb strings.Builder
	sb.WriteString(headerStyle.Render("============================================================") + "\n")
	sb.WriteString(headerStyle.Render("📡 Ralph Monitor – Live Log") + "\n")
	sb.WriteString(dimStyle.Render(fmt.Sprintf("  %-18s %s", "Log file:", m.logPath)) + "\n")
	sb.WriteString(dimStyle.Render(fmt.Sprintf("  %-18s %s", "Current iteration:", m.iter)) + "\n")
	sb.WriteString(headerStyle.Render("============================================================") + "\n")
	sb.WriteString(dimStyle.Render("(Press q or Ctrl+C to stop)") + "\n\n")
	for _, l := range m.lines {
		sb.WriteString(lineStyle.Render(l) + "\n")
	}
	return sb.String()
}

func tick() tea.Cmd {
	return tea.Tick(pollInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func loadLines(logPath, iterPath string, _ int64) tea.Cmd {
	return func() tea.Msg {
		data, _ := os.ReadFile(logPath)
		all := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
		if len(all) > maxLines {
			all = all[len(all)-maxLines:]
		}

		iter := "?"
		if b, err := os.ReadFile(iterPath); err == nil {
			iter = strings.TrimSpace(string(b))
		}

		return linesMsg{lines: all, iter: iter, pos: int64(len(data))}
	}
}

// Run starts the monitor TUI. It reads from ralphDir/ralph.log.
func Run(ralphDir string) error {
	lp := filepath.Join(ralphDir, "ralph.log")
	ip := filepath.Join(ralphDir, "iteration.txt")

	if _, err := os.Stat(lp); err != nil {
		fmt.Fprintf(os.Stderr, "Error: No log file found: %s\nStart a ralph run first before using --monitor.\n", lp)
		return nil
	}

	m := model{logPath: lp, iterPath: ip}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
