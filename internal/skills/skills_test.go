package skills

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	shipped "github.com/balakin/solitary/skills"
)

// state points the record at a temp directory, so a test never reads or writes
// the real one.
func state(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))

	return root
}

func source(body string) fstest.MapFS {
	return fstest.MapFS{
		"solitary-one/SKILL.md":         {Data: []byte("---\nname: solitary-one\n---\n" + body)},
		"solitary-one/scripts/check.sh": {Data: []byte("#!/bin/sh\n")},
		"solitary-two/SKILL.md":         {Data: []byte("---\nname: solitary-two\n---\n")},
	}
}

func write(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestShippedSkillsAreWellFormed(t *testing.T) {
	names, err := Names(shipped.Host())
	if err != nil {
		t.Fatal(err)
	}
	if len(names) == 0 {
		t.Fatal("no skills embedded")
	}

	// The spec every agent reads requires the name to be the directory's.
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join("..", "..", "skills", "host", name, "SKILL.md"))
		if err != nil {
			t.Fatal(err)
		}
		want := "---\nname: " + name + "\ndescription: "
		if len(data) < len(want) || string(data[:len(want)]) != want {
			t.Errorf("%s/SKILL.md does not open with name: %s and a description", name, name)
		}
	}
}

// agentHome is a home with nothing configured and no override set, so that
// only what a test creates is detected.
func agentHome(t *testing.T, dirs ...string) string {
	t.Helper()

	home := t.TempDir()
	saved := system
	system = t.TempDir()
	t.Cleanup(func() { system = saved })
	t.Setenv("XDG_CONFIG_HOME", "")
	for name := range homes {
		t.Setenv(name, "")
	}
	t.Setenv("FLATPAK_XDG_CONFIG_HOME", "")
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(home, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

func TestDetectFindsConfiguredAgentsOnly(t *testing.T) {
	home := agentHome(t, ".claude", ".codex", ".warp", ".config/opencode")

	got := Detect(home)
	want := []Target{
		{Agents: []string{"Claude Code"}, Dir: filepath.Join(home, ".claude", "skills")},
		// Two agents reading one directory are one target.
		{Agents: []string{"Codex", "Warp"}, Dir: filepath.Join(home, ".agents", "skills")},
		{Agents: []string{"OpenCode"}, Dir: filepath.Join(home, ".config", "opencode", "skills")},
	}
	if !slices.EqualFunc(got, want, func(a, b Target) bool {
		return a.Dir == b.Dir && slices.Equal(a.Agents, b.Agents)
	}) {
		t.Errorf("Detect = %v, want %v", got, want)
	}

	if got := Detect(agentHome(t)); len(got) != 0 {
		t.Errorf("Detect in an empty home = %v, want nothing", got)
	}
}

func TestDetectHonoursTheAgentsOwnOverrides(t *testing.T) {
	home := agentHome(t)
	claude := filepath.Join(t.TempDir(), "claude")
	config := filepath.Join(t.TempDir(), "config")
	for _, dir := range []string{claude, filepath.Join(config, "goose")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("CLAUDE_CONFIG_DIR", claude)
	t.Setenv("XDG_CONFIG_HOME", config)

	var dirs []string
	for _, target := range Detect(home) {
		dirs = append(dirs, target.Dir)
	}
	want := []string{filepath.Join(claude, "skills"), filepath.Join(config, "goose", "skills")}
	if !slices.Equal(dirs, want) {
		t.Errorf("Detect = %v, want %v", dirs, want)
	}
}

func TestDetectTakesOneDirectoryPerAgent(t *testing.T) {
	// OpenClaw has had three names; with two of them present, the first wins.
	home := agentHome(t, ".clawdbot", ".moltbot")

	got := Detect(home)
	if len(got) != 1 || got[0].Dir != filepath.Join(home, ".clawdbot", "skills") {
		t.Errorf("Detect = %v, want ~/.clawdbot/skills alone", got)
	}
}

func TestEveryAgentResolvesWithoutOverrides(t *testing.T) {
	home := agentHome(t)
	for _, agent := range agents {
		if dir := resolve(home, agent.skills); dir == "" || !filepath.IsAbs(dir) {
			t.Errorf("%s: skills directory %q resolves to %q", agent.name, agent.skills, dir)
		}
	}
}

func TestInstallReplacesItsOwnAndLeavesOthersAlone(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "mine", "SKILL.md"), "someone else's")
	write(t, filepath.Join(dir, "solitary-renamed", "SKILL.md"), "from an older release")
	write(t, filepath.Join(dir, "solitary-one", "SKILL.md"), "an old copy")
	write(t, filepath.Join(dir, "solitary-one", "stale.md"), "a file the new copy lacks")

	if err := Install(source("new"), dir); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "solitary-one", "SKILL.md"))
	if err != nil || string(got) != "---\nname: solitary-one\n---\nnew" {
		t.Errorf("solitary-one/SKILL.md = %q, %v; want the new copy", got, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "solitary-one", "stale.md")); !os.IsNotExist(err) {
		t.Error("a file from the old copy survived the replacement")
	}
	if _, err := os.Stat(filepath.Join(dir, "solitary-one", "scripts", "check.sh")); err != nil {
		t.Errorf("nested file not installed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "solitary-renamed")); !os.IsNotExist(err) {
		t.Error("a skill this binary no longer ships was left behind")
	}
	if got, err := os.ReadFile(filepath.Join(dir, "mine", "SKILL.md")); err != nil || string(got) != "someone else's" {
		t.Errorf("another skill was touched: %q, %v", got, err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if want := []string{"mine", "solitary-one", "solitary-two"}; !slices.Equal(names, want) {
		t.Errorf("directory holds %v, want %v and no staging left behind", names, want)
	}
}

func TestStatusComparesWithTheBinary(t *testing.T) {
	dir := t.TempDir()
	if err := Install(source("v1"), dir); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(dir, "solitary-two")); err != nil {
		t.Fatal(err)
	}

	status, err := Status(source("v2"), dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []Skill{{"solitary-one", Outdated}, {"solitary-two", Missing}}
	if !slices.Equal(status, want) {
		t.Errorf("Status = %v, want %v", status, want)
	}

	if status, _ := Status(source("v1"), dir); status[0].State != Current {
		t.Errorf("Status of an identical copy = %v, want current", status[0].State)
	}
}

func TestRemoveTakesOnlyItsOwn(t *testing.T) {
	dir := t.TempDir()
	if err := Install(source(""), dir); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "mine", "SKILL.md"), "someone else's")
	write(t, filepath.Join(dir, "solitary-notes", "README"), "prefixed, but not a skill")

	names, err := Remove(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"solitary-one", "solitary-two"}; !slices.Equal(names, want) {
		t.Errorf("Remove = %v, want %v", names, want)
	}
	for _, kept := range []string{"mine", "solitary-notes"} {
		if _, err := os.Stat(filepath.Join(dir, kept)); err != nil {
			t.Errorf("%s was removed: %v", kept, err)
		}
	}

	if names, err := Remove(filepath.Join(dir, "absent")); err != nil || names != nil {
		t.Errorf("Remove of a missing directory = %v, %v; want nothing", names, err)
	}
}

