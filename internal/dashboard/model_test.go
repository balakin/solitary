package dashboard

import (
	"fmt"
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

// The preview is a preview; the whole list has to be readable somewhere, and
// the dashboard is where someone is already looking.
func TestNetworkViewShowsEveryEntry(t *testing.T) {
	allow := []string{
		"anthropic.com", "claude.ai", "mcp.context7.com", "github.com",
		"githubusercontent.com", "npmjs.org", "nodejs.org", "ubuntu.com",
		"docker.io", "docker.com",
	}
	m := listed(t)
	next, _ := m.Update(detailMsg{detail: cell.Detail{Name: "claude", Network: config.Network{Allow: allow}}})
	m = next.(model)

	m, _ = press(t, m, "n")
	if m.mode != viewingNetwork {
		t.Fatalf("mode = %v after n, want the network view", m.mode)
	}

	view := m.View()
	for _, entry := range allow {
		if !strings.Contains(view, entry) {
			t.Errorf("the network view leaves out %q:\n%s", entry, view)
		}
	}
	if strings.Contains(view, "more") {
		t.Error("the network view summarises instead of showing everything")
	}

	// Reading it changes nothing, and any key returns.
	if m, _ = press(t, m, "x"); m.mode != browsing {
		t.Errorf("mode = %v, want to be back in the list", m.mode)
	}
}

// Whether a cell leaves through a tunnel changes what its allow list means, so
// the list is not worth reading without it.
func TestNetworkViewShowsTheTunnel(t *testing.T) {
	m := listed(t)
	next, _ := m.Update(detailMsg{detail: cell.Detail{Name: "claude", Network: config.Network{
		Allow:  []string{"github.com"},
		VPN:    "./vpn.conf",
		Tunnel: &config.Tunnel{EndpointHost: "de-01.example.net", EndpointPort: "51820"},
	}}})
	m = next.(model)

	if view := m.View(); !strings.Contains(view, "tunnel") {
		t.Errorf("the cell pane does not say the cell is tunnelled:\n%s", view)
	}

	m, _ = press(t, m, "n")
	view := m.View()
	if !strings.Contains(view, "de-01.example.net") {
		t.Errorf("the network view does not name the peer:\n%s", view)
	}
	if !strings.Contains(view, "nothing leaves at all") {
		t.Errorf("the network view does not say what a tunnel being down means:\n%s", view)
	}
}

// An unrestricted cell has no list to read, and the view should say what that
// means rather than showing an empty pane.
func TestNetworkViewOfAnUnrestrictedCell(t *testing.T) {
	m := listed(t)
	next, _ := m.Update(detailMsg{detail: cell.Detail{Name: "claude"}})
	m, _ = press(t, next.(model), "n")

	if view := m.View(); !strings.Contains(view, "reaches whatever the host reaches") {
		t.Errorf("the network view does not explain an unrestricted cell:\n%s", view)
	}
}

// One lookup answers with every address a name has, and a refused program
// retries. Without folding, one event fills the pane and hides the rest.
func TestTrafficFoldsRepeats(t *testing.T) {
	var rows []trafficRow
	for _, entry := range []cell.Traffic{
		{At: "12:07:53", Kind: cell.TrafficResolved, Detail: "registry.npmjs.org → 104.16.2.34"},
		{At: "12:07:53", Kind: cell.TrafficResolved, Detail: "registry.npmjs.org → 104.16.0.34"},
		{At: "12:07:54", Kind: cell.TrafficResolved, Detail: "registry.npmjs.org → 104.16.5.34"},
		{At: "12:07:55", Kind: cell.TrafficRefused, Detail: "example.com"},
		{At: "12:07:56", Kind: cell.TrafficRefused, Detail: "example.com"},
		{At: "12:07:57", Kind: cell.TrafficResolved, Detail: "github.com → 140.82.121.4"},
	} {
		rows = add(rows, entry)
	}

	if len(rows) != 3 {
		t.Fatalf("folded into %d rows, want 3: %+v", len(rows), rows)
	}
	if rows[0].count != 3 || rows[1].count != 2 || rows[2].count != 1 {
		t.Errorf("counts = %d, %d, %d; want 3, 2, 1", rows[0].count, rows[1].count, rows[2].count)
	}
	// The newest time wins, so a folded row does not look stale.
	if rows[0].entry.At != "12:07:54" {
		t.Errorf("folded row shows %s, want the most recent time", rows[0].entry.At)
	}
}

func TestTrafficViewNeedsARunningCell(t *testing.T) {
	m := listed(t)
	m, _ = press(t, m, "down") // scratch, uninitialized

	m, cmd := press(t, m, "t")
	if cmd != nil || m.mode == watchingTraffic {
		t.Error("followed a cell that is not running")
	}
	if m.failure == nil || !strings.Contains(m.failure.Error(), "no traffic") {
		t.Errorf("failure = %v, want it to explain why", m.failure)
	}
}

// The view is live, so leaving it has to end the follow rather than leave a
// command reading a machine's log for nobody. Only esc and q leave: the other
// keys move around the feed, and losing it to a stray keystroke is worse than
// having to press escape.
func TestLeavingTrafficStopsFollowing(t *testing.T) {
	m := listed(t)
	m.mode = watchingTraffic
	m.traffic = []trafficRow{{entry: cell.Traffic{Kind: cell.TrafficDenied, Detail: "1.1.1.1:443"}, count: 1}}

	if stayed, _ := press(t, m, "x"); stayed.mode != watchingTraffic {
		t.Error("a key with nothing bound to it left the view")
	}

	m, _ = press(t, m, "esc")
	if m.mode != browsing {
		t.Errorf("mode = %v after esc, want to be back in the list", m.mode)
	}
	if m.stream != nil {
		t.Error("the follow was left running")
	}
}

// An entry arriving after the view is left must not be recorded.
func TestTrafficIgnoredWhenNotWatching(t *testing.T) {
	m := listed(t)
	next, _ := m.Update(trafficMsg{entry: cell.Traffic{Kind: cell.TrafficDenied, Detail: "1.1.1.1:443"}})

	if got := next.(model).traffic; len(got) != 0 {
		t.Errorf("recorded %+v while not watching", got)
	}
}

// watched is a traffic view holding a mixed feed.
func watched(t *testing.T, entries ...cell.Traffic) model {
	t.Helper()

	m := listed(t)
	m.mode = watchingTraffic
	for _, entry := range entries {
		m.traffic = add(m.traffic, entry)
	}

	return m
}

var feed = []cell.Traffic{
	{At: "12:00:01", Kind: cell.TrafficResolved, Detail: "api.github.com → 140.82.121.6"},
	{At: "12:00:02", Kind: cell.TrafficRefused, Detail: "example.com"},
	{At: "12:00:03", Kind: cell.TrafficQuery, Detail: "registry.npmjs.org"},
	{At: "12:00:04", Kind: cell.TrafficDenied, Detail: "1.1.1.1:443"},
}

// Someone opening this view after something failed wants the failures.
func TestTrafficFilters(t *testing.T) {
	m := watched(t, feed...)

	m, _ = press(t, m, "b")
	kinds := map[cell.TrafficKind]bool{}
	for _, row := range m.visibleTraffic() {
		kinds[row.entry.Kind] = true
	}
	if kinds[cell.TrafficResolved] || kinds[cell.TrafficQuery] {
		t.Errorf("refused-only still shows what succeeded: %v", kinds)
	}
	if len(m.visibleTraffic()) != 2 {
		t.Errorf("kept %d rows, want the 2 that were blocked", len(m.visibleTraffic()))
	}

	// Typed filters narrow by name, and match however they are cased.
	m, _ = press(t, m, "b") // off again
	m, _ = press(t, m, "/")
	if m.mode != filteringTraffic {
		t.Fatalf("mode = %v after /, want to be typing a filter", m.mode)
	}
	for _, r := range "GITHUB" {
		m, _ = press(t, m, string(r))
	}
	if got := m.visibleTraffic(); len(got) != 1 || !strings.Contains(got[0].entry.Detail, "github") {
		t.Errorf("filter kept %+v, want only the github row", got)
	}

	// Escape leaves the view showing everything again.
	m, _ = press(t, m, "esc")
	if len(m.visibleTraffic()) != len(feed) {
		t.Errorf("clearing the filter kept %d rows, want all %d", len(m.visibleTraffic()), len(feed))
	}
}

// Scrolling back has to hold the view still, and say that it is holding it.
func TestTrafficScrollingPausesTheFeed(t *testing.T) {
	m := watched(t, feed...)
	// More rows than fit, so there is something to scroll through.
	for i := 0; i < trafficShown*2; i++ {
		m.traffic = add(m.traffic, cell.Traffic{At: "12:01:00", Kind: cell.TrafficQuery, Detail: fmt.Sprintf("n%d.example.org", i)})
	}

	if !strings.Contains(m.View(), "live") {
		t.Error("a feed at the newest end does not say it is live")
	}

	m, _ = press(t, m, "up")
	if m.offset != 1 {
		t.Fatalf("offset = %d after scrolling up once, want 1", m.offset)
	}
	if view := m.View(); !strings.Contains(view, "paused · 1 newer") {
		t.Errorf("a held feed does not say so:\n%s", view)
	}

	// G returns to following, which is the way out of being paused.
	m, _ = press(t, m, "G")
	if m.offset != 0 || !strings.Contains(m.View(), "live") {
		t.Errorf("G left offset = %d", m.offset)
	}

	// Scrolling stops at the oldest rather than running off the end.
	m, _ = press(t, m, "g")
	before := m.offset
	m, _ = press(t, m, "up")
	if m.offset != before {
		t.Errorf("scrolled past the oldest row: %d then %d", before, m.offset)
	}
}
