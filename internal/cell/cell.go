// Package cell orchestrates the pieces of a cell: its definition on disk, the
// Lima machine it runs in and the container inside that machine.
package cell

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/dm-balakin/solitary/internal/config"
	"github.com/dm-balakin/solitary/internal/lima"
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

		if c, err := config.LoadCell(name); err == nil {
			info.Image = c.Image
		} else {
			info.Image = "(unreadable)"
		}

		if inst, ok := byName[config.Instance(name)]; ok {
			info.Status = statusOf(inst)
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

	instance := config.Instance(name)
	inst, err := lima.Lookup(instance)
	if err != nil {
		return err
	}

	switch {
	case inst == nil:
		fmt.Fprintf(progress, "Creating cell %q (this takes a few minutes the first time)...\n", name)
		if err := config.WriteRendered(name, rendered); err != nil {
			return err
		}
		path, err := config.RenderedFile(name)
		if err != nil {
			return err
		}
		if err := lima.Create(instance, path); err != nil {
			return err
		}

	case inst.Status == lima.StatusBroken:
		return fmt.Errorf("cell %q is broken; inspect it with 'limactl shell %s' or discard it with 'solitary rm %s'", name, instance, name)

	case inst.Status == lima.StatusRunning:
		warnDrift(name, rendered, progress)

	default:
		warnDrift(name, rendered, progress)
		fmt.Fprintf(progress, "Starting cell %q...\n", name)
		if err := lima.Start(instance); err != nil {
			return err
		}
	}

	return nil
}

// warnDrift reports a vm block that has changed since the machine was created.
// Lima cannot apply those changes to an existing machine, and silently ignoring
// them would leave the cell running settings the file no longer describes.
func warnDrift(name, rendered string, w io.Writer) {
	applied, err := config.ReadRendered(name)
	if err != nil || applied == "" || applied == rendered {
		return
	}
	fmt.Fprintf(w, "Warning: the vm settings for %q changed since its machine was created.\n", name)
	fmt.Fprintf(w, "         The running cell still uses the old ones. To apply the change:\n")
	fmt.Fprintf(w, "           solitary rm %s && solitary up %s\n", name, name)
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
	if path, err := config.RenderedFile(name); err == nil {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing %s: %w", path, err)
		}
	}

	fmt.Fprintf(progress, "The definition and secrets for %q are kept; 'solitary up %s' rebuilds it.\n", name, name)

	return nil
}