func TestRecordRoundTrips(t *testing.T) {
	state(t)

	if dirs, err := Recorded(); err != nil || dirs != nil {
		t.Fatalf("Recorded before anything = %v, %v; want nothing", dirs, err)
	}
	want := []string{"/a/skills", "/b/skills"}
	if err := Record(want); err != nil {
		t.Fatal(err)
	}
	if dirs, err := Recorded(); err != nil || !slices.Equal(dirs, want) {
		t.Errorf("Recorded = %v, %v; want %v", dirs, err, want)
	}
	if err := Record(nil); err != nil {
		t.Fatal(err)
	}
	if dirs, err := Recorded(); err != nil || dirs != nil {
		t.Errorf("Recorded after clearing = %v, %v; want nothing", dirs, err)
	}
}

func TestNamedInstallsForAnAgentThatHasNotRunYet(t *testing.T) {
	home := agentHome(t)

	got, err := Named(home, []string{"claude-code", "codex", "warp"})
	if err != nil {
		t.Fatal(err)
	}
	want := []Target{
		{Agents: []string{"Claude Code"}, Dir: filepath.Join(home, ".claude", "skills")},
		{Agents: []string{"Codex", "Warp"}, Dir: filepath.Join(home, ".agents", "skills")},
	}
	if !slices.EqualFunc(got, want, func(a, b Target) bool {
		return a.Dir == b.Dir && slices.Equal(a.Agents, b.Agents)
	}) {
		t.Errorf("Named = %v, want %v", got, want)
	}
}

func TestNamedPrefersTheDirectoryAlreadyInUse(t *testing.T) {
	home := agentHome(t, ".moltbot")

	got, err := Named(home, []string{"openclaw"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Dir != filepath.Join(home, ".moltbot", "skills") {
		t.Errorf("Named = %v, want ~/.moltbot/skills", got)
	}

	got, err = Named(agentHome(t), []string{"openclaw"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !strings.HasSuffix(got[0].Dir, filepath.Join(".openclaw", "skills")) {
		t.Errorf("Named with nothing configured = %v, want ~/.openclaw/skills", got)
	}
}

func TestNamedRefusesAnUnknownAgent(t *testing.T) {
	_, err := Named(agentHome(t), []string{"claude"})
	if err == nil || !strings.Contains(err.Error(), `unknown agent "claude"`) || !strings.Contains(err.Error(), "claude-code") {
		t.Errorf("Named(claude) error = %v, want one naming the agent and listing the known ones", err)
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
}
