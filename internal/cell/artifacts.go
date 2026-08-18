package cell

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/balakin/solitary/internal/config"
	"github.com/balakin/solitary/internal/lima"
	"github.com/balakin/solitary/internal/podman"
)

// artifactScript is the tool a cell publishes with. It is embedded rather than
// left to the image, so that every cell has it whatever it was built from.
//
//go:embed artifact.sh
var artifactScript string

// Where the hand-off happens. These are paths under the cell's home, which is a
// directory in the machine bind-mounted over the container's home — so the
// container writes them and the machine has them, with nothing mounted from the
// host and no second mechanism to reason about.
const (
	outboxDir = "outbox"
	inboxDir  = "inbox"
	// toolPath is where the script lives in the cell's home. It is the
	// cell's own file and the cell may edit it: nothing here is a control.
	// What makes the channel safe is enforced on the host, in Fetch.
	toolDir  = ".solitary"
	toolName = "artifact"
	// toolLink puts it on PATH. It lives in the container rather than the
	// home, so it is remade whenever the container is replaced.
	toolLink = "/usr/local/bin/" + toolName
)

// Queue is a summary of one hand-off folder.
type Queue struct {
	Files int
	Bytes int64
}

// Handoff reports what is waiting to cross the cell boundary. The machine has
// to be running, but the container does not: these folders live on the VM disk.
type Handoff struct {
	Inbox  Queue
	Outbox Queue
}

// Artifact is one file a cell has published.
type Artifact struct {
	Name string
	Size int64
	// Problem is why this one cannot be handed over, when it cannot. The
	// name was chosen inside the cell, so it is shown and refused rather
	// than hidden: something that will not come out should say so.
	Problem string
}

// OK reports whether this artifact can be fetched.
func (a Artifact) OK() bool { return a.Problem == "" }

// installArtifacts creates the outbox and inbox and puts the publishing tool
// where a shell in the cell will find it.
//
// Run on every up rather than only when the container is replaced: the link
// lives in the container and the script's contents change with solitary, and
// neither is worth a stale cell.
func installArtifacts(instance string) error {
	home, err := machineHome(instance)
	if err != nil {
		return err
	}

	tool := filepath.Join(home, toolDir, toolName)
	if _, err := lima.Exec(instance, "mkdir", "-p",
		filepath.Join(home, outboxDir), filepath.Join(home, inboxDir), filepath.Join(home, toolDir),
	); err != nil {
		return fmt.Errorf("creating the outbox and inbox: %w", err)
	}

	// Written through the machine rather than into the container, because
	// the home is the same directory on both sides of that bind mount.
	if _, err := lima.Exec(instance, "sh", "-c",
		"cat > "+tool+" <<'SOLITARY_ARTIFACT'\n"+artifactScript+"SOLITARY_ARTIFACT\nchmod 0755 "+tool,
	); err != nil {
		return fmt.Errorf("installing %s in the cell: %w", toolName, err)
	}

	// The container sees the same file at its own home. A link, so that
	// editing the script is picked up without reinstalling anything.
	container := podman.HomeDir + "/" + toolDir + "/" + toolName
	if _, err := lima.Exec(instance, "podman", "exec", podman.Container,
		"ln", "-sf", container, toolLink,
	); err != nil {
		return fmt.Errorf("putting %s on the cell's PATH: %w", toolName, err)
	}

	return nil
}

