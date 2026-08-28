package dashboard

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/balakin/solitary/internal/cell"
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
	if m.mode == viewingNetwork {
		return pane("network · "+m.detail.Name, m.networkBody())
	}
	if m.mode == watchingTraffic || m.mode == filteringTraffic {
		return pane("traffic · "+m.detail.Name, m.trafficBody())
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
	}
	if m.detail.ProvisionChanged {
		// The one change stopping and starting the cell does not apply, so
		// it is said here rather than left to the next up.
		lines = append(lines,
			field("provision", warnStyle.Render("changed since this machine was provisioned")),
			field("", valueStyle.Render("d then u rebuilds it; the disk and home go with it")),
		)
	}
	lines = append(lines, m.network()...)
	lines = append(lines, m.vpn()...)
	lines = append(lines, m.handoffLines()...)
	lines = append(lines, field("secrets", m.secretsSummary()))

	return pane(m.detail.Name, strings.Join(lines, "\n"))
}

// networkPreview is how many allowed entries are shown before the rest are
// summarised. Enough to recognise a list; not enough to push everything else
// off the pane.
const networkPreview = 4

// network shows what the cell may reach. Unlike everything else in this pane it
// is a security control, so a cell that restricts nothing has to be as plain to
// read as one that does — the absence is the thing worth noticing.
func (m model) network() []string {
	if !m.detail.Network.Restricted() {
		return []string{field("network", statusStyle(cell.StatusDegraded).Render("unrestricted"))}
	}

	allow := m.detail.Network.Allow
	summary := fmt.Sprintf("%d allowed", len(allow))
	if m.detail.Network.Tunnel != nil {
		summary += " · tunnel"
	}
	lines := []string{field("network", noticeStyle.Render(summary))}

	shown := allow
	if len(shown) > networkPreview {
		shown = shown[:networkPreview]
	}
	for _, entry := range shown {
		lines = append(lines, field("", valueStyle.Render(entry)))
	}
	if rest := len(allow) - len(shown); rest > 0 {
		lines = append(lines, field("", labelStyle.Render(fmt.Sprintf("+%d more", rest))))
	}

	return lines
}

// vpn is where the cell's traffic leaves from, and whether it is leaving at
// all. Configured and up are separate facts and the gap between them is the
// one worth seeing: a cell whose tunnel is down reaches nothing.
func (m model) vpn() []string {
	tunnel := m.detail.Network.Tunnel
	if tunnel == nil {
		return []string{field("vpn", labelStyle.Render("none"))}
	}

	endpoint := tunnel.EndpointHost + ":" + tunnel.EndpointPort
	switch {
	case m.selected().Status != cell.StatusRunning:
		return []string{
			field("vpn", labelStyle.Render("configured")),
			field("", valueStyle.Render(endpoint)),
		}

	case m.tunnel == nil:
		return []string{
			field("vpn", labelStyle.Render("asking the machine…")),
			field("", valueStyle.Render(endpoint)),
		}

	case !m.tunnel.Up:
		return []string{
			field("vpn", errorStyle.Render("down — this cell reaches nothing")),
			field("", valueStyle.Render(endpoint)),
		}

	case !m.tunnel.Healthy():
		// Up but silent. From inside the cell this is indistinguishable
		// from the network being broken, which is why it is said here.
		return []string{
			field("vpn", statusStyle(cell.StatusDegraded).Render("up, no recent handshake")),
			field("", valueStyle.Render(m.peer(endpoint))),
			field("", labelStyle.Render(m.tunnel.Transferred())),
		}

	default:
		return []string{
			field("vpn", noticeStyle.Render("up · handshake "+ago(m.tunnel.Since))),
			field("", valueStyle.Render(m.peer(endpoint))),
			field("", labelStyle.Render(m.tunnel.Transferred())),
		}
	}
}

// peer names what the cell is actually talking to. A provider that balances one
// hostname across servers moves this between handshakes, so the address is
// worth showing next to the name it came from.
func (m model) peer(endpoint string) string {
	if m.tunnel == nil || m.tunnel.Peer == "" {
		return endpoint
	}

	return endpoint + " → " + m.tunnel.Peer
}

// ago renders how long since something happened, at the precision worth reading
// at a glance.
func ago(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
}

// networkBody is the whole allow list, for when the preview is not enough.
func (m model) networkBody() string {
	if !m.detail.Network.Restricted() {
		return valueStyle.Render("This cell reaches whatever the host reaches.") + "\n" +
			labelStyle.Render("Restrict it with network.allow in its cell.yaml.")
	}

	rows := make([]string, 0, len(m.detail.Network.Allow)+3)
	for _, entry := range m.detail.Network.Allow {
		rows = append(rows, "  "+valueStyle.Render(entry))
	}
	rows = append(rows, "", labelStyle.Render("Everything else is refused, including the host."))
	if tunnel := m.detail.Network.Tunnel; tunnel != nil {
		rows = append(rows,
			labelStyle.Render("And what is allowed is reached only through the tunnel to ")+
				valueStyle.Render(tunnel.EndpointHost)+labelStyle.Render(","),
			labelStyle.Render("so while that is down, nothing leaves at all."),
			"")
		rows = append(rows, m.vpn()...)
	}
	if m.detail.Network.HostResolverOutsideTunnel() {
		rows = append(rows, "", statusStyle(cell.StatusDegraded).Render(
			"Names are still resolved outside it: resolvers names host."))
	}

	return strings.Join(rows, "\n")
}

// trafficShown is how many entries fit in the pane. The newest are the ones
// worth seeing, so the list is cut from the top.
const trafficShown = 14

