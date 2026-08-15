// Package dashboard is solitary's terminal interface: one live view of every
// cell, its state and its secrets, with the actions that apply to it.
//
// Every action here is one the commands already do, and the slow ones are run
// as those commands. A cell being built prints what a build prints, and one
// asking for a secret it is missing asks the way it always does — the dashboard
// steps out of the way and comes back rather than reimplementing either.
package dashboard

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/dm-balakin/solitary/internal/cell"
	"github.com/dm-balakin/solitary/internal/config"
	"github.com/dm-balakin/solitary/internal/secrets"
)

// refreshEvery is how often the list of cells is re-read. A cell can stop, wedge
// or be started from another terminal, and none of that announces itself.
const refreshEvery = 5 * time.Second

// mode is what the dashboard is currently asking of the person using it.
type mode int

const (
	// browsing is the resting state: a list, a selection, and keys that act
	// on it.
	browsing mode = iota
	// confirming is waiting for an answer before destroying a machine.
	confirming
	// viewingNetwork is the full list of what the selected cell may reach.
	viewingNetwork
	// watchingTraffic is a live view of what it is actually reaching.
	watchingTraffic
	// filteringTraffic is typing what to narrow that view to.
	filteringTraffic
	// managingSecrets is the secrets view for the selected cell.
	managingSecrets
	// typing is entering a value for one secret, hidden as it is typed.
	typing
)

// runner starts one of solitary's own commands with the terminal to itself.
// The model holds it as a field so that a test can see what would be run
// without running anything.
type runner func(name, command, notice string, args ...string) tea.Cmd

type model struct {
	run    runner
	cells  []cell.Info
	cursor int

	detail    cell.Detail
	detailErr error
	// tunnel is what the selected cell's tunnel is doing, when it has one
	// and its machine could be asked. Nil means unknown, which is not the
	// same as down.
	tunnel *cell.TunnelState
	// handoff is the live inbox/outbox count for the selected running cell.
	handoff *cell.Handoff

	traffic []trafficRow
	stream  *stream
	// offset is how far back through the traffic the view is scrolled, in
	// rows from the newest. Zero means the newest, which is where it stays
	// unless someone scrolls: a feed that jumps while being read is worse
	// than one that has to be sent back to the end.
	offset int
	// onlyBlocked narrows the view to what was refused, which is what
	// someone opening it after something failed is looking for.
	onlyBlocked bool
	filter      textinput.Model

	mode    mode
	secret  int // index into detail.Secrets, in the secrets views
	input   textinput.Model
	notice  string
	failure error

	width, height int
	// loaded is false until the first listing arrives, so that an empty
	// screen is never mistaken for "no cells".
	loaded   bool
	quitting bool
}

// Messages. Everything slow happens in a command and arrives as one of these.
type (
	cellsMsg  struct{ cells []cell.Info }
	detailMsg struct{ detail cell.Detail }
	// doneMsg reports a command that ran with the terminal to itself.
	doneMsg struct {
		notice string
		err    error
	}
	savedMsg struct{ name string }
	failMsg  struct{ err error }
	tickMsg  struct{}
	// tunnelMsg is the state of one cell's tunnel. It carries the name it
	// was asked about, since the selection can move while it is in flight.
	tunnelMsg struct {
		name  string
		state *cell.TunnelState
	}
	handoffMsg struct {
		name  string
		state *cell.Handoff
	}
)