// Artifacts lists what a cell has published.
//
// The listing is made here rather than read from anything the cell wrote: only
// regular files, only the top level, and symlinks are not among them — a name
// in the outbox is chosen by whatever runs in the cell, and is treated that way.
func Artifacts(name string) ([]Artifact, error) {
	instance, err := usable(name)
	if err != nil {
		return nil, err
	}
	home, err := machineHome(instance)
	if err != nil {
		return nil, err
	}

	out, err := lima.Exec(instance, "find", filepath.Join(home, outboxDir),
		"-maxdepth", "1", "-type", "f", "-printf", `%s\t%f\n`)
	if err != nil {
		return nil, fmt.Errorf("reading the outbox of %q: %w", name, err)
	}

	var artifacts []Artifact
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		size, file, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		n, err := strconv.ParseInt(size, 10, 64)
		if err != nil {
			continue
		}
		artifact := Artifact{Name: file, Size: n}
		if err := validName(file); err != nil {
			artifact.Problem = err.Error()
		}
		artifacts = append(artifacts, artifact)
	}

	return artifacts, nil
}

// HandoffStatus summarizes both queues for the dashboard.
func HandoffStatus(name string) (Handoff, error) {
	instance, err := usable(name)
	if err != nil {
		return Handoff{}, err
	}
	home, err := machineHome(instance)
	if err != nil {
		return Handoff{}, err
	}

	inbox, err := queue(filepath.Join(home, inboxDir), instance)
	if err != nil {
		return Handoff{}, fmt.Errorf("reading the inbox of %q: %w", name, err)
	}
	outbox, err := queue(filepath.Join(home, outboxDir), instance)
	if err != nil {
		return Handoff{}, fmt.Errorf("reading the outbox of %q: %w", name, err)
	}
	return Handoff{Inbox: inbox, Outbox: outbox}, nil
}

func queue(dir, instance string) (Queue, error) {
	out, err := lima.Exec(instance, "find", dir, "-maxdepth", "1", "-type", "f", "-printf", "%s\\n")
	if err != nil {
		return Queue{}, err
	}
	var q Queue
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\\n") {
		if line == "" {
			continue
		}
		size, err := strconv.ParseInt(line, 10, 64)
		if err != nil {
			continue
		}
		q.Files++
		q.Bytes += size
	}
	return q, nil
}

// Fetch copies what a cell published onto the host. With no names it takes
// everything; the cell keeps its copies either way, so fetching twice is not a
// mistake and an interrupted fetch loses nothing.
func Fetch(name string, names []string, into string, force bool, progress io.Writer) error {
	instance, err := usable(name)
	if err != nil {
		return err
	}
	home, err := machineHome(instance)
	if err != nil {
		return err
	}

	published, err := Artifacts(name)
	if err != nil {
		return err
	}
	wanted, refused, err := selection(published, names)
	if err != nil {
		return err
	}
	for _, artifact := range refused {
		fmt.Fprintf(progress, "Left behind: %s\n", artifact.Problem)
	}
	if len(wanted) == 0 {
		if len(refused) == 0 {
			fmt.Fprintf(progress, "Cell %q has published nothing.\n", name)
		}
		return nil
	}

	if into == "" {
		into = "."
	}
	if err := os.MkdirAll(into, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", into, err)
	}
	// Checked as a group and before anything is written: a half-done fetch
	// that stopped at the first collision is worse than one that did not
	// start, and the names came from the cell.
	if !force {
		var existing []string
		for _, artifact := range wanted {
			if _, err := os.Lstat(filepath.Join(into, artifact.Name)); err == nil {
				existing = append(existing, artifact.Name)
			}
		}
		if len(existing) > 0 {
			return fmt.Errorf("%s already exists in %s; fetch with --force to replace it",
				strings.Join(existing, ", "), into)
		}
	}

	for _, artifact := range wanted {
		target := filepath.Join(into, artifact.Name)
		if err := lima.CopyOut(instance, filepath.Join(home, outboxDir, artifact.Name), target); err != nil {
			return fmt.Errorf("fetching %s: %w", artifact.Name, err)
		}
		// What a cell produces is data on this host, never a program.
		if err := os.Chmod(target, 0o644); err != nil {
			return fmt.Errorf("fetching %s: %w", artifact.Name, err)
		}
		fmt.Fprintf(progress, "%s  (%s)\n", target, Size(artifact.Size))
	}

	return nil
}

