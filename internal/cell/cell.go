// Package cell orchestrates the pieces of a cell: its definition on disk, the
// Lima machine it runs in and the container inside that machine.
package cell

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dm-balakin/solitary/internal/config"
	"github.com/dm-balakin/solitary/internal/host"
	"github.com/dm-balakin/solitary/internal/lima"
	"github.com/dm-balakin/solitary/internal/podman"
	"github.com/dm-balakin/solitary/internal/secrets"
)

// Status is what a cell is currently doing.
type Status string

const (
	// StatusUninitialized means the cell is defined but no machine was ever
	// created for it.
	StatusUninitialized Status = "uninitialized"
	// StatusStopped means the machine exists but is not running.
	StatusStopped Status = "stopped"
	// StatusRunning means the machine is up and the container inside it is
	// running.
	StatusRunning Status = "running"
	// StatusDegraded means the machine is up but the container is not.
	StatusDegraded Status = "degraded"
	// StatusUnreachable means Lima considers the machine running but nothing
	// inside it answers.
	StatusUnreachable Status = "unreachable"
	// StatusBroken means Lima reports the machine as broken.
	StatusBroken Status = "broken"
)

// ErrNotRunning is returned by operations that need a running cell.
var ErrNotRunning = errors.New("cell is not running")

// Info summarises a cell for listing.
type Info struct {
	Name   string
	Image  string
	Status Status
}

// List returns every defined cell with its current state.
func List() ([]Info, error) {
	names, err := config.ListCells()
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, nil
	}

	instances, err := lima.List()
	if err != nil {
		return nil, err
	}
	byName := make(map[string]lima.Instance, len(instances))
	for _, inst := range instances {
		byName[inst.Name] = inst
	}

	infos := make([]Info, 0, len(names))
	for _, name := range names {
		info := Info{Name: name, Status: StatusUninitialized}

		switch c, err := config.LoadCell(name); {
		case err != nil:
			info.Image = "(unreadable)"
		case c.Build != "":
			info.Image = "build:" + c.Build
		default:
			info.Image = c.Image
		}

		if inst, ok := byName[config.Instance(name)]; ok {
			info.Status = statusOf(inst)
			// A machine can be up while the container inside it is not,
			// which is not the same thing as the cell being usable — and it
			// can be up while the guest itself has stopped answering.
			if info.Status == StatusRunning {
				switch state, err := podman.Inspect(inst.Name); {
				case errors.Is(err, lima.ErrUnreachable):
					info.Status = StatusUnreachable
				case err == nil && !state.Running:
					info.Status = StatusDegraded
				}
			}
		}

		infos = append(infos, info)
	}

	return infos, nil
}

// statusOf maps a machine's state onto a cell's state.
func statusOf(inst lima.Instance) Status {
	switch inst.Status {
	case lima.StatusRunning:
		return StatusRunning
	case lima.StatusStopped:
		return StatusStopped
	case lima.StatusBroken:
		return StatusBroken
	default:
		return Status(inst.Status)
	}
}

// Up brings a cell's machine up, creating it first if it does not exist.
// It is safe to call repeatedly: a running cell is left alone.
func Up(name string, progress io.Writer) error {
	c, err := config.LoadCell(name)
	if err != nil {
		return err
	}

	rendered, err := lima.Render(c.VM, c.Ports)
	if err != nil {
		return err
	}

	if err := config.MigrateApplied(name); err != nil {
		return err
	}

	instance := config.Instance(name)
	inst, err := lima.Lookup(instance)
	if err != nil {
		return err
	}

	if err := verifyMemory(c.VM.Memory, progress); err != nil {
		return err
	}

	switch {
	case inst == nil:
		fmt.Fprintf(progress, "Creating cell %q (this takes a few minutes the first time)...\n", name)
		if err := createMachine(instance, rendered); err != nil {
			return err
		}
		if err := config.WriteApplied(name, rendered); err != nil {
			return err
		}

	case inst.Status == lima.StatusBroken:
		return fmt.Errorf("cell %q is broken; inspect it with 'limactl shell %s' or discard it with 'solitary rm %s'", name, instance, name)

	case inst.Status == lima.StatusRunning && !lima.Reachable(instance):
		// The machine's process is alive and Lima still calls it running, but
		// nothing inside answers. Stopping it releases the memory and lets the
		// next up start it cleanly.
		return fmt.Errorf("cell %q is running but not responding.\n"+
			"Stop it with 'solitary down %s' and start it again; if it keeps happening,\n"+
			"check that vm.memory fits this host and look at ~/.lima/%s/ha.stderr.log",
			name, name, instance)

	case inst.Status == lima.StatusRunning:
		warnDrift(name, rendered, progress)

	default:
		warnDrift(name, rendered, progress)
		fmt.Fprintf(progress, "Starting cell %q...\n", name)
		if err := lima.Start(instance); err != nil {
			return err
		}
	}

	env, err := resolveSecrets(name, c, progress)
	if err != nil {
		return err
	}

	return ensureContainer(name, instance, c, env, progress)
}

