package dashboard

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dm-balakin/solitary/internal/cell"
	"github.com/dm-balakin/solitary/internal/config"
)

// press sends one key, the way a person would.
func press(t *testing.T, m model, key string) (model, tea.Cmd) {
	t.Helper()

	var msg tea.KeyMsg
	switch key {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	case "up":
		msg = tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		msg = tea.KeyMsg{Type: tea.KeyDown}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}

	next, cmd := m.Update(msg)

	return next.(model), cmd
}

// listed is a model showing two cells, as it would be after the first refresh.
func listed(t *testing.T) model {
	t.Helper()

	m := newModel()
	next, _ := m.Update(cellsMsg{cells: []cell.Info{
		{Name: "claude", Status: cell.StatusRunning, Image: "build:./Containerfile"},
		{Name: "scratch", Status: cell.StatusUninitialized, Image: "ubuntu:24.04"},
	}})

	return next.(model)
}

func TestListingKeepsTheSelectionOnTheSameCell(t *testing.T) {
	m := listed(t)

	m, _ = press(t, m, "down")
	if got := m.selected().Name; got != "scratch" {
		t.Fatalf("after moving down, selected = %q", got)
	}

	// A refresh arrives with the cells in a different order.
	next, _ := m.Update(cellsMsg{cells: []cell.Info{
		{Name: "scratch", Status: cell.StatusStopped},
		{Name: "claude", Status: cell.StatusRunning},
	}})
	m = next.(model)

	if got := m.selected().Name; got != "scratch" {
		t.Errorf("refresh moved the selection to %q, want it to stay on scratch", got)
	}
}

// Destroying a machine is the one irreversible thing here, so it always asks
// first, and anything other than yes means no.
func TestRemoveAsksFirst(t *testing.T) {
	m := listed(t)

	m, cmd := press(t, m, "d")
	if m.mode != confirming {
		t.Fatalf("mode = %v after d, want confirming", m.mode)
	}
	if cmd != nil {
		t.Error("d ran something before the question was answered")
	}
	if !strings.Contains(m.View(), "Everything inside it is lost") {
		t.Error("the question does not say what is lost")
	}

	m, cmd = press(t, m, "n")
	if m.mode != browsing || cmd != nil {
		t.Errorf("answering no left mode = %v with a command = %v", m.mode, cmd != nil)
	}
	if m.notice != "Cancelled." {
		t.Errorf("notice = %q, want it to say nothing happened", m.notice)
	}
}

// A cell with no machine has nothing to destroy or stop.
func TestActionsThatNeedAMachine(t *testing.T) {
	m := listed(t)
	m, _ = press(t, m, "down") // scratch, uninitialized

	m, _ = press(t, m, "d")
	if m.mode == confirming {
		t.Error("offered to destroy a cell that has no machine")
	}
	if m.failure == nil {
		t.Error("said nothing about why nothing happened")
	}

	if _, cmd := press(t, m, "s"); cmd != nil {
		t.Error("tried to stop a cell that has no machine")
	}
}

// Attaching to a cell that is not running would fail in a way the dashboard can
// explain better than the shell can.
func TestAttachNeedsARunningCell(t *testing.T) {
	m := listed(t)
	m, _ = press(t, m, "down")

	m, cmd := press(t, m, "enter")
	if cmd != nil {
		t.Error("tried to attach to a cell that is not running")
	}
	if m.failure == nil || !strings.Contains(m.failure.Error(), "press u to start it") {
		t.Errorf("failure = %v, want it to say how to fix this", m.failure)
	}
}

func TestSecretsViewNeverShowsAValue(t *testing.T) {
	m := listed(t)
	next, _ := m.Update(detailMsg{detail: cell.Detail{
		Name: "claude",
		Secrets: []cell.SecretState{
			{Name: "GH_TOKEN", Set: true},
			{Name: "CLAUDE_CODE_OAUTH_TOKEN", Set: false},
		},
	}})
	m = next.(model)

	m, _ = press(t, m, "e")
	if m.mode != managingSecrets {
		t.Fatalf("mode = %v after e, want the secrets view", m.mode)
	}

	view := m.View()
	if !strings.Contains(view, "GH_TOKEN") || !strings.Contains(view, "set") || !strings.Contains(view, "not set") {
		t.Errorf("secrets view does not report which are set:\n%s", view)
	}

	// Type a value: it must not appear on screen, then or after.
	m, _ = press(t, m, "enter")
	if m.mode != typing {
		t.Fatalf("mode = %v after enter, want typing", m.mode)
	}
	for _, r := range "sk-ant-secret" {
		m, _ = press(t, m, string(r))
	}
	if strings.Contains(m.View(), "sk-ant-secret") {
		t.Error("the value being typed is on screen")
	}

	// Cancelling forgets it rather than keeping it for the next time.
	m, _ = press(t, m, "esc")
	if m.input.Value() != "" {
		t.Error("cancelling kept the typed value")
	}
}

