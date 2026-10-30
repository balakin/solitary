package dashboard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const actionHelp = `↑/↓ or j/k  select cell or scroll details
Tab  switch between cells and details
PgUp/PgDown  page through the focused pane
Home/End  first or last item
Enter  open shell
u  start or apply changes
b  rebuild container
m  replace machine, keeping its home
p  patch machine
s  stop cell
e  manage secrets
n  read network policy
i  inspect cell
t  watch traffic
d  remove machine
r  refresh
q / Esc  quit

Network, inspect and help: ↑/↓ scroll, Esc back.
Traffic: ↑/↓ scroll, G live, / filter, b refused only, c clear.
Secrets: ↑/↓ select, Enter set value, Esc back.`

func (m model) narrow() bool { return m.width > 0 && m.width < 100 }

func (m model) footer() string {
	if m.mode != browsing {
		return m.help()
	}
	focus := "cells"
	if m.detailsFocused {
		focus = "details"
	}
	navigation := "↑↓ " + focus + " · tab switch · ? keys · q quit"
	if m.width >= 100 {
		return m.help() + "\n" + navigation
	}
	return navigation
}

func (m model) wrapped(s string, width int) string {
	return lipgloss.NewStyle().Width(max(1, width)).MaxWidth(max(1, width)).Render(s)
}

func (m model) footerLines() []string {
	width := m.width
	if width <= 0 {
		width = 100
	}
	footer := m.wrapped(helpStyle.Render(m.footer()), width)
	lines := strings.Split(footer, "\n")
	if m.height > 0 && len(lines) > max(1, m.height/3) {
		short := "esc back"
		if m.mode == browsing {
			short = "? keys · q quit"
		}
		lines = strings.Split(m.wrapped(helpStyle.Render(short), width), "\n")
	}
	if message := m.message(); message != "" {
		msg := strings.Split(m.wrapped(message, width), "\n")
		// Confirmation text must remain visible before the answer is accepted.
		limit := len(msg)
		if m.height > 0 && m.mode != confirming {
			limit = max(1, m.height-len(lines)-5)
		}
		lines = append(msg[:min(len(msg), limit)], lines...)
	}
	return lines
}

func (m model) bodyHeight() int {
	if m.height <= 0 {
		return 0
	}
	return max(0, m.height-1-len(m.footerLines()))
}

func (m model) listCapacity() int {
	if m.height <= 0 {
		return max(1, len(m.cells))
	}
	return max(1, m.bodyHeight()-3)
}

func (m model) cellWindow() (int, int) {
	count := m.listCapacity()
	start := max(0, m.cursor-count+1)
	return start, min(len(m.cells), start+count)
}

func (m model) paneWidth(list bool) int {
	if m.width <= 0 {
		return 0
	}
	if m.narrow() {
		return m.width
	}
	left := min(36, m.width/3)
	if list {
		return left
	}
	return m.width - left
}

// Subviews own the keys even when the main view last focused the cell list.
func (m model) cellsFocused() bool {
	return (m.mode == browsing || m.mode == confirming) && !m.detailsFocused
}

func (m model) tabs() string {
	cells, details := "cells", "details"
	if m.cellsFocused() {
		cells = selectedStyle.Foreground(mauve).Render("[cells]")
		details = labelStyle.Render(details)
	} else {
		cells = labelStyle.Render(cells)
		details = selectedStyle.Foreground(mauve).Render("[details]")
	}
	return cells + "  " + details
}

