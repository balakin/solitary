// Package skills installs solitary's own agent skills into the directories
// agents read them from. It owns the "solitary-" prefix in those directories and
// nothing else: a skill of anyone else's is never written over or removed.
package skills

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/balakin/solitary/internal/agents"
	"github.com/balakin/solitary/internal/config"
)

// Prefix opens the name of every skill solitary ships, and is how one is told
// apart from the skills beside it.
const Prefix = "solitary-"

// Target is one directory skills are installed into.
type Target = agents.Target

// system is where an absolute marker is looked for, and is only ever moved by a
// test: a marker like /etc/codex is a fact about the machine running it.
var system = "/"

// host is this machine as a place to look for agents.
func host(home string) agents.Place {
	return agents.Place{
		Expand: func(path string) string {
			expanded := agents.Expand(path, home, os.Getenv)
			if expanded != "" && filepath.IsAbs(path) {
				expanded = filepath.Join(system, expanded)
			}
			return expanded
		},
		Exists: func(path string) bool {
			_, err := os.Stat(path)
			return err == nil
		},
	}
}

// Detect returns the skills directory of every agent configured under home.
func Detect(home string) []Target {
	return agents.Detect(host(home))
}

// Named returns the skills directory of each agent named by its ID.
func Named(home string, ids []string) ([]Target, error) {
	return agents.Named(host(home), ids)
}

// Names lists the skills in src, sorted.
func Names(src fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(src, ".")
	if err != nil {
		return nil, fmt.Errorf("reading embedded skills: %w", err)
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), Prefix) {
			names = append(names, entry.Name())
		}
	}
	return names, nil
}

// Install writes every skill in src into dir, replacing the copy already there,
// and removes any of solitary's skills that src no longer has — a skill renamed
// between releases would otherwise linger under its old name, describing
// commands that may since have changed.
func Install(src fs.FS, dir string) error {
	names, err := Names(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}

	for _, name := range names {
		if err := installOne(src, dir, name); err != nil {
			return err
		}
	}

	installed, err := owned(dir)
	if err != nil {
		return err
	}
	for _, name := range installed {
		if !slices.Contains(names, name) {
			if err := os.RemoveAll(filepath.Join(dir, name)); err != nil {
				return fmt.Errorf("removing %s: %w", name, err)
			}
		}
	}

	return nil
}

// installOne stages a skill beside where it goes and then swaps it in, so an
// agent reading the directory meanwhile sees the old skill or the new one and
// not half of each. The swap itself is two renames, since a rename cannot
// replace a directory that has anything in it.
func installOne(src fs.FS, dir, name string) error {
	staged, err := os.MkdirTemp(dir, "."+name+"-")
	if err != nil {
		return fmt.Errorf("staging %s: %w", name, err)
	}
	defer os.RemoveAll(staged) // a no-op once it has been renamed into place

	sub, err := fs.Sub(src, name)
	if err != nil {
		return fmt.Errorf("reading %s: %w", name, err)
	}
	if err := os.CopyFS(staged, sub); err != nil {
		return fmt.Errorf("writing %s: %w", name, err)
	}
	if err := os.Chmod(staged, 0o755); err != nil {
		return fmt.Errorf("writing %s: %w", name, err)
	}

	target := filepath.Join(dir, name)
	old := staged + ".old"
	if err := os.Rename(target, old); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("replacing %s: %w", target, err)
	}
	if err := os.Rename(staged, target); err != nil {
		return fmt.Errorf("replacing %s: %w", target, err)
	}

	return os.RemoveAll(old)
}

// Remove deletes every one of solitary's skills from dir and reports their names.
// A directory that does not exist holds none.
func Remove(dir string) ([]string, error) {
	names, err := owned(dir)
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		if err := os.RemoveAll(filepath.Join(dir, name)); err != nil {
			return nil, fmt.Errorf("removing %s: %w", name, err)
		}
	}
	return names, nil
}

// owned lists the skills in dir that are solitary's: a prefixed directory, or a
// link to one, holding a SKILL.md.
func owned(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", dir, err)
	}
	var names []string
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), Prefix) {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, entry.Name(), "SKILL.md")); err == nil {
			names = append(names, entry.Name())
		}
	}
	return names, nil
}

// State is how an installed skill compares to the one in this binary.
type State string

const (
	Current  State = "current"
	Outdated State = "outdated" // installed by another solitary, or edited
	Missing  State = "missing"
)

// Skill is one of solitary's skills as found in a directory.
type Skill struct {
	Name  string
	State State
}

// Status compares what dir holds with src, file by file. A file present in the
// directory and not in src is left out of the comparison: it is not solitary's.
func Status(src fs.FS, dir string) ([]Skill, error) {
	names, err := Names(src)
	if err != nil {
		return nil, err
	}

	var skills []Skill
	for _, name := range names {
		state, err := compare(src, dir, name)
		if err != nil {
			return nil, err
		}
		skills = append(skills, Skill{Name: name, State: state})
	}
	return skills, nil
}

func compare(src fs.FS, dir, name string) (State, error) {
	if _, err := os.Stat(filepath.Join(dir, name)); errors.Is(err, fs.ErrNotExist) {
		return Missing, nil
	}

	state := Current
	err := fs.WalkDir(src, name, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		want, err := fs.ReadFile(src, path)
		if err != nil {
			return err
		}
		have, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(path)))
		if err != nil || !bytes.Equal(have, want) {
			state = Outdated
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("comparing %s: %w", name, err)
	}
	return state, nil
}

// record is the directories skills were installed into, kept so that ls, rm and
// a refresh after update reach a directory named with --into as well as the
// ones that would be detected again.
type record struct {
	Dirs []string `json:"dirs"`
}

func recordFile() (string, error) {
	dir, err := config.StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "skills.json"), nil
}

// Recorded returns the directories skills have been installed into.
func Recorded() ([]string, error) {
	path, err := recordFile()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var saved record
	if err := json.Unmarshal(data, &saved); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return saved.Dirs, nil
}

// Record replaces the list of directories skills are installed into.
func Record(dirs []string) error {
	path, err := recordFile()
	if err != nil {
		return err
	}
	if len(dirs) == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("removing %s: %w", path, err)
		}
		return nil
	}
	data, err := json.Marshal(record{Dirs: dirs})
	if err != nil {
		return fmt.Errorf("encoding %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}
