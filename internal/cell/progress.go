package cell

import (
	"fmt"
	"io"
	"os"
	"time"

	"golang.org/x/term"

	"github.com/balakin/solitary/internal/host"
)

// meter shows how far a long copy has got. On a terminal it redraws one line
// in place; anywhere else — a log, a pipe — it writes a line at every tenth,
// so that a file someone reads afterwards is not a wall of carriage returns.
type meter struct {
	w     io.Writer
	label string
	total uint64
	done  uint64
	tty   bool
	start time.Time
	drawn time.Time
	tenth uint64
	now   func() time.Time
}

func newMeter(w io.Writer, label string, total uint64) *meter {
	f, ok := w.(*os.File)
	m := &meter{
		w:     w,
		label: label,
		total: total,
		tty:   ok && term.IsTerminal(int(f.Fd())),
		now:   time.Now,
	}
	m.start = m.now()

	return m
}

// add counts n more bytes copied.
func (m *meter) add(n uint64) {
	m.done += n
	if m.tty {
		if now := m.now(); now.Sub(m.drawn) >= 200*time.Millisecond {
			m.drawn = now
			fmt.Fprintf(m.w, "\r\033[K%s", m.line())
		}
		return
	}
	if tenth := m.fraction() * 10 / 1000; tenth > m.tenth {
		m.tenth = tenth
		fmt.Fprintln(m.w, m.line())
	}
}

// finish ends the line a terminal was redrawing.
func (m *meter) finish() {
	if m.tty {
		m.done = max(m.done, m.total)
		fmt.Fprintf(m.w, "\r\033[K%s\n", m.line())
	}
}

// fraction is how much is done, in thousandths. The total is an estimate, so
// what is done can pass it; the meter then waits at the end rather than
// counting past it.
func (m *meter) fraction() uint64 {
	if m.total == 0 {
		return 0
	}
	return min(m.done*1000/m.total, 999)
}

func (m *meter) line() string {
	f := m.fraction()
	if m.done >= m.total {
		f = 1000
	}
	const width = 24
	filled := int(f * width / 1000)
	bar := make([]rune, width)
	for i := range bar {
		bar[i] = '░'
		if i < filled {
			bar[i] = '█'
		}
	}

	line := fmt.Sprintf("  %s %s %3d%% · %s of %s", m.label, string(bar), f/10,
		host.FormatSize(min(m.done, m.total)), host.FormatSize(m.total))
	if elapsed := m.now().Sub(m.start); f > 0 && f < 1000 && elapsed > 2*time.Second {
		left := time.Duration(float64(elapsed) * float64(1000-f) / float64(f))
		line += " · about " + left.Round(time.Second).String() + " left"
	}

	return line
}

// countingReader counts what is read through it onto a meter.
type countingReader struct {
	r io.Reader
	m *meter
}

func (c countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.m.add(uint64(n))
	return n, err
}

// checkpoints reads what tar writes to its standard error while it archives:
// a dot for every checkpointBytes of archive, which go onto a meter, and
// anything else, which is kept as what tar had to say.
type checkpoints struct {
	m     *meter
	other []byte
	// inLine is set inside a message, where a dot is punctuation: tar's
	// dots stand alone between messages, and a message runs to its newline.
	inLine bool
}

// checkpointRecords is how many records tar writes between dots. A record is
// 20 blocks of 512 bytes, tar's default blocking factor, so a dot is 1000KiB
// of the archive — which is the home's own size plus
// a header for each file, and so what the total is estimated in.
const (
	checkpointRecords = 100
	checkpointBytes   = checkpointRecords * 20 * 512
)

func (c *checkpoints) Write(p []byte) (int, error) {
	for _, b := range p {
		if b == '.' && !c.inLine {
			c.m.add(checkpointBytes)
			continue
		}
		c.other = append(c.other, b)
		c.inLine = b != '\n'
	}
	return len(p), nil
}
