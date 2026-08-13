// Package cell orchestrates the pieces of a cell: its definition on disk, the
// Lima machine it runs in and the container inside that machine.
package cell

import (
	"github.com/dm-balakin/solitary/internal/config"
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

	infos := make([]Info, 0, len(names))
	for _, name := range names {
		info := Info{Name: name, Status: StatusUninitialized}

		if c, err := config.LoadCell(name); err == nil {
			info.Image = c.Image
		} else {
			info.Image = "(unreadable)"
		}

		infos = append(infos, info)
	}

	return infos, nil
}
