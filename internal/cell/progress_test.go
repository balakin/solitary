package cell

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// tar's dots are progress and everything else is what it had to say, dots in
// its sentences included.
func TestCheckpointsSplitProgressFromMessages(t *testing.T) {
	var out bytes.Buffer
	c := &checkpoints{m: newMeter(&out, "saving", 10*checkpointBytes)}

	for _, chunk := range []string{"...", "tar: ./sock: socket ignored.\n", "..", "tar: done. Bye.\n"} {
		if _, err := c.Write([]byte(chunk)); err != nil {
			t.Fatal(err)
		}
	}
	if c.m.done != 5*checkpointBytes {
		t.Errorf("counted %d bytes, want %d", c.m.done, 5*checkpointBytes)
	}
	if got, want := string(c.other), "tar: ./sock: socket ignored.\ntar: done. Bye.\n"; got != want {
		t.Errorf("kept %q, want %q", got, want)
	}
}

// Off a terminal the meter writes a line at each tenth and no more, and an
// estimate that falls short stops at the end rather than passing it.
func TestMeterOffATerminalWritesTenths(t *testing.T) {
	var out bytes.Buffer
	m := newMeter(&out, "saving", 1000)
	clock := m.start
	m.now = func() time.Time { return clock }

	for range 30 {
		m.add(50)
		clock = clock.Add(time.Second)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 9 {
		t.Errorf("wrote %d lines, want one for each tenth up to 90%%:\n%s", len(lines), out.String())
	}
	if last := lines[len(lines)-1]; !strings.Contains(last, " 90% ·") {
		t.Errorf("last line = %q, want it at 90%%", last)
	}
	if strings.Contains(out.String(), "\r") {
		t.Error("the meter redraws in place where nothing can be redrawn")
	}
	if got := m.line(); !strings.Contains(got, "100%") || strings.Contains(got, "left") {
		t.Errorf("line() past the estimate = %q, want 100%% and no time left", got)
	}
}