// resolveSecrets collects the values this cell is allowed to see, asking for
// any that are declared but not set yet.
func resolveSecrets(name string, c *config.Cell, progress io.Writer) ([]string, error) {
	if len(c.Secrets) == 0 {
		return nil, nil
	}

	path, err := config.EnvFile(name)
	if err != nil {
		return nil, err
	}
	values, err := secrets.Load(path)
	if err != nil {
		return nil, err
	}

	if missing := secrets.Missing(c.Secrets, values); len(missing) > 0 {
		if !secrets.CanPrompt() {
			return nil, fmt.Errorf("cell %q needs values for %s; set them with 'solitary secrets %s'",
				name, strings.Join(missing, ", "), name)
		}

		fmt.Fprintf(progress, "Cell %q needs %d secret(s). Input is hidden.\n", name, len(missing))
		changed, err := secrets.Prompt(progress, missing, values)
		if err != nil {
			return nil, err
		}
		if changed {
			if err := secrets.Save(path, values); err != nil {
				return nil, err
			}
			fmt.Fprintf(progress, "Saved to %s\n", path)
		}
	}

	return secrets.Env(c.Secrets, values), nil
}

// ensureContainer starts the cell's container if it is not already running the
// requested image. Work lives in a directory on the machine that is mounted
// over the container's home, so replacing the container keeps files, caches and
// anything an editor installed into the home directory.
func ensureContainer(name, instance string, c *config.Cell, env []string, progress io.Writer) error {
	ref, identity, err := ensureImage(name, instance, c, progress)
	if err != nil {
		return err
	}

	state, err := podman.Inspect(instance)
	if err != nil {
		return err
	}

	digest := podman.EnvDigest(env)
	if state.Running && state.Image == identity && state.EnvDigest == digest {
		return nil
	}

	switch {
	case state.Running && state.Image != identity:
		fmt.Fprintln(progress, "Image changed; replacing the container.")
	case state.Running && state.EnvDigest != digest:
		// A container's environment is fixed once it is running, so secrets
		// that changed only reach the cell by replacing it.
		fmt.Fprintln(progress, "Secrets changed; restarting the container.")
	}

	home, err := machineHome(instance)
	if err != nil {
		return err
	}

	return podman.Run(instance, podman.RunOptions{
		Image:    ref,
		Identity: identity,
		Command:  c.Command,
		Env:      env,
		HostHome: home,
	})
}

// ensureImage makes the cell's image available inside the machine, building it
// when the cell declares a Containerfile and pulling it otherwise.
//
// It returns the reference to run and an identity for it. The identity is what
// a running container is compared against: for a built image it covers the
// build context, so editing a Containerfile is noticed even though the tag
// never changes.
func ensureImage(name, instance string, c *config.Cell, progress io.Writer) (ref, identity string, err error) {
	if c.Build == "" {
		exists, err := podman.ImageExists(instance, c.Image)
		if err != nil {
			return "", "", err
		}
		if !exists {
			fmt.Fprintf(progress, "Pulling %s...\n", c.Image)
			if err := podman.Pull(instance, c.Image); err != nil {
				return "", "", err
			}
		}
		return c.Image, c.Image, nil
	}

	digest, err := podman.ContextDigest(c.BuildPath)
	if err != nil {
		return "", "", err
	}
	tag := config.Tag(name)
	identity = "build:" + digest

	built, err := podman.BuiltDigest(instance, tag)
	if err != nil {
		return "", "", err
	}
	if built != digest {
		fmt.Fprintf(progress, "Building %s from %s...\n", tag, c.Build)
		if err := podman.Build(instance, c.BuildPath, tag, digest); err != nil {
			return "", "", err
		}
	}

	return tag, identity, nil
}

// machineHome is the directory inside the machine that backs a cell's home.
func machineHome(instance string) (string, error) {
	out, err := lima.Exec(instance, "sh", "-c", "echo $HOME")
	if err != nil {
		return "", fmt.Errorf("locating the home directory in the machine: %w", err)
	}

	home := strings.TrimSpace(string(out))
	if home == "" {
		return "", errors.New("the machine reported an empty home directory")
	}

	return home + "/cell", nil
}

// Shell opens a shell inside a running cell. It never changes state: a cell
// that is stopped or absent is an error rather than something to start.
func Shell(name string) error {
	if _, err := config.LoadCell(name); err != nil {
		return err
	}

	instance := config.Instance(name)
	inst, err := lima.Lookup(instance)
	if err != nil {
		return err
	}
	switch {
	case inst == nil:
		return fmt.Errorf("cell %q does not exist yet: run 'solitary up %s'", name, name)
	case inst.Status != lima.StatusRunning:
		return fmt.Errorf("%w: run 'solitary up %s'", ErrNotRunning, name)
	}

	state, err := podman.Inspect(instance)
	if err != nil {
		return err
	}
	if !state.Running {
		return fmt.Errorf("the container in %q is not running: run 'solitary up %s'", name, name)
	}

	return podman.Shell(instance)
}

