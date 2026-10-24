package cell

import (
	"bytes"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/balakin/solitary/internal/agents"
	"github.com/balakin/solitary/internal/config"
	"github.com/balakin/solitary/internal/lima"
	"github.com/balakin/solitary/internal/podman"
)

// cellSkills holds the skills for an agent inside a cell, one directory each.
// They describe the artifact tool, the inbox and the firewall that the same up
// sets up, which is why they are compiled in beside the script that installs
// that tool rather than shipped any other way.
//
//go:embed skills
var cellSkills embed.FS

// skillsHelper does the writing in the machine: one call for the whole sync,
// walking the cell's home without following anything the cell put there.
//
//go:embed skills.py
var skillsHelper string

// skillsRecord is what up remembers about the skills it put in a cell's home.
// It lives on the host, since anything in the home can be edited by whatever
// runs in the cell, and both lists are relative to the home.
type skillsRecord struct {
	// Dirs are where the skills were installed, so that the next up can
	// bring them up to date or, once nothing names them, take them out.
	Dirs []string `json:"dirs"`

	// Created are the directories up had to make to install them. A
	// directory up made, and that holds nothing but solitary's skills, is
	// not an agent: without this, a cell that once had agents: all would
	// find every agent on the list forever after.
	Created []string `json:"created"`
}

// installSkills gives the agents in a cell solitary's skills for working there.
// It runs on every up, beside the artifact tool those skills describe, so the
// two always come from the same binary.
//
// It never fails up: the skills are a convenience for the agent, and a cell
// that cannot be given them still works. What went wrong is said instead.
func installSkills(name, instance string, c *config.Cell, progress io.Writer) {
	if err := syncSkills(name, instance, c, progress); err != nil {
		fmt.Fprintf(progress, "Warning: could not install solitary's skills in %q: %v\n", name, err)
	}
}

func syncSkills(name, instance string, c *config.Cell, progress io.Writer) error {
	record, err := readSkillsRecord(name)
	if err != nil {
		return err
	}
	// Nothing to set up and nothing left from before: no machine calls.
	if c.Agents.Set && !c.Agents.All && len(c.Agents.List) == 0 && len(record.Dirs) == 0 {
		return nil
	}

	home, err := machineHome(instance)
	if err != nil {
		return err
	}

	var targets []agents.Target
	if !c.Agents.Set || c.Agents.All || len(c.Agents.List) > 0 {
		place, err := containerPlace(instance, home, record)
		if err != nil {
			return err
		}
		targets, err = skillTargets(c.Agents, place)
		if err != nil {
			return err
		}
	}

	// The home is all solitary writes to: it is the part of a cell that
	// lasts, and the only part reachable from the machine without going
	// through the container. Anything outside it is the image's.
	var dirs []string
	for _, target := range targets {
		rel, ok := inHome(target.Dir)
		if !ok {
			fmt.Fprintf(progress, "Warning: skipping skills for %s: %s is outside the cell's home, the only place solitary writes\n", strings.Join(target.Agents, ", "), target.Dir)
			continue
		}
		dirs = append(dirs, rel)
	}

	src, err := fs.Sub(cellSkills, "skills")
	if err != nil {
		return err
	}
	skills, err := skillFiles(src)
	if err != nil {
		return err
	}
	var result struct {
		Written []string `json:"written"`
		Skipped []string `json:"skipped"`
		Created []string `json:"created"`
	}
	if err := runSkillsHelper(instance, home, "sync", map[string]any{
		"skills":   skills,
		"targets":  nonNil(dirs),
		"previous": nonNil(record.Dirs),
		"created":  nonNil(record.Created),
	}, &result); err != nil {
		return err
	}
	for _, dir := range result.Skipped {
		fmt.Fprintf(progress, "Warning: skipping skills in ~/%s: a symlink or a file is in the way\n", dir)
	}

	reportSkills(progress, record.Dirs, result.Written)

	return writeSkillsRecord(name, skillsRecord{Dirs: result.Written, Created: result.Created})
}