// trafficBody is what the cell's network is doing, as it happens.
func (m model) trafficBody() string {
	if len(m.traffic) > 0 {
		return strings.Join(append(m.trafficRows(), "", m.trafficStatus()), "\n")
	}

	{
		if m.stream == nil {
			return labelStyle.Render("Waiting for the machine's log…")
		}
		if !m.detail.Network.Restricted() {
			return valueStyle.Render("Nothing is logged for this cell.") + "\n" +
				labelStyle.Render("Traffic is recorded where it is filtered; set network.allow to see it.")
		}

		return labelStyle.Render("Nothing yet. This is live: it fills as the cell talks.")
	}
}

// trafficRows is the window of the feed currently being looked at.
func (m model) trafficRows() []string {
	visible := m.visibleTraffic()
	if len(visible) == 0 {
		return []string{labelStyle.Render("Nothing matches.")}
	}

	end := len(visible) - m.offset
	if end < 1 {
		end = 1
	}
	start := end - trafficShown
	if start < 0 {
		start = 0
	}
	entries := visible[start:end]

	rows := make([]string, 0, len(entries))
	for _, row := range entries {
		line := fmt.Sprintf("%s %s %s",
			labelStyle.Render(row.entry.At),
			trafficStyle(row.entry.Kind).Render(fmt.Sprintf("%-8s", string(row.entry.Kind))),
			valueStyle.Render(row.entry.Detail),
		)
		if row.count > 1 {
			line += labelStyle.Render(fmt.Sprintf(" ×%d", row.count))
		}
		rows = append(rows, line)
	}

	return rows
}

// trafficStatus says whether what is on screen is still moving, and what is
// being left out. A feed that has quietly stopped following looks identical to
// a cell that has gone quiet, and the difference matters.
func (m model) trafficStatus() string {
	parts := []string{}

	if m.offset > 0 {
		parts = append(parts, statusStyle(cell.StatusDegraded).Render(
			fmt.Sprintf("paused · %d newer", m.offset)))
	} else {
		parts = append(parts, noticeStyle.Render("live"))
	}

	if m.onlyBlocked {
		parts = append(parts, errorStyle.Render("refused only"))
	}
	if m.mode == filteringTraffic {
		parts = append(parts, m.filter.View())
	} else if value := m.filter.Value(); value != "" {
		parts = append(parts, valueStyle.Render("/"+value))
	}

	kept := len(m.visibleTraffic())
	parts = append(parts, labelStyle.Render(fmt.Sprintf("%d of %d", kept, len(m.traffic))))

	return strings.Join(parts, labelStyle.Render(" · "))
}

// trafficStyle colours what happened: what was refused is the point of the
// view, what was allowed is context.
func trafficStyle(kind cell.TrafficKind) lipgloss.Style {
	switch kind {
	case cell.TrafficDenied, cell.TrafficRefused:
		return lipgloss.NewStyle().Foreground(red)
	case cell.TrafficResolved:
		return lipgloss.NewStyle().Foreground(green)
	default:
		return lipgloss.NewStyle().Foreground(dim)
	}
}

func (m model) handoffLines() []string {
	if m.selected().Status != cell.StatusRunning {
		return []string{field("handoff", labelStyle.Render("machine not running"))}
	}
	if m.handoff == nil {
		return []string{field("handoff", labelStyle.Render("asking the machine…"))}
	}
	return []string{
		field("inbox", queueSummary(m.handoff.Inbox)),
		field("outbox", queueSummary(m.handoff.Outbox)),
	}
}

func queueSummary(q cell.Queue) string {
	return fmt.Sprintf("%d file%s · %s", q.Files, plural(q.Files), cell.Size(q.Bytes))
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// secretsSummary says how many of a cell's secrets are ready without listing
// them, since the detail pane is about the cell as a whole.
func (m model) secretsSummary() string {
	if len(m.detail.Secrets) == 0 {
		return labelStyle.Render("none declared")
	}

	set, missing := 0, 0
	for _, s := range m.detail.Secrets {
		switch {
		case s.Set:
			set++
		case s.Required:
			missing++
		}
	}
	// Counted against every declared name, but degraded only on a missing
	// required one: a cell running without an optional secret is working as
	// asked, not broken.
	summary := fmt.Sprintf("%d of %d set", set, len(m.detail.Secrets))
	if missing > 0 {
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
		switch {
		case s.Set:
			state = noticeStyle.Render("set")
		case !s.Required:
			state = labelStyle.Render("not set (optional)")
		}
		rows = append(rows, fmt.Sprintf("%s%s  %s", cursor, style.Render(fmt.Sprintf("%-*s", width, s.Name)), state))
		// What the secret is for, under the name it belongs to, so a
		// value can be found before it is asked for.
		if s.Description != "" {
			rows = append(rows, fmt.Sprintf("  %s  %s", strings.Repeat(" ", width), labelStyle.Render(s.Description)))
		}
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
	case viewingNetwork:
		return "any key back"
	case filteringTraffic:
		return "type to narrow · enter keep · esc clear"
	case watchingTraffic:
		return "↑↓ scroll · G live · / filter · b refused only · c clear · esc back"
	case managingSecrets:
		return "↑↓ move · enter set value · esc back"
	default:
		return "↑↓ move · ⏎ shell · u up · s stop · e secrets · n network · t traffic · d rm · q quit"
	}
}

func field(label, value string) string {
	return labelStyle.Render(fmt.Sprintf("%-8s", label)) + valueStyle.Render(value)
}

func pane(title, body string) string {
	return paneStyle.Render(paneTitle.Render(title) + "\n" + body)
}
