package dashboard

import (
	"bufio"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/balakin/solitary/internal/cell"
)

// trafficKept is how much of a cell's traffic is held. A busy cell resolves
// hundreds of names a minute, and only what fits on the screen is ever read;
// the rest is kept so that scrolling back a little is possible.
const trafficKept = 500

// trafficRow is one line of the view. Repeats are counted rather than
// repeated: a single lookup answers with every address a name has, and a
// program refused once retries, so without this one event fills the pane.
type trafficRow struct {
	entry cell.Traffic
	count int
}

// subject is what a row is about, ignoring which of a name's addresses came
// back, so that answers for one name collapse into one line.
func subject(entry cell.Traffic) string {
	name, _, _ := strings.Cut(entry.Detail, " → ")

	return string(entry.Kind) + " " + name
}

// add appends an entry, folding it into the last row when it says the same
// thing again.
func add(rows []trafficRow, entry cell.Traffic) []trafficRow {
	if n := len(rows); n > 0 && subject(rows[n-1].entry) == subject(entry) {
		rows[n-1].count++
		rows[n-1].entry.At = entry.At

		return rows
	}

	return append(rows, trafficRow{entry: entry, count: 1})
}

// stream is a running follow of one cell's traffic. The command outlives a
// single Update, so the model holds it and kills it when the view is left.
type stream struct {
	cmd   *exec.Cmd
	lines <-chan cell.Traffic
}

type (
	// trafficMsg is one thing that happened, with the channel the next one
	// arrives on.
	trafficMsg struct {
		entry cell.Traffic
		lines <-chan cell.Traffic
	}
	// trafficStoppedMsg means the follow ended: the view was left, the cell
	// stopped, or the connection to it broke.
	trafficStoppedMsg struct{}
	// streamMsg carries a started follow back to the model.
	streamMsg struct{ stream *stream }
)

// follow starts reading a cell's traffic. Parsing happens here rather than in
// the model so that only what is displayed crosses into it.
func follow(name string) tea.Cmd {
	return func() tea.Msg {
		cmd, err := cell.TrafficCommand(name)
		if err != nil {
			return failMsg{err}
		}

		out, err := cmd.StdoutPipe()
		if err != nil {
			return failMsg{err}
		}
		if err := cmd.Start(); err != nil {
			return failMsg{err}
		}

		lines := make(chan cell.Traffic, 64)
		go func() {
			defer close(lines)
			scanner := bufio.NewScanner(out)
			scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
			for scanner.Scan() {
				entry, ok := cell.ParseTraffic(scanner.Text())
				if !ok {
					continue
				}
				// Dropped rather than blocked on: a cell that
				// suddenly talks a lot must not stall the screen.
				select {
				case lines <- entry:
				default:
				}
			}
			_ = cmd.Wait()
		}()

		return streamMsg{&stream{cmd: cmd, lines: lines}}
	}
}

// next waits for the following entry.
func next(lines <-chan cell.Traffic) tea.Cmd {
	return func() tea.Msg {
		entry, ok := <-lines
		if !ok {
			return trafficStoppedMsg{}
		}

		return trafficMsg{entry: entry, lines: lines}
	}
}

// stop ends a follow. Killing the command closes the pipe, which ends the
// goroutine reading it.
func (s *stream) stop() {
	if s == nil || s.cmd == nil || s.cmd.Process == nil {
		return
	}
	_ = s.cmd.Process.Kill()
}