func newModel() model {
	input := textinput.New()
	// A secret is typed in front of whoever is looking at the screen.
	input.EchoMode = textinput.EchoPassword
	input.Prompt = "value "
	input.CharLimit = 4096

	filter := textinput.New()
	filter.Prompt = "/"
	filter.Placeholder = "name or address"
	filter.CharLimit = 64

	return model{run: run, input: input, filter: filter}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(refresh(), tick())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tickMsg:
		// The secrets views are a conversation; re-reading underneath them
		// would move what is being talked about.
		switch m.mode {
		case browsing:
			return m, tea.Batch(refresh(), m.watchTunnel(), m.watchHandoff(), tick())
		case viewingNetwork:
			// The list itself cannot change while it is being read, but
			// the tunnel carrying it can.
			return m, tea.Batch(m.watchTunnel(), tick())
		default:
			return m, tick()
		}

	case cellsMsg:
		return m.withCells(msg.cells)

	case detailMsg:
		if msg.detail.Name != m.detail.Name {
			// A different cell is being looked at, and the tunnel on
			// screen belonged to the last one.
			m.tunnel = nil
		}
		m.detail, m.detailErr = msg.detail, nil
		m.handoff = nil
		tunnelCmd, handoffCmd := m.watchTunnel(), m.watchHandoff()
		if tunnelCmd != nil {
			return m, tea.Batch(tunnelCmd, handoffCmd)
		}
		m.tunnel = nil
		return m, handoffCmd

	case tunnelMsg:
		// The selection may have moved while the machine was answering.
		if msg.name == m.detail.Name {
			m.tunnel = msg.state
		}
		return m, nil

	case handoffMsg:
		if msg.name == m.detail.Name {
			m.handoff = msg.state
		}
		return m, nil

	case savedMsg:
		m.notice = fmt.Sprintf("%s saved.", msg.name)
		if m.selected().Status == cell.StatusRunning {
			m.notice += " The cell is running with the old value; press u to restart it."
		}
		return m, describe(m.selected().Name)

	case doneMsg:
		m.failure, m.notice = msg.err, msg.notice
		return m, refresh()

	case streamMsg:
		// The view may already have been left by the time the follow
		// started, in which case nothing should be reading it.
		if m.mode != watchingTraffic {
			msg.stream.stop()
			return m, nil
		}
		m.stream = msg.stream
		return m, next(msg.stream.lines)

	case trafficMsg:
		if m.mode != watchingTraffic {
			return m, nil
		}
		m.traffic = add(m.traffic, msg.entry)
		if len(m.traffic) > trafficKept {
			m.traffic = m.traffic[len(m.traffic)-trafficKept:]
		}
		return m, next(msg.lines)

	case trafficStoppedMsg:
		m.stream = nil
		return m, nil

	case failMsg:
		m.failure = msg.err
		return m, nil

	case tea.KeyMsg:
		return m.key(msg)
	}

	// The text inputs own everything else while focused: cursor blink,
	// paste, and the rest.
	switch m.mode {
	case typing:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	case filteringTraffic:
		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		return m, cmd
	}

	return m, nil
}

// withCells takes a fresh listing, keeping the selection on the same cell where
// it can rather than on the same row.
func (m model) withCells(cells []cell.Info) (tea.Model, tea.Cmd) {
	previous := m.selected().Name
	m.cells, m.loaded = cells, true

	m.cursor = 0
	for i, c := range cells {
		if c.Name == previous {
			m.cursor = i
			break
		}
	}

	if len(cells) == 0 {
		m.detail = cell.Detail{}
		return m, nil
	}
	// Only re-read the definition when the selection actually moved: it is
	// a file read, but this runs every few seconds.
	if previous != m.selected().Name {
		return m, describe(m.selected().Name)
	}

	return m, nil
}

// watchTunnel re-reads what the selected cell's tunnel is doing, or nothing
// when there is no tunnel to read.
//
// The tunnel is the one part of this pane that moves on its own: the definition
// beside it only changes when someone edits a file, but a handshake ages and a
// counter climbs by the second. A figure that has quietly stopped being re-read
// looks exactly like a tunnel that has quietly stopped carrying anything.
func (m model) watchTunnel() tea.Cmd {
	if m.detail.Network.Tunnel == nil || m.selected().Status != cell.StatusRunning {
		return nil
	}

	return tunnelStatus(m.detail.Name)
}

func (m model) watchHandoff() tea.Cmd {
	if m.detail.Name == "" || m.selected().Status != cell.StatusRunning {
		return nil
	}
	return handoffStatus(m.detail.Name)
}

func (m model) selected() cell.Info {
	if m.cursor < 0 || m.cursor >= len(m.cells) {
		return cell.Info{}
	}

	return m.cells[m.cursor]
}

func (m model) key(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case confirming:
		return m.confirmKey(msg)
	case viewingNetwork:
		return m.networkKey(msg)
	case watchingTraffic:
		return m.trafficKey(msg)
	case filteringTraffic:
		return m.filterKey(msg)
	case managingSecrets:
		return m.secretsKey(msg)
	case typing:
		return m.typingKey(msg)
	default:
		return m.browseKey(msg)
	}
}

