package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// nameRE matches cell names. Lima instance names are used verbatim in
// hostnames, so the same restrictions apply here.
var nameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,58}[a-z0-9]$|^[a-z0-9]$`)

// ValidateName reports whether name can be used as a cell name.
func ValidateName(name string) error {
	if !nameRE.MatchString(name) {
		return fmt.Errorf("invalid cell name %q: use lowercase letters, digits and dashes, starting and ending with a letter or digit", name)
	}
	return nil
}

// Root is ~/.config/solitary, honouring XDG_CONFIG_HOME.
func Root() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locating config directory: %w", err)
	}
	return filepath.Join(dir, "solitary"), nil
}

// CellsDir is the directory holding one subdirectory per cell.
func CellsDir() (string, error) {
	root, err := Root()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "cells"), nil
}

// CellDir is the directory holding a single cell's definition and secrets.
func CellDir(name string) (string, error) {
	cells, err := CellsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cells, name), nil
}

// CellFile is a cell's definition.
func CellFile(name string) (string, error) {
	dir, err := CellDir(name)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "cell.yaml"), nil
}

// EnvFile holds a cell's secret values. It stays on the host and is never
// copied into the VM.
func EnvFile(name string) (string, error) {
	dir, err := CellDir(name)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ".env"), nil
}

// RenderedFile is the Lima definition last applied for a cell, kept so that
// changes to the vm block can be detected after the machine already exists.
func RenderedFile(name string) (string, error) {
	dir, err := CellDir(name)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "lima.yaml"), nil
}

// UserConfigFile is the user-wide defaults file.
func UserConfigFile() (string, error) {
	root, err := Root()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "config.yaml"), nil
}

// Instance is the name of the Lima machine backing a cell. Cells are prefixed
// so they are distinguishable from machines Lima manages for other tools.
func Instance(name string) string {
	return "solitary-" + name
}