// selection resolves what was asked for against what is there, refusing a name
// that is not published rather than quietly fetching less than was asked for.
//
// Whatever cannot be handed over is left out here, so that one file a cell
// named badly does not stop the rest coming out.
func selection(published []Artifact, names []string) ([]Artifact, []Artifact, error) {
	byName := make(map[string]Artifact, len(published))
	for _, artifact := range published {
		byName[artifact.Name] = artifact
	}

	asked := published
	if len(names) > 0 {
		asked = make([]Artifact, 0, len(names))
		for _, want := range names {
			artifact, ok := byName[want]
			if !ok {
				return nil, nil, fmt.Errorf("%q is not published; fetch with --list to see what is", want)
			}
			asked = append(asked, artifact)
		}
	}

	var wanted, refused []Artifact
	for _, artifact := range asked {
		if artifact.OK() {
			wanted = append(wanted, artifact)
		} else {
			refused = append(refused, artifact)
		}
	}

	return wanted, refused, nil
}

// Send copies files from the host into a cell's inbox.
func Send(name string, paths []string, progress io.Writer) error {
	instance, err := usable(name)
	if err != nil {
		return err
	}
	home, err := machineHome(instance)
	if err != nil {
		return err
	}

	for _, path := range paths {
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file; send an archive of it instead", path)
		}
		if err := validName(filepath.Base(path)); err != nil {
			return err
		}
	}

	if _, err := lima.Exec(instance, "mkdir", "-p", filepath.Join(home, inboxDir)); err != nil {
		return fmt.Errorf("creating the inbox: %w", err)
	}
	for _, path := range paths {
		file := filepath.Base(path)
		if err := lima.CopyFile(path, instance, filepath.Join(home, inboxDir, file)); err != nil {
			return fmt.Errorf("sending %s: %w", file, err)
		}
		fmt.Fprintf(progress, "sent %s\n", file)
	}
	fmt.Fprintf(progress, "Waiting in the cell at %s/%s\n", podman.HomeDir, inboxDir)

	return nil
}

// validName rejects anything that is not a plain file name.
//
// This is the rule the whole channel rests on. A name in the outbox is written
// by whatever runs in the cell, and is then joined onto a path on this host; a
// name that is not a name — ".." , "a/b", an empty string — would be written
// somewhere other than where the fetch was aimed.
func validName(name string) error {
	switch {
	case name == "":
		return fmt.Errorf("a file with no name cannot be handed over")
	case name == "." || name == "..":
		return fmt.Errorf("%q is not a file name", name)
	case strings.ContainsAny(name, `/\`):
		return fmt.Errorf("%q is a path rather than a file name", name)
	case strings.HasPrefix(name, "-"):
		return fmt.Errorf("%q starts with a dash, which a command would read as a flag", name)
	case strings.ContainsAny(name, "\n\r\t\x00"):
		return fmt.Errorf("%q contains a character that cannot be in a file name", name)
	}

	return nil
}

// usable returns the machine behind a cell, which has to be running. The
// container does not: the outbox is a directory in the machine, so what a cell
// published is still there to collect after whatever produced it has died.
func usable(name string) (string, error) {
	instance := config.Instance(name)
	inst, err := lima.Lookup(instance)
	if err != nil {
		return "", err
	}
	switch {
	case inst == nil:
		return "", fmt.Errorf("cell %q has no machine; start it with 'solitary up %s'", name, name)
	case inst.Status != lima.StatusRunning:
		return "", fmt.Errorf("cell %q is %s; start it with 'solitary up %s'", name, inst.Status, name)
	}

	return instance, nil
}

// Size renders a number of bytes the way a listing should read. Shared with the
// tunnel's counters, which are the other thing here measured in bytes.
func Size(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}

	value, exp := float64(n)/unit, 0
	for value >= unit && exp < 3 {
		value, exp = value/unit, exp+1
	}

	return fmt.Sprintf("%.1f %ciB", value, "KMGT"[exp])
}
