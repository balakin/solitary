package agents

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func unset(string) string { return "" }

func TestEveryAgentResolvesWithoutOverrides(t *testing.T) {
	for _, agent := range list {
		if dir := Expand(agent.Skills, "/home/u", unset); dir == "" || !filepath.IsAbs(dir) {
			t.Errorf("%s: skills directory %q expands to %q", agent.ID, agent.Skills, dir)
		}
	}
}

func TestExpandReadsTheAgentsOwnVariables(t *testing.T) {
	env := map[string]string{"CLAUDE_CONFIG_DIR": "/opt/claude", "XDG_CONFIG_HOME": "/cfg"}
	getenv := func(name string) string { return env[name] }

	for path, want := range map[string]string{
		"$CLAUDE_CONFIG_DIR/skills":    "/opt/claude/skills",
		"$CODEX_HOME":                  "/home/u/.codex",
		"$CONFIG/goose/skills":         "/cfg/goose/skills",
		"~/.kiro/skills":               "/home/u/.kiro/skills",
		"$FLATPAK_XDG_CONFIG_HOME/zed": "",
		"/etc/codex":                   "/etc/codex",
	} {
		if got := Expand(path, "/home/u", getenv); got != want {
			t.Errorf("Expand(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestVarsCoversEveryVariableTheListUses(t *testing.T) {
	vars := Vars()
	for _, want := range []string{"XDG_CONFIG_HOME", "CLAUDE_CONFIG_DIR", "CODEX_HOME", "FLATPAK_XDG_CONFIG_HOME"} {
		if !slices.Contains(vars, want) {
			t.Errorf("Vars = %v, missing %s", vars, want)
		}
	}
}

func TestIDsAreUniqueAndWellFormed(t *testing.T) {
	ids := IDs()
	if !slices.Contains(ids, "claude-code") || !slices.Contains(ids, "openclaw") {
		t.Errorf("IDs = %v, missing claude-code or openclaw", ids)
	}
	if len(slices.Compact(slices.Sorted(slices.Values(ids)))) != len(ids) {
		t.Error("IDs lists an agent twice")
	}
	for _, id := range ids {
		if strings.ToLower(id) != id || strings.ContainsAny(id, " _") {
			t.Errorf("ID %q is not lowercase with dashes", id)
		}
	}
	if !Known("codex") || Known("claude") {
		t.Error("Known disagrees with the list")
	}
}
