package dashboard

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/balakin/solitary/internal/cell"
	"github.com/balakin/solitary/internal/config"
)

func resized(m model, width, height int) model {
	next, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return next.(model)
}

func assertFits(t *testing.T, m model) {
	t.Helper()
	view := m.View()
	if lipgloss.Height(view) > m.height || lipgloss.Width(view) > m.width {
		t.Fatalf("view is %dx%d, terminal %dx%d:\n%s", lipgloss.Width(view), lipgloss.Height(view), m.width, m.height, view)
	}
}

func TestDashboardFitsTerminal(t *testing.T) {
	for _, size := range [][2]int{{120, 24}, {100, 10}, {80, 12}, {40, 8}, {20, 6}, {12, 4}} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			m := resized(listed(t), size[0], size[1])
			m.detail = cell.Detail{Name: "claude", Description: strings.Repeat("description ", 20)}
			for _, state := range []mode{browsing, viewingNetwork, viewingReport, viewingHelp, managingSecrets, typing, watchingTraffic, filteringTraffic, confirming} {
				m.mode = state
				m.asking = question{prompt: removePrompt("claude"), verb: "destroy"}
				assertFits(t, m)
			}
		})
	}
}

func TestCellSelectionScrollsAndPages(t *testing.T) {
	m := resized(listed(t), 80, 10)
	for i := 0; i < 40; i++ {
		m.cells = append(m.cells, cell.Info{Name: fmt.Sprintf("cell-%02d", i), Status: cell.StatusStopped})
	}
	for range 30 {
		m, _ = press(t, m, "down")
	}
	if view := m.View(); !strings.Contains(view, "› ○ cell-28") || !strings.Contains(view, "? keys") {
		t.Fatalf("selection or controls missing:\n%s", view)
	}
	assertFits(t, m)
	m, _ = press(t, m, "end")
	if m.cursor != len(m.cells)-1 {
		t.Fatal("End did not select last cell")
	}
	m, _ = press(t, m, "pgup")
	if m.cursor != len(m.cells)-1-m.listCapacity() {
		t.Fatal("page up did not move a page")
	}
	m = resized(m, 120, 7)
	assertFits(t, m)
	if !strings.Contains(m.View(), m.selected().Name) {
		t.Fatal("resize hid selection")
	}
}

func TestDetailsScrollWithoutChangingCell(t *testing.T) {
	m := resized(listed(t), 60, 10)
	m.detail = cell.Detail{Name: "claude", Network: config.Network{Allow: []string{"first.example", "second.example", "third.example", "fourth.example", "last.example"}}}
	m, _ = press(t, m, "n")
	m, _ = press(t, m, "end")
	if m.mode != viewingNetwork || !strings.Contains(m.View(), "share access.") {
		t.Fatalf("cannot reach last network line:\n%s", m.View())
	}
	offset := m.detailOffset
	m, _ = press(t, m, "down")
	if m.detailOffset != offset {
		t.Fatal("scrolled past end")
	}
	m, _ = press(t, m, "up")
	if m.detailOffset != offset-1 {
		t.Fatal("scrolling back from end did not respond")
	}
	m, _ = press(t, m, "esc")
	m, _ = press(t, m, "tab")
	m, _ = press(t, m, "down")
	if m.cursor != 0 {
		t.Fatal("scrolling details moved cell selection")
	}
}

func TestHelpAndTrafficUseAvailableHeight(t *testing.T) {
	m := resized(listed(t), 80, 12)
	m, _ = press(t, m, "?")
	m, _ = press(t, m, "end")
	if !strings.Contains(m.View(), "Secrets:") {
		t.Fatalf("help cannot reach final action:\n%s", m.View())
	}
	m = watched(t, feed...)
	m = resized(m, 100, 12)
	for i := 0; i < 30; i++ {
		m.traffic = add(m.traffic, cell.Traffic{Detail: fmt.Sprint(i), Kind: cell.TrafficQuery})
	}
	assertFits(t, m)
	if !strings.Contains(m.View(), "live") {
		t.Fatal("traffic status hidden")
	}
	m, _ = press(t, m, "pgup")
	if m.offset != m.trafficCapacity() {
		t.Fatal("traffic did not page by visible rows")
	}
}

func TestSecretsSelectionAndInputStayVisible(t *testing.T) {
	m := resized(listed(t), 60, 10)
	m.detail.Name = "claude"
	for i := 0; i < 30; i++ {
		m.detail.Secrets = append(m.detail.Secrets, cell.SecretState{Name: fmt.Sprintf("TOKEN_%02d", i), Description: "A token for the selected service."})
	}
	m.mode = managingSecrets
	for range 29 {
		m, _ = press(t, m, "down")
	}
	if !strings.Contains(m.View(), "› TOKEN_29") {
		t.Fatalf("selected secret hidden:\n%s", m.View())
	}
	assertFits(t, m)
	m, _ = press(t, m, "enter")
	if !strings.Contains(m.View(), "value ") {
		t.Fatalf("input hidden:\n%s", m.View())
	}
	assertFits(t, m)
}

func TestSmallEmptyAndLoadingViews(t *testing.T) {
	for _, size := range [][2]int{{40, 8}, {20, 6}, {12, 4}} {
		m := resized(newModel(), size[0], size[1])
		assertFits(t, m)
		m.loaded = true
		assertFits(t, m)
	}
}

func TestConfirmationMustBeVisibleBeforeAccepting(t *testing.T) {
	var rec recorder
	m := resized(listed(t), 20, 6)
	m.run = rec.runner()
	m, _ = press(t, m, "d")
	m, _ = press(t, m, "y")
	if len(rec.calls) > 0 || m.mode != confirming {
		t.Fatal("accepted an unreadable confirmation")
	}
	m = resized(m, 80, 12)
	if !strings.Contains(m.View(), m.asking.prompt[:20]) {
		t.Fatal("confirmation missing after resize")
	}
	m, _ = press(t, m, "y")
	if len(rec.calls) != 1 {
		t.Fatal("visible confirmation was not accepted")
	}
}