// skillTargets turns a cell's agents: into the directories to install into.
func skillTargets(want config.Agents, place agents.Place) ([]agents.Target, error) {
	switch {
	case !want.Set:
		return agents.Detect(place), nil
	case want.All:
		var targets []agents.Target
		for _, id := range agents.IDs() {
			named, err := agents.Named(place, []string{id})
			if err != nil {
				return nil, err
			}
			targets = mergeTargets(targets, named...)
		}
		return targets, nil
	}

	var targets []agents.Target
	for _, entry := range want.List {
		if entry.Skills != "" {
			targets = mergeTargets(targets, agents.Target{Agents: []string{entry.Name}, Dir: path.Join(podman.HomeDir, entry.Skills)})
			continue
		}
		named, err := agents.Named(place, []string{entry.Name})
		if err != nil {
			return nil, err
		}
		targets = mergeTargets(targets, named...)
	}
	return targets, nil
}

func mergeTargets(targets []agents.Target, more ...agents.Target) []agents.Target {
	for _, target := range more {
		i := slices.IndexFunc(targets, func(t agents.Target) bool { return t.Dir == target.Dir })
		if i < 0 {
			targets = append(targets, target)
			continue
		}
		for _, agent := range target.Agents {
			if !slices.Contains(targets[i].Agents, agent) {
				targets[i].Agents = append(targets[i].Agents, agent)
			}
		}
	}
	return targets
}

// varName is what a variable from the agent list has to look like to be put in
// a shell command; they are solitary's own constants, and this keeps them so.
var varName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// containerPlace is the container as somewhere to look for agents: its own
// environment, since an image can move an agent with CLAUDE_CONFIG_DIR and the
// like, and its own filesystem. Both are asked once, in a batch, rather than
// once per agent.
//
// Only the variables the agent list is built on are read. The container's
// environment is also where its secrets are, and nothing else in it is any of
// solitary's business.
func containerPlace(instance, home string, record skillsRecord) (agents.Place, error) {
	names := agents.Vars()
	var script strings.Builder
	script.WriteString("printf '%s\\0'")
	for _, name := range names {
		if !varName.MatchString(name) {
			return agents.Place{}, fmt.Errorf("invalid variable name %q in the agent list", name)
		}
		fmt.Fprintf(&script, " \"${%s-}\"", name)
	}
	out, err := lima.Exec(instance, "podman", "exec", podman.Container, "sh", "-c", script.String())
	if err != nil {
		return agents.Place{}, fmt.Errorf("reading the container's environment: %w", err)
	}
	values := strings.Split(string(out), "\x00")
	env := map[string]string{}
	for i, name := range names {
		if i < len(values) {
			env[name] = values[i]
		}
	}

	expand := func(p string) string {
		return agents.Expand(p, podman.HomeDir, func(name string) string { return env[name] })
	}

	var markers []string
	for _, agent := range agents.All() {
		for _, marker := range agent.Markers {
			if p := expand(marker); p != "" && !slices.Contains(markers, p) {
				markers = append(markers, p)
			}
		}
	}
	args := append([]string{
		"podman", "exec", podman.Container, "sh", "-c",
		`for p do [ -e "$p" ] && printf '%s\0' "$p"; done; true`, "sh",
	}, markers...)
	out, err = lima.Exec(instance, args...)
	if err != nil {
		return agents.Place{}, fmt.Errorf("looking for agents in the container: %w", err)
	}
	present := map[string]bool{}
	for _, p := range strings.Split(string(out), "\x00") {
		if p != "" {
			present[p] = true
		}
	}

	// A marker up made itself, holding nothing but solitary's skills, is
	// solitary's footprint and not an agent.
	var candidates []string
	for p := range present {
		if rel, ok := inHome(p); ok && slices.Contains(record.Created, rel) {
			candidates = append(candidates, rel)
		}
	}
	footprint := map[string]bool{}
	if len(candidates) > 0 {
		var ours []string
		if err := runSkillsHelper(instance, home, "footprint", map[string]any{
			"markers": candidates,
			"created": record.Created,
		}, &ours); err != nil {
			return agents.Place{}, err
		}
		for _, rel := range ours {
			footprint[path.Join(podman.HomeDir, rel)] = true
		}
	}

	return agents.Place{
		Expand: expand,
		Exists: func(p string) bool { return present[p] && !footprint[p] },
	}, nil
}