// createMachine builds a machine from a definition rendered for this call. The
// definition is a temporary file: it is derived from cell.yaml and the defaults
// compiled in, so keeping a copy would only invite someone to edit the copy.
func createMachine(instance, rendered string) error {
	dir, err := os.MkdirTemp("", "solitary-")
	if err != nil {
		return fmt.Errorf("creating a temporary directory: %w", err)
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "lima.yaml")
	if err := os.WriteFile(path, []byte(rendered), 0o600); err != nil {
		return fmt.Errorf("writing the machine definition: %w", err)
	}

	return lima.Create(instance, path)
}

// verifyMemory refuses a machine the host cannot back, and warns about one it
// can only just back. A machine larger than its backing store starts, reports
// itself running and then dies with nothing written anywhere the user looks.
func verifyMemory(memory string, progress io.Writer) error {
	backing, err := host.MemoryBacking()
	if err != nil {
		// Not being able to measure the host is not a reason to refuse to
		// work, but it is worth saying that the check did not happen.
		fmt.Fprintf(progress, "Warning: could not check this host's memory: %v\n", err)
		backing = host.Backing{}
	}

	warning, err := host.VerifyMemory(memory, backing)
	if err != nil {
		return err
	}
	if warning != "" {
		fmt.Fprintf(progress, "Warning: %s\n", warning)
	}

	return nil
}

// warnDrift reports a vm block that has changed since the machine was created.
// Lima cannot apply those changes to an existing machine, and silently ignoring
// them would leave the cell running settings the file no longer describes.
func warnDrift(name, rendered string, w io.Writer) {
	applied, err := config.ReadApplied(name)
	if err != nil || applied == "" || applied == config.Digest(rendered) {
		return
	}
	fmt.Fprintf(w, "Warning: the vm settings for %q changed since its machine was created.\n", name)
	fmt.Fprintf(w, "         The running cell still uses the old ones. To apply the change:\n")
	fmt.Fprintf(w, "           solitary rm %s && solitary up %s\n", name, name)
}

// SetSecrets asks for every secret a cell declares, keeping values that are
// already set unless something new is typed. It reports whether the cell needs
// restarting for the change to take effect.
func SetSecrets(name string, progress io.Writer) error {
	c, err := config.LoadCell(name)
	if err != nil {
		return err
	}
	if len(c.Secrets) == 0 {
		path, err := config.CellFile(name)
		if err != nil {
			return err
		}
		fmt.Fprintf(progress, "Cell %q declares no secrets. Add them under secrets: in %s.\n", name, path)
		return nil
	}

	path, err := config.EnvFile(name)
	if err != nil {
		return err
	}
	values, err := secrets.Load(path)
	if err != nil {
		return err
	}

	if !secrets.CanPrompt() {
		return errors.New("solitary secrets needs a terminal to ask on")
	}

	changed, err := secrets.Prompt(progress, c.Secrets, values)
	if err != nil {
		return err
	}
	if !changed {
		fmt.Fprintln(progress, "Nothing changed.")
		return nil
	}

	if err := secrets.Save(path, values); err != nil {
		return err
	}
	fmt.Fprintf(progress, "Saved to %s\n", path)

	// A running container was started with the old environment.
	inst, err := lima.Lookup(config.Instance(name))
	if err != nil {
		return err
	}
	if inst != nil && inst.Status == lima.StatusRunning {
		state, err := podman.Inspect(inst.Name)
		if err == nil && state.Running {
			fmt.Fprintf(progress, "Cell %q is running with the old values. Apply them with: solitary up %s\n", name, name)
		}
	}

	return nil
}

// Down stops a cell's machine, keeping its disk and its secrets.
func Down(name string, progress io.Writer) error {
	if _, err := config.LoadCell(name); err != nil {
		return err
	}

	instance := config.Instance(name)
	inst, err := lima.Lookup(instance)
	if err != nil {
		return err
	}
	if inst == nil {
		return fmt.Errorf("cell %q has no machine to stop", name)
	}
	if inst.Status != lima.StatusRunning {
		fmt.Fprintf(progress, "Cell %q is already stopped.\n", name)
		return nil
	}

	fmt.Fprintf(progress, "Stopping cell %q...\n", name)

	return lima.Stop(instance)
}

// Remove destroys a cell's machine. The definition and the secrets file stay on
// the host, so up rebuilds an equivalent cell.
func Remove(name string, progress io.Writer) error {
	if err := config.ValidateName(name); err != nil {
		return err
	}

	instance := config.Instance(name)
	inst, err := lima.Lookup(instance)
	if err != nil {
		return err
	}
	if inst == nil {
		return fmt.Errorf("cell %q has no machine to remove", name)
	}

	fmt.Fprintf(progress, "Destroying the machine behind %q...\n", name)
	if err := lima.Delete(instance); err != nil {
		return err
	}

	// Drop the record of what was applied, so a later up starts clean rather
	// than comparing against a machine that no longer exists.
	if err := config.RemoveApplied(name); err != nil {
		return err
	}

	fmt.Fprintf(progress, "The definition and secrets for %q are kept; 'solitary up %s' rebuilds it.\n", name, name)

	return nil
}