func (m model) screen() string {
	body := ""
	if m.narrow() {
		if m.detailsFocused || (m.mode != browsing && m.mode != confirming) {
			body = m.detailPane()
		} else {
			body = m.list()
		}
	} else {
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.list(), m.detailPane())
	}
	header := titleStyle.Render("solitary") + " · " + m.tabs()
	if m.width > 0 {
		header = lipgloss.NewStyle().MaxWidth(m.width).Render(header)
	}
	parts := []string{header}
	if m.height <= 0 || m.bodyHeight() > 0 {
		parts = append(parts, body)
	}
	parts = append(parts, m.footerLines()...)
	result := strings.Join(parts, "\n")
	if m.height > 0 && lipgloss.Height(result) > m.height {
		// Even a terminal too short for borders keeps the selection and controls.
		result = m.selected().Name + "\n" + m.footer()
		if m.mode == confirming {
			result = "Enlarge terminal to read confirmation · esc cancel"
		}
		lines := strings.Split(m.wrapped(result, m.width), "\n")
		result = strings.Join(lines[max(0, len(lines)-m.height):], "\n")
	}
	return result
}

func (m model) detailFrame(title, body string) string {
	return m.frame(title, body, false, m.detailOffset)
}

func (m model) frame(title, body string, list bool, offset int) string {
	width := m.paneWidth(list)
	if width <= 0 || m.height <= 0 {
		return pane(title, body)
	}
	inner := max(1, width-4)
	// List and traffic rows stay one line each so paging counts visible entries.
	if list || m.mode == watchingTraffic || m.mode == filteringTraffic {
		body = lipgloss.NewStyle().MaxWidth(inner).Render(body)
	} else {
		body = m.wrapped(body, inner)
	}
	if m.measuring {
		return body
	}
	lines := strings.Split(body, "\n")
	capacity := max(1, m.bodyHeight()-3)
	if !list && (m.mode == managingSecrets || m.mode == typing) {
		selected := 0
		for i, line := range lines {
			if strings.Contains(line, "› ") {
				selected = i
				break
			}
		}
		if m.mode == typing {
			selected = len(lines) - 1
		}
		offset = max(0, selected-capacity+1)
	}
	offset = max(0, min(offset, len(lines)-min(capacity, len(lines))))
	end := min(len(lines), offset+capacity)
	if len(lines) > capacity {
		title += fmt.Sprintf(" · %d–%d/%d", offset+1, end, len(lines))
	}
	title = lipgloss.NewStyle().MaxWidth(inner).Render(title)
	if m.bodyHeight() < 4 {
		return strings.Join(lines[offset:end], "\n")
	}
	style, heading := paneStyle, labelStyle
	if list == m.cellsFocused() {
		style = style.BorderForeground(mauve)
		heading = paneTitle.Bold(true)
	}
	content := heading.Render(title) + "\n" + strings.Join(lines[offset:end], "\n")
	return style.Width(width - 2).Height(m.bodyHeight() - 2).Render(content)
}

func (m model) scrollDetails(key string) model {
	by := 1
	switch key {
	case "up", "k":
		by = -1
	case "pgup":
		by = -m.listCapacity()
	case "pgdown":
		by = m.listCapacity()
	case "resize":
		by = 0
	case "home":
		m.detailOffset = 0
		return m
	case "end":
		by = 1 << 20
	}
	// Clamp against the wrapped content so repeated keys never accumulate an
	// invisible overscroll that would make scrolling back appear unresponsive.
	probe := m
	probe.measuring = true
	lines := strings.Split(probe.detailPane(), "\n")
	limit := max(0, len(lines)-m.listCapacity())
	m.detailOffset = max(0, min(limit, m.detailOffset+by))
	return m
}

func (m model) trafficCapacity() int {
	if m.height <= 0 {
		return trafficShown
	}
	return max(1, m.listCapacity()-1)
}

func (m model) simpleView(body, footer string) string {
	if m.width <= 0 || m.height <= 0 {
		return "solitary\n\n" + body + "\n" + footer
	}
	controls := strings.Split(m.wrapped(helpStyle.Render(footer), m.width), "\n")
	if len(controls) >= m.height {
		return strings.Join(controls[:m.height], "\n")
	}
	content := strings.Split(m.wrapped(titleStyle.Render("solitary")+"\n"+body, m.width), "\n")
	content = content[:min(len(content), m.height-len(controls))]
	return strings.Join(append(content, controls...), "\n")
}
