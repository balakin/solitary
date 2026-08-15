package dashboard

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/dm-balakin/solitary/internal/cell"
)

func (m model) View() string {
	if m.quitting {
		return ""
	}
	if !m.loaded {
		return titleStyle.Render("solitary") + "\n\n" + labelStyle.Render("Reading cells…") + "\n"
	}
	if len(m.cells) == 0 {
		return m.empty()
	}

	body := lipgloss.JoinHorizontal(lipgloss.Top, m.list(), m.detailPane())

	parts := []string{titleStyle.Render("solitary"), body}
	if line := m.message(); line != "" {
		parts = append(parts, line)
	}
	parts = append(parts, helpStyle.Render(m.help()))

	return lipgloss.JoinVertical(lipgloss.Left, parts...) + "\n"
}

func (m model) empty() string {
	return lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("solitary"),
		"",
		valueStyle.Render("No cells yet."),
		labelStyle.Render("Create one with: solitary init <name>"),
		"",
		helpStyle.Render("r refresh · q quit"),
	) + "\n"
}

// list is every cell and its state, which is the whole point of the screen.
func (m model) list() string {
	width := 0
	for _, c := range m.cells {
		if len(c.Name) > width {
			width = len(c.Name)
		}
	}

	rows := make([]string, 0, len(m.cells))
	for i, c := range m.cells {
		style, cursor := valueStyle, "  "
		if i == m.cursor {
			style, cursor = selectedStyle, "› "
		}
		mark := statusStyle(c.Status).Render(statusMark(c.Status))
		name := style.Render(fmt.Sprintf("%-*s", width, c.Name))
		status := statusStyle(c.Status).Render(string(c.Status))
		rows = append(rows, fmt.Sprintf("%s%s %s  %s", cursor, mark, name, status))
	}

	return pane("cells", strings.Join(rows, "\n"))
}

// detailPane is what the selected cell is made of: where its tools come from,
// the machine it runs in, and which of its secrets are set.
func (m model) detailPane() string {
	if m.mode == managingSecrets || m.mode == typing {
		return pane("secrets · "+m.detail.Name, m.secretsBody())
	}

	if m.detailErr != nil {
		return pane(m.selected().Name, errorStyle.Render(m.detailErr.Error()))
	}

	machine := fmt.Sprintf("%d cpus · %s · %s", m.detail.VM.CPUs, m.detail.VM.Memory, m.detail.VM.Disk)

	ports := "all reach host localhost"
	if len(m.detail.Ports) > 0 {
		list := make([]string, 0, len(m.detail.Ports))
		for _, p := range m.detail.Ports {
			list = append(list, strconv.Itoa(p))
		}
		ports = strings.Join(list, ", ")
	}

	lines := []string{
		field("image", m.detail.Image),
		field("machine", machine),
		field("ports", ports),
		field("secrets", m.secretsSummary()),
	}

	return pane(m.detail.Name, strings.Join(lines, "\n"))
}

// secretsSummary says how many of a cell's secrets are ready without listing
// them, since the detail pane is about the cell as a whole.
func (m model) secretsSummary() string {
	if len(m.detail.Secrets) == 0 {
		return labelStyle.Render("none declared")
	}

	set := 0
	for _, s := range m.detail.Secrets {
		if s.Set {
			set++
		}
	}
	summary := fmt.Sprintf("%d of %d set", set, len(m.detail.Secrets))
	if set < len(m.detail.Secrets) {
		return statusStyle(cell.StatusDegraded).Render(summary)
	}

	return summary
}

// secretsBody lists the names a cell is allowed to see and whether each has a
// value. The values themselves are never here, and never anywhere on screen.
func (m model) secretsBody() string {
	if len(m.detail.Secrets) == 0 {
		return valueStyle.Render("This cell declares no secrets.") + "\n" +
			labelStyle.Render("Add them under secrets: in its cell.yaml.")
	}

	width := 0
	for _, s := range m.detail.Secrets {
		if len(s.Name) > width {
			width = len(s.Name)
		}
	}

	rows := make([]string, 0, len(m.detail.Secrets)+2)
	for i, s := range m.detail.Secrets {
		style, cursor := valueStyle, "  "
		if i == m.secret {
			style, cursor = selectedStyle, "› "
		}
		state := labelStyle.Render("not set")
		if s.Set {
			state = noticeStyle.Render("set")
		}
		rows = append(rows, fmt.Sprintf("%s%s  %s", cursor, style.Render(fmt.Sprintf("%-*s", width, s.Name)), state))
	}

	if m.mode == typing {
		rows = append(rows, "", m.input.View())
	}

	return strings.Join(rows, "\n")
}

// message is whatever the last thing to happen had to say.
func (m model) message() string {
	switch {
	case m.mode == confirming:
		return errorStyle.Render(fmt.Sprintf(
			"Destroy the machine behind %q? Everything inside it is lost. [y/N]", m.selected().Name))
	case m.failure != nil:
		return errorStyle.Render(m.failure.Error())
	case m.notice != "":
		return noticeStyle.Render(m.notice)
	default:
		return ""
	}
}

func (m model) help() string {
	switch m.mode {
	case confirming:
		return "y destroy · any other key cancel"
	case typing:
		return "enter save · esc cancel"
	case managingSecrets:
		return "↑↓ move · enter set value · esc back"
	default:
		return "↑↓ move · ⏎ shell · u up · s stop · e secrets · d rm · r refresh · q quit"
	}
}

func field(label, value string) string {
	return labelStyle.Render(fmt.Sprintf("%-8s", label)) + valueStyle.Render(value)
}

func pane(title, body string) string {
	return paneStyle.Render(paneTitle.Render(title) + "\n" + body)
}
