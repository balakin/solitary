package cell

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/balakin/solitary/internal/agents"
	"github.com/balakin/solitary/internal/config"
	"github.com/balakin/solitary/internal/podman"
)

// fakePlace is a container with the given paths present and no variables set.
func fakePlace(present ...string) agents.Place {
	return agents.Place{
		Expand: func(p string) string {
			return agents.Expand(p, podman.HomeDir, func(string) string { return "" })
		},
		Exists: func(p string) bool { return slices.Contains(present, p) },
	}
}

func dirsOf(targets []agents.Target) []string {
	var dirs []string
	for _, target := range targets {
		dirs = append(dirs, target.Dir)
	}
	return dirs
}

func TestSkillTargetsFollowAgents(t *testing.T) {
	place := fakePlace("/home/cell/.claude", "/home/cell/.warp")

	detected, err := skillTargets(config.Agents{}, place)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"/home/cell/.claude/skills", "/home/cell/.agents/skills"}; !slices.Equal(dirsOf(detected), want) {
		t.Errorf("detected = %v, want %v", dirsOf(detected), want)
	}

	none, err := skillTargets(config.Agents{Set: true}, place)
	if err != nil || len(none) != 0 {
		t.Errorf("agents: [] = %v, %v; want nothing", none, err)
	}

	listed, err := skillTargets(config.Agents{Set: true, List: []config.Agent{
		{Name: "kiro-cli"}, // not present, installed for anyway
		{Name: "claude-code", Skills: ".config/claude/skills"},
		{Name: "my-agent", Skills: "my-agent/skills"},
	}}, place)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/home/cell/.kiro/skills", "/home/cell/.config/claude/skills", "/home/cell/my-agent/skills"}
	if !slices.Equal(dirsOf(listed), want) {
		t.Errorf("listed = %v, want %v", dirsOf(listed), want)
	}

	all, err := skillTargets(config.Agents{Set: true, All: true}, place)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) < 60 || !slices.Contains(dirsOf(all), "/home/cell/.kiro/skills") {
		t.Errorf("all = %d directories, want every agent's", len(all))
	}
	if len(slices.Compact(slices.Sorted(slices.Values(dirsOf(all))))) != len(all) {
		t.Error("all lists a directory twice")
	}
}

func TestInHomeKeepsToTheHome(t *testing.T) {
	for p, want := range map[string]string{
		"/home/cell/.claude/skills": ".claude/skills",
		"/home/cell/../etc":         "",
		"/home/cell":                "",
		"/opt/claude/skills":        "",
		"/home/cellar/skills":       "",
	} {
		got, ok := inHome(p)
		if got != want || ok != (want != "") {
			t.Errorf("inHome(%q) = %q, %v; want %q", p, got, ok, want)
		}
	}
}

func TestSkillsRecordRoundTripsAndForgets(t *testing.T) {
	writeCell(t, "image: alpine\n", "")

	if record, err := readSkillsRecord("probe"); err != nil || len(record.Dirs) != 0 {
		t.Fatalf("record before anything = %+v, %v", record, err)
	}
	want := skillsRecord{Dirs: []string{".claude/skills"}, Created: []string{".kiro"}}
	if err := writeSkillsRecord("probe", want); err != nil {
		t.Fatal(err)
	}
	if got, err := readSkillsRecord("probe"); err != nil || !slices.Equal(got.Dirs, want.Dirs) || !slices.Equal(got.Created, want.Created) {
		t.Errorf("record = %+v, %v; want %+v", got, err, want)
	}

	// What up made is kept after the skills are gone, so that detection
	// still knows those directories for what they are.
	if err := writeSkillsRecord("probe", skillsRecord{Created: want.Created}); err != nil {
		t.Fatal(err)
	}
	if got, _ := readSkillsRecord("probe"); !slices.Equal(got.Created, want.Created) {
		t.Errorf("record with only created = %+v", got)
	}

	if err := removeSkillsRecord("probe"); err != nil {
		t.Fatal(err)
	}
	if got, _ := readSkillsRecord("probe"); len(got.Created) != 0 {
		t.Errorf("record after removal = %+v", got)
	}
}