// inHome turns a path in the container into one relative to its home, and says
// whether it is in the home at all.
func inHome(p string) (string, bool) {
	rel, ok := strings.CutPrefix(path.Clean(p), podman.HomeDir+"/")
	if !ok || rel == "" || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", false
	}
	return rel, true
}

type skillFile struct {
	Data string `json:"data"`
	Mode string `json:"mode"`
}

// skillFiles reads every skill in src into what the helper writes: each file's
// contents and mode, keyed by skill and then by path within it.
func skillFiles(src fs.FS) (map[string]map[string]skillFile, error) {
	skills := map[string]map[string]skillFile{}
	err := fs.WalkDir(src, ".", func(p string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		skill, rel, ok := strings.Cut(p, "/")
		if !ok {
			return nil
		}
		data, err := fs.ReadFile(src, p)
		if err != nil {
			return err
		}
		mode := "0644"
		if bytes.HasPrefix(data, []byte("#!")) {
			mode = "0755"
		}
		if skills[skill] == nil {
			skills[skill] = map[string]skillFile{}
		}
		skills[skill][rel] = skillFile{Data: base64.StdEncoding.EncodeToString(data), Mode: mode}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reading embedded skills: %w", err)
	}
	return skills, nil
}

func runSkillsHelper(instance, home, action string, request, response any) error {
	input, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("encoding the skills request: %w", err)
	}
	var out bytes.Buffer
	if err := lima.ExecStream(instance, bytes.NewReader(input), &out, "python3", "-c", skillsHelper, action, home); err != nil {
		return fmt.Errorf("installing skills in the machine: %w", err)
	}
	if err := json.Unmarshal(out.Bytes(), response); err != nil {
		return fmt.Errorf("reading what the machine installed: %w", err)
	}
	return nil
}

// reportSkills says what changed, and nothing when nothing did: every up runs
// this, and a line repeated at each start is a line nobody reads.
func reportSkills(progress io.Writer, before, after []string) {
	var added, removed []string
	for _, dir := range after {
		if !slices.Contains(before, dir) {
			added = append(added, "~/"+dir)
		}
	}
	for _, dir := range before {
		if !slices.Contains(after, dir) {
			removed = append(removed, "~/"+dir)
		}
	}
	if len(added) > 0 {
		fmt.Fprintf(progress, "Installed solitary's skills in %s\n", summarise(added))
	}
	if len(removed) > 0 {
		fmt.Fprintf(progress, "Removed solitary's skills from %s\n", summarise(removed))
	}
}

func summarise(dirs []string) string {
	if len(dirs) > 4 {
		return fmt.Sprintf("%d directories", len(dirs))
	}
	return strings.Join(dirs, ", ")
}

func nonNil(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}

func readSkillsRecord(name string) (skillsRecord, error) {
	file, err := config.SkillsFile(name)
	if err != nil {
		return skillsRecord{}, err
	}
	data, err := os.ReadFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		return skillsRecord{}, nil
	}
	if err != nil {
		return skillsRecord{}, fmt.Errorf("reading %s: %w", file, err)
	}
	return decodeSkillsRecord(data), nil
}

// decodeSkillsRecord returns an empty record for one it cannot make sense of:
// starting over at worst installs for an agent that is not there.
func decodeSkillsRecord(data []byte) skillsRecord {
	var record skillsRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return skillsRecord{}
	}
	return record
}

func writeSkillsRecord(name string, record skillsRecord) error {
	file, err := config.SkillsFile(name)
	if err != nil {
		return err
	}
	if len(record.Dirs) == 0 && len(record.Created) == 0 {
		return removeSkillsRecord(name)
	}
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encoding %s: %w", file, err)
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(file), err)
	}
	if err := os.WriteFile(file, data, 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", file, err)
	}
	return nil
}

// removeSkillsRecord forgets a cell's skills, for when its home is gone.
func removeSkillsRecord(name string) error {
	file, err := config.SkillsFile(name)
	if err != nil {
		return err
	}
	if err := os.Remove(file); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("removing %s: %w", file, err)
	}
	return nil
}