func (m model) browseKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	name := m.selected().Name

	switch msg.String() {
	case "q", "ctrl+c", "esc":
		m.stream.stop()
		m.quitting = true
		return m, tea.Quit

	case "up", "k":
		return m.move(-1)
	case "down", "j":
		return m.move(1)

	case "r":
		m.notice, m.failure = "", nil
		return m, refresh()

	case "enter":
		if name == "" {
			return m, nil
		}
		if m.selected().Status != cell.StatusRunning {
			m.failure = fmt.Errorf("cell %q is not running: press u to start it", name)
			return m, nil
		}
		return m.clear(), attach(name)

	case "u":
		if name == "" {
			return m, nil
		}
		// --detach because up otherwise opens a shell when it finishes,
		// which would leave the dashboard behind a session nobody asked
		// for. Attaching is what enter is for.
		return m.clear(), m.run(name, "up", fmt.Sprintf("Cell %q is up.", name), "--detach")

	case "s":
		if name == "" || m.selected().Status == cell.StatusUninitialized {
			return m, nil
		}
		return m.clear(), m.run(name, "down", fmt.Sprintf("Cell %q is stopped.", name))

	case "d":
		if name == "" || m.selected().Status == cell.StatusUninitialized {
			m.failure = fmt.Errorf("cell %q has no machine to remove", name)
			return m, nil
		}
		m.mode = confirming
		return m.clear(), nil

	case "e":
		if name == "" {
			return m, nil
		}
		m.mode, m.secret = managingSecrets, 0
		return m.clear(), describe(name)

	case "n":
		if name == "" {
			return m, nil
		}
		m.mode = viewingNetwork
		return m.clear(), describe(name)

	case "t":
		if name == "" {
			return m, nil
		}
		if m.selected().Status != cell.StatusRunning {
			m.failure = fmt.Errorf("cell %q is not running, so it has no traffic to watch", name)
			return m, nil
		}
		m.mode, m.traffic = watchingTraffic, nil
		m.offset, m.onlyBlocked = 0, false
		m.filter.SetValue("")
		return m.clear(), tea.Batch(describe(name), follow(name))
	}

	return m, nil
}

func (m model) confirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	name := m.selected().Name

	switch msg.String() {
	case "y", "Y":
		m.mode = browsing
		// The dashboard has already asked, so the command must not ask
		// again from behind a suspended screen.
		return m, m.run(name, "rm", fmt.Sprintf("The machine behind %q is gone.", name), "--force")
	default:
		m.mode = browsing
		m.notice = "Cancelled."
		return m, nil
	}
}

// networkKey handles the full allow list, which is read rather than edited:
// what a cell may reach is decided in its definition, not in a dashboard.
func (m model) networkKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	default:
		m.mode = browsing
		return m.clear(), nil
	}
}

// trafficKey handles the live view. Leaving it ends the follow: a cell's log
// should not be read over a connection nobody is watching.
func (m model) trafficKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rows := len(m.visibleTraffic())

	switch msg.String() {
	case "ctrl+c":
		m.stream.stop()
		m.quitting = true
		return m, tea.Quit

	case "q", "esc":
		m.stream.stop()
		m.stream, m.mode = nil, browsing
		return m.clear(), nil

	case "up", "k":
		return m.scroll(1, rows), nil
	case "down", "j":
		return m.scroll(-1, rows), nil
	case "pgup", "ctrl+b":
		return m.scroll(trafficShown, rows), nil
	case "pgdown", "ctrl+f", " ":
		return m.scroll(-trafficShown, rows), nil
	case "g", "home":
		return m.scroll(rows, rows), nil
	case "G", "end":
		m.offset = 0
		return m, nil

	case "b":
		m.onlyBlocked = !m.onlyBlocked
		m.offset = 0
		return m, nil

	case "/":
		m.mode = filteringTraffic
		m.filter.Focus()
		return m, textinput.Blink

	case "c":
		m.traffic, m.offset = nil, 0
		return m, nil
	}

	return m, nil
}

// filterKey narrows the view as it is typed, so what is being looked for shows
// up before it has been fully spelled.
func (m model) filterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.stream.stop()
		m.quitting = true
		return m, tea.Quit

	case "enter":
		m.mode = watchingTraffic
		m.filter.Blur()
		return m, nil

	case "esc":
		m.mode = watchingTraffic
		m.filter.Blur()
		m.filter.SetValue("")
		return m, nil
	}

	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	m.offset = 0

	return m, cmd
}

// scroll moves the view back through what has been kept, stopping at either
// end rather than wrapping.
func (m model) scroll(by, rows int) model {
	limit := rows - trafficShown
	if limit < 0 {
		limit = 0
	}

	m.offset += by
	if m.offset > limit {
		m.offset = limit
	}
	if m.offset < 0 {
		m.offset = 0
	}

	return m
}

// visibleTraffic is what the current filters leave.
func (m model) visibleTraffic() []trafficRow {
	needle := strings.ToLower(m.filter.Value())
	if !m.onlyBlocked && needle == "" {
		return m.traffic
	}

	rows := make([]trafficRow, 0, len(m.traffic))
	for _, row := range m.traffic {
		if m.onlyBlocked && !blocked(row.entry.Kind) {
			continue
		}
		if needle != "" && !strings.Contains(strings.ToLower(row.entry.Detail), needle) {
			continue
		}
		rows = append(rows, row)
	}

	return rows
}

