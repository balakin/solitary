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

	traffic []trafficRow
	stream  *stream

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
)

func newModel() model {
	input := textinput.New()
	// A secret is typed in front of whoever is looking at the screen.
	input.EchoMode = textinput.EchoPassword
	input.Prompt = "value "
	input.CharLimit = 4096

	return model{run: run, input: input}
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
		if m.mode == browsing {
			return m, tea.Batch(refresh(), tick())
		}
		return m, tick()

	case cellsMsg:
		return m.withCells(msg.cells)

	case detailMsg:
		m.detail, m.detailErr = msg.detail, nil
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

	// The text input owns everything else while it has focus: cursor blink,
	// paste, and the rest.
	if m.mode == typing {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
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
	switch msg.String() {
	case "ctrl+c":
		m.stream.stop()
		m.quitting = true
		return m, tea.Quit
	case "c":
		m.traffic = nil
		return m, nil
	default:
		m.stream.stop()
		m.stream, m.mode = nil, browsing
		return m.clear(), nil
	}
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