// Saving nothing must not overwrite a secret with an empty value.
func TestEmptyValueChangesNothing(t *testing.T) {
	m := listed(t)
	next, _ := m.Update(detailMsg{detail: cell.Detail{
		Name:    "claude",
		Secrets: []cell.SecretState{{Name: "GH_TOKEN", Set: true}},
	}})
	m = next.(model)

	m, _ = press(t, m, "e")
	m, _ = press(t, m, "enter")

	m, cmd := press(t, m, "enter")
	if cmd != nil {
		t.Error("an empty value was saved")
	}
	if m.mode != managingSecrets {
		t.Errorf("mode = %v, want to be back in the secrets view", m.mode)
	}
}

// Until the first listing arrives, an empty screen means "not known yet", not
// "no cells".
func TestUnloadedIsNotEmpty(t *testing.T) {
	m := newModel()
	if strings.Contains(m.View(), "No cells yet") {
		t.Error("claimed there are no cells before looking")
	}

	next, _ := m.Update(cellsMsg{cells: nil})
	if !strings.Contains(next.(model).View(), "No cells yet") {
		t.Error("an empty listing should say there are no cells")
	}
}

// recorder stands in for running solitary's own commands, so a test can see
// what would be run.
type recorder struct{ calls [][]string }

func (r *recorder) runner() runner {
	return func(name, command, _ string, args ...string) tea.Cmd {
		r.calls = append(r.calls, append([]string{command, name}, args...))
		return nil
	}
}

// The dashboard is the thing attaching; up must not open a shell behind it, and
// rm must not ask a second time from behind a suspended screen.
func TestLifecycleKeysRunTheRightCommands(t *testing.T) {
	for _, tc := range []struct {
		keys []string
		want []string
	}{
		{[]string{"u"}, []string{"up", "claude", "--detach"}},
		{[]string{"s"}, []string{"down", "claude"}},
		{[]string{"d", "y"}, []string{"rm", "claude", "--force"}},
	} {
		var rec recorder
		m := listed(t)
		m.run = rec.runner()

		for _, key := range tc.keys {
			m, _ = press(t, m, key)
		}

		if len(rec.calls) != 1 {
			t.Fatalf("%v ran %d commands, want 1", tc.keys, len(rec.calls))
		}
		if got := rec.calls[0]; !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%v ran %q, want %q", tc.keys, got, tc.want)
		}
	}
}

// Answering no must not run anything at all.
func TestCancelledRemoveRunsNothing(t *testing.T) {
	var rec recorder
	m := listed(t)
	m.run = rec.runner()

	m, _ = press(t, m, "d")
	if _, _ = press(t, m, "n"); len(rec.calls) != 0 {
		t.Errorf("cancelling ran %q", rec.calls)
	}
}

// A cell that restricts nothing must say so as plainly as one that does: the
// absence of an allow list is the thing worth noticing.
func TestNetworkPreview(t *testing.T) {
	m := listed(t)

	next, _ := m.Update(detailMsg{detail: cell.Detail{Name: "claude"}})
	if view := next.(model).View(); !strings.Contains(view, "unrestricted") {
		t.Errorf("a cell with no allow list does not say so:\n%s", view)
	}

	next, _ = m.Update(detailMsg{detail: cell.Detail{
		Name: "claude",
		Network: config.Network{Allow: []string{
			"github.com", "api.anthropic.com", "registry.npmjs.org",
			"deb.debian.org", "objects.githubusercontent.com", "10.1.2.0/24",
		}},
	}})
	view := next.(model).View()

	if !strings.Contains(view, "6 allowed") {
		t.Errorf("the allow list is not counted:\n%s", view)
	}
	if !strings.Contains(view, "github.com") {
		t.Errorf("the allow list is not previewed:\n%s", view)
	}
	// Six entries would crowd out everything else, so the rest are summarised.
	if !strings.Contains(view, "+2 more") {
		t.Errorf("a long allow list is not summarised:\n%s", view)
	}
	if strings.Contains(view, "10.1.2.0/24") {
		t.Errorf("the whole list was shown rather than a preview:\n%s", view)
	}
}