func TestSkillFilesCarryContentsAndModes(t *testing.T) {
	files, err := skillFiles(fstest.MapFS{
		"solitary-a/SKILL.md":       {Data: []byte("---\nname: solitary-a\n---\n")},
		"solitary-a/scripts/run.sh": {Data: []byte("#!/bin/sh\n")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if files["solitary-a"]["SKILL.md"].Mode != "0644" || files["solitary-a"]["scripts/run.sh"].Mode != "0755" {
		t.Errorf("modes = %+v", files["solitary-a"])
	}
}

// helper runs the guest script on this machine, against a directory standing in
// for the cell's home.
func helper(t *testing.T, home, action string, request, response any) {
	t.Helper()

	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not available")
	}
	input, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(python, "-I", "-c", skillsHelper, action, home)
	cmd.Stdin = bytes.NewReader(input)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s: %v\n%s", action, err, stderr.String())
	}
	if err := json.Unmarshal(out, response); err != nil {
		t.Fatalf("%s: %v in %q", action, err, out)
	}
}

type syncResult struct {
	Written, Skipped, Created []string
}

func TestHelperSyncsInstallsAndClearsWithinTheHome(t *testing.T) {
	home := t.TempDir()
	outside := t.TempDir()
	mustMkdir(t, filepath.Join(home, ".claude"))
	mustWrite(t, filepath.Join(home, ".claude", "settings.json"), "{}")
	mustWrite(t, filepath.Join(home, ".claude", "skills", "solitary-retired", "SKILL.md"), "from an older solitary")
	mustWrite(t, filepath.Join(home, ".claude", "skills", "mine", "SKILL.md"), "the user's")
	if err := os.Symlink(outside, filepath.Join(home, ".evil")); err != nil {
		t.Skip("symlinks not supported")
	}

	skills, err := skillFiles(fstest.MapFS{"solitary-a/SKILL.md": {Data: []byte("new")}})
	if err != nil {
		t.Fatal(err)
	}

	var first syncResult
	helper(t, home, "sync", map[string]any{
		"skills":   skills,
		"targets":  []string{".claude/skills", ".kiro/skills", ".evil/skills"},
		"previous": []string{},
		"created":  []string{},
	}, &first)

	if !slices.Equal(first.Written, []string{".claude/skills", ".kiro/skills"}) || !slices.Equal(first.Skipped, []string{".evil/skills"}) {
		t.Errorf("written %v, skipped %v", first.Written, first.Skipped)
	}
	if !slices.Equal(first.Created, []string{".kiro", ".kiro/skills"}) {
		t.Errorf("created = %v, want only what did not exist", first.Created)
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Errorf("wrote through a symlink: %v", entries)
	}
	if data, err := os.ReadFile(filepath.Join(home, ".kiro", "skills", "solitary-a", "SKILL.md")); err != nil || string(data) != "new" {
		t.Errorf("installed skill = %q, %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "solitary-retired")); !os.IsNotExist(err) {
		t.Error("a skill this binary no longer ships was left behind")
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "mine", "SKILL.md")); err != nil {
		t.Errorf("the user's own skill was touched: %v", err)
	}

	// The footprint: .kiro is all solitary's, .claude never was.
	var ours []string
	helper(t, home, "footprint", map[string]any{
		"markers": []string{".claude", ".kiro"},
		"created": first.Created,
	}, &ours)
	if !slices.Equal(ours, []string{".kiro"}) {
		t.Errorf("footprint = %v, want [.kiro]", ours)
	}

	// Narrowed to Claude: Kiro's copy goes, its directories stay.
	var second syncResult
	helper(t, home, "sync", map[string]any{
		"skills":   skills,
		"targets":  []string{".claude/skills"},
		"previous": first.Written,
		"created":  first.Created,
	}, &second)
	if entries, err := os.ReadDir(filepath.Join(home, ".kiro", "skills")); err != nil || len(entries) != 0 {
		t.Errorf("~/.kiro/skills after narrowing = %v, %v; want it empty and still there", entries, err)
	}
	if !slices.Equal(second.Created, first.Created) {
		t.Errorf("created after narrowing = %v, want %v kept", second.Created, first.Created)
	}

	// Once the agent writes something of its own there, it is an agent.
	mustWrite(t, filepath.Join(home, ".kiro", "settings.json"), "{}")
	helper(t, home, "footprint", map[string]any{
		"markers": []string{".kiro"},
		"created": second.Created,
	}, &ours)
	if len(ours) != 0 {
		t.Errorf("footprint after the agent wrote = %v, want none", ours)
	}

	// A directory removed by hand is forgotten.
	if err := os.RemoveAll(filepath.Join(home, ".kiro")); err != nil {
		t.Fatal(err)
	}
	var third syncResult
	helper(t, home, "sync", map[string]any{
		"skills":   skills,
		"targets":  []string{".claude/skills"},
		"previous": second.Written,
		"created":  second.Created,
	}, &third)
	if slices.ContainsFunc(third.Created, func(p string) bool { return strings.HasPrefix(p, ".kiro") }) {
		t.Errorf("created after removal = %v, want .kiro forgotten", third.Created)
	}
}

func TestHelperRefusesPathsLeavingTheHome(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not available")
	}
	input, _ := json.Marshal(map[string]any{
		"skills": map[string]any{}, "targets": []string{"../escape"}, "previous": []string{}, "created": []string{},
	})
	cmd := exec.Command("python3", "-I", "-c", skillsHelper, "sync", t.TempDir())
	cmd.Stdin = bytes.NewReader(input)
	if err := cmd.Run(); err == nil {
		t.Error("a target with .. was accepted")
	}
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, file, data string) {
	t.Helper()
	mustMkdir(t, path.Dir(filepath.ToSlash(file)))
	if err := os.WriteFile(file, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCellSkillsAreWellFormed(t *testing.T) {
	src, err := fs.Sub(cellSkills, "skills")
	if err != nil {
		t.Fatal(err)
	}
	entries, err := fs.ReadDir(src, ".")
	if err != nil || len(entries) != 4 {
		t.Fatalf("%d cell skills, %v; want 4", len(entries), err)
	}
	for _, entry := range entries {
		data, err := fs.ReadFile(src, entry.Name()+"/SKILL.md")
		if err != nil {
			t.Errorf("%s: %v", entry.Name(), err)
			continue
		}
		head, _, ok := strings.Cut(strings.TrimPrefix(string(data), "---\n"), "\n---\n")
		lines := strings.Split(head, "\n")
		if !strings.HasPrefix(entry.Name(), "solitary-") || !ok || len(lines) != 2 || lines[0] != "name: "+entry.Name() || !strings.HasPrefix(lines[1], "description: ") {
			t.Errorf("%s/SKILL.md frontmatter is %q, want only its name and a description", entry.Name(), head)
		}
	}
}