// blocked is what someone opening this view after a failure is looking for.
func blocked(kind cell.TrafficKind) bool {
	return kind == cell.TrafficDenied || kind == cell.TrafficRefused
}

func (m model) secretsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.mode = browsing
		return m.clear(), nil

	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "up", "k":
		if m.secret > 0 {
			m.secret--
		}
		return m, nil

	case "down", "j":
		if m.secret < len(m.detail.Secrets)-1 {
			m.secret++
		}
		return m, nil

	case "enter":
		if len(m.detail.Secrets) == 0 {
			return m, nil
		}
		m.mode = typing
		m.input.SetValue("")
		m.input.Focus()
		return m.clear(), textinput.Blink
	}

	return m, nil
}

func (m model) typingKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = managingSecrets
		m.input.Blur()
		m.input.SetValue("")
		return m, nil

	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "enter":
		value := m.input.Value()
		name := m.detail.Secrets[m.secret].Name
		m.mode = managingSecrets
		m.input.Blur()
		m.input.SetValue("")
		if value == "" {
			m.notice = "Nothing typed; the value is unchanged."
			return m, nil
		}
		return m, save(m.detail.Name, name, value)
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	return m, cmd
}

func (m model) move(by int) (tea.Model, tea.Cmd) {
	if len(m.cells) == 0 {
		return m, nil
	}

	next := m.cursor + by
	if next < 0 || next >= len(m.cells) {
		return m, nil
	}
	m.cursor = next

	return m.clear(), describe(m.selected().Name)
}

// clear drops whatever the last action had to say, so that a notice never
// outlives the thing it was about.
func (m model) clear() model {
	m.notice, m.failure = "", nil

	return m
}

// Commands.

func refresh() tea.Cmd {
	return func() tea.Msg {
		cells, err := cell.List()
		if err != nil {
			return failMsg{err}
		}

		return cellsMsg{cells}
	}
}

func describe(name string) tea.Cmd {
	return func() tea.Msg {
		detail, err := cell.Describe(name)
		if err != nil {
			return failMsg{err}
		}

		return detailMsg{detail}
	}
}

// handoffStatus asks the machine for the two hand-off queues. Errors are
// intentionally quiet because the cell may stop between the row refresh and
// this request.
func handoffStatus(name string) tea.Cmd {
	return func() tea.Msg {
		state, err := cell.HandoffStatus(name)
		if err != nil {
			return handoffMsg{name: name}
		}
		return handoffMsg{name: name, state: &state}
	}
}

// tunnelStatus asks the machine what its tunnel is doing. Unlike describe this
// leaves the host, so it is only ever run for the cell being looked at, and
// only when that cell has a tunnel and is running.
func tunnelStatus(name string) tea.Cmd {
	return func() tea.Msg {
		state, err := cell.TunnelStatus(name)
		if err != nil {
			// Not worth a failure on screen: the row says what is known,
			// and the next tick asks again.
			return tunnelMsg{name: name}
		}

		return tunnelMsg{name: name, state: &state}
	}
}

func tick() tea.Cmd {
	return tea.Tick(refreshEvery, func(time.Time) tea.Msg { return tickMsg{} })
}

// run hands the terminal to solitary's own command for as long as it takes.
//
// These are the slow actions, and they have things to show and sometimes to
// ask: a build prints what a build prints, and a cell missing a secret asks for
// it. Reimplementing that inside a pane would mean a second, worse version of
// each. Running the command instead means there is only ever one.
func run(name, command, notice string, args ...string) tea.Cmd {
	self, err := os.Executable()
	if err != nil {
		return func() tea.Msg { return failMsg{fmt.Errorf("finding solitary: %w", err)} }
	}

	full := append([]string{command}, args...)
	cmd := exec.Command(self, append(full, name)...)

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return doneMsg{err: fmt.Errorf("%s %s: %w", command, name, err)}
		}

		return doneMsg{notice: notice}
	})
}

// attach hands the terminal to the cell's shell, and takes it back when it
// exits.
func attach(name string) tea.Cmd {
	cmd, err := cell.ShellCommand(name)
	if err != nil {
		return func() tea.Msg { return failMsg{err} }
	}

	return tea.ExecProcess(cmd, func(error) tea.Msg {
		// A shell exiting non-zero is how shells exit; it says nothing
		// about whether attaching worked.
		return doneMsg{}
	})
}

func save(cellName, secret, value string) tea.Cmd {
	return func() tea.Msg {
		path, err := config.EnvFile(cellName)
		if err != nil {
			return failMsg{err}
		}
		if err := secrets.Set(path, secret, value); err != nil {
			return failMsg{err}
		}

		return savedMsg{name: secret}
	}
}
