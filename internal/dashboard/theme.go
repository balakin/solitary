package dashboard

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/dm-balakin/solitary/internal/cell"
)

// The palette is Catppuccin Mocha, the same one a cell's own tools are themed
// with. lipgloss degrades these to whatever the terminal actually has.
var (
	mauve  = lipgloss.Color("#cba6f7")
	green  = lipgloss.Color("#a6e3a1")
	yellow = lipgloss.Color("#f9e2af")
	red    = lipgloss.Color("#f38ba8")
	dim    = lipgloss.Color("#7f849c")
	text   = lipgloss.Color("#cdd6f4")
)

var (
	titleStyle    = lipgloss.NewStyle().Foreground(mauve).Bold(true)
	paneStyle     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(dim).Padding(0, 1)
	paneTitle     = lipgloss.NewStyle().Foreground(mauve)
	selectedStyle = lipgloss.NewStyle().Foreground(text).Bold(true)
	labelStyle    = lipgloss.NewStyle().Foreground(dim)
	valueStyle    = lipgloss.NewStyle().Foreground(text)
	helpStyle     = lipgloss.NewStyle().Foreground(dim)
	errorStyle    = lipgloss.NewStyle().Foreground(red)
	noticeStyle   = lipgloss.NewStyle().Foreground(green)
)

// statusStyle colours a cell's state: green for one that is ready, yellow for
// one that needs attention, red for one that is wrong, and grey for one that is
// simply not running.
func statusStyle(status cell.Status) lipgloss.Style {
	style := lipgloss.NewStyle()

	switch status {
	case cell.StatusRunning:
		return style.Foreground(green)
	case cell.StatusDegraded:
		return style.Foreground(yellow)
	case cell.StatusUnreachable, cell.StatusBroken:
		return style.Foreground(red)
	default:
		return style.Foreground(dim)
	}
}

// statusMark is a shape for the same information, for terminals and eyes that
// do not separate colours well.
func statusMark(status cell.Status) string {
	switch status {
	case cell.StatusRunning:
		return "●"
	case cell.StatusStopped:
		return "○"
	case cell.StatusDegraded:
		return "◐"
	case cell.StatusUnreachable, cell.StatusBroken:
		return "!"
	default:
		return "◌"
	}
}
