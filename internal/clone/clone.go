package clone

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dm-balakin/solitary/internal/config"
)

// cellFile is the one file that makes a directory a cell.
const cellFile = "cell.yaml"

// Staged is a cell definition that has been copied out of a repository and
// checked, but has not become a cell yet.
//
// Nothing is installed until Install is called, so a caller can show what the
// definition asks for — an image, a set of secrets, an allow list — and let
// someone decide before any of it lands.
type Staged struct {
	// Dir holds the copied definition. It is a temporary directory, and
	// Discard removes it.
	Dir string

	// Cell is what the definition parses to, with the vpn file it may name
	// deliberately not read: that one is the runner's, not the author's.
	Cell *config.Cell

	// Name is what the cell would be called if the caller does not choose.
	Name string

	// Source is where this came from, for saying so.
	Source Source

	// Refused names files that were left behind and why, one line each.
	// They are reported rather than hidden: something in the repository is
	// not going to arrive, and silence about it is worse than the fact.
	Refused []string
}

// Discard removes the staged copy. It is safe to call after Install.
func (s *Staged) Discard() {
	if s == nil || s.Dir == "" {
		return
	}
	_ = os.RemoveAll(s.Dir)
}

// Catalogue is a repository that holds several cells rather than being one.
// It is returned instead of a staged cell when a repository has no cell.yaml of
// its own and the caller did not say which cell they wanted.
type Catalogue struct {
	Source Source
	Cells  []string
}

func (c *Catalogue) Error() string {
	if len(c.Cells) == 0 {
		return fmt.Sprintf("%s holds no cells: no cell.yaml at its root, and none in the directories inside it", c.Source.Display)
	}

	return fmt.Sprintf("%s holds %d cells: name the one you want, e.g. 'solitary clone %s#%s'",
		c.Source.Display, len(c.Cells), c.Source.Display, c.Cells[0])
}

// Stage clones the repository, finds the cell inside it, copies it somewhere
// safe and checks that it is a cell.
//
// A repository holding several cells with none named is not a failure of this
// function so much as a question for the caller, so it comes back as a
// *Catalogue error, which a caller can show as a list.
func Stage(source Source, progress io.Writer) (*Staged, error) {
	repo, err := fetch(source, progress)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(repo)

	dir, err := locate(repo, source)
	if err != nil {
		return nil, err
	}

	staged, err := os.MkdirTemp("", "solitary-clone-")
	if err != nil {
		return nil, fmt.Errorf("creating a staging directory: %w", err)
	}
	if err := os.Chmod(staged, 0o700); err != nil {
		return nil, fmt.Errorf("creating a staging directory: %w", err)
	}

	result := &Staged{Dir: staged, Source: source, Name: source.DefaultName()}
	result.Refused, err = copyTree(dir, staged)
	if err != nil {
		result.Discard()
		return nil, err
	}

	data, err := os.ReadFile(filepath.Join(staged, cellFile))
	if err != nil {
		result.Discard()
		return nil, fmt.Errorf("reading the cloned definition: %w", err)
	}
	result.Cell, err = config.CheckCell(data, staged)
	if err != nil {
		result.Discard()
		return nil, err
	}

	return result, nil
}

// fetch puts the repository on disk. git is shelled out to, as limactl and
// podman are: it already knows this machine's credentials, its SSH agent and
// whatever rewriting rules are configured, and none of that is worth
// reimplementing.
func fetch(source Source, progress io.Writer) (string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", errors.New("git is needed to clone a cell definition, and is not on PATH")
	}

	dir, err := os.MkdirTemp("", "solitary-repo-")
	if err != nil {
		return "", fmt.Errorf("creating a temporary directory: %w", err)
	}

	fmt.Fprintf(progress, "Cloning %s\n", source.Display)

	// A definition is wanted, not its history — but git ignores a depth on
	// a local clone and warns about having been asked, so only ask when it
	// saves something.
	args := []string{"clone", "--quiet"}
	if !source.Local {
		args = append(args, "--depth", "1")
	}
	args = append(args, "--", source.URL, dir)

	cmd := exec.Command("git", args...)
	cmd.Stdout, cmd.Stderr = progress, progress
	// git must never stop for a password on a URL that was typed rather
	// than chosen: it would hang with no terminal to answer on.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	if err := cmd.Run(); err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("cloning %s: %w", source.Display, err)
	}

	return dir, nil
}

// locate finds the cell inside a cloned repository.
//
// A repository is either one cell — a cell.yaml at its root, the shape a
// dotfiles repository has — or a catalogue of them.
func locate(repo string, source Source) (string, error) {
	if source.Path != "" {
		dir := filepath.Join(repo, filepath.FromSlash(source.Path))
		if isCell(dir) {
			return dir, nil
		}

		cells, err := catalogue(repo)
		if err != nil {
			return "", err
		}
		if len(cells) == 0 {
			return "", fmt.Errorf("%s has no cell at %s, and none anywhere else in it", source.Display, source.Path)
		}

		return "", fmt.Errorf("%s has no cell at %s\nit holds: %s", source.Display, source.Path, strings.Join(cells, ", "))
	}

	if isCell(repo) {
		return repo, nil
	}

	cells, err := catalogue(repo)
	if err != nil {
		return "", err
	}

	return "", &Catalogue{Source: source, Cells: cells}
}

// catalogue lists the directories one level down that are cells. Only one
// level: a repository of cells is a flat thing, and searching deeper would
// start finding cells inside build contexts.
func catalogue(repo string) ([]string, error) {
	entries, err := os.ReadDir(repo)
	if err != nil {
		return nil, fmt.Errorf("reading the cloned repository: %w", err)
	}

	var cells []string
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if isCell(filepath.Join(repo, entry.Name())) {
			cells = append(cells, entry.Name())
		}
	}
	sort.Strings(cells)

	return cells, nil
}

func isCell(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, cellFile))

	return err == nil && info.Mode().IsRegular()
}

// withheld reports whether a file is one that carries a credential, and so is
// never taken out of someone else's repository.
//
// A .env holds the values a cell's secrets are read from and a vpn.conf holds a
// private key. Both are meant to stay on the host that runs the cell — a
// definition is published without them by design, and one that arrives anyway
// is either a mistake in the repository or an attempt to seed this machine with
// someone else's credential. Neither is worth copying.
func withheld(name string) bool {
	lower := strings.ToLower(name)

	return lower == ".env" || strings.HasSuffix(lower, ".env") || lower == "vpn.conf"
}

// copyTree copies a cell directory into the staging area under rules of its
// own, since the names in it were written by someone else.
//
// Regular files and directories are copied. A symlink is refused wherever it
// appears: it is the one entry whose target need not be inside the repository.
// Anything holding a credential is refused by name, and .git is left behind.
// What is refused comes back rather than being dropped quietly.
func copyTree(src, dst string) ([]string, error) {
	var refused []string

	err := filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		switch {
		case entry.Name() == ".git":
			return fs.SkipDir
		case entry.Type()&fs.ModeSymlink != 0:
			refused = append(refused, fmt.Sprintf("%s: a symlink, which points somewhere this cannot vouch for", rel))
			return nil
		case !entry.IsDir() && !entry.Type().IsRegular():
			refused = append(refused, fmt.Sprintf("%s: neither a file nor a directory", rel))
			return nil
		case !entry.IsDir() && withheld(entry.Name()):
			refused = append(refused, fmt.Sprintf("%s: holds credentials, which are yours rather than the cell's", rel))
			return nil
		}

		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o700)
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		return copyFile(path, target, mode(info.Mode()))
	})
	if err != nil {
		return nil, fmt.Errorf("copying the definition: %w", err)
	}

	return refused, nil
}

// mode is what a copied file is written with.
//
// The executable bit is kept, and only that: nothing here runs on this machine
// — a build context is copied into the machine and built there — so a script a
// Containerfile runs would only be broken by stripping it. Everything else is
// closed up the way a cell directory is written by init.
func mode(m fs.FileMode) fs.FileMode {
	if m&0o100 != 0 {
		return 0o700
	}

	return 0o600
}

func copyFile(src, dst string, perm fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}

	// O_CREATE leaves an existing file's mode alone, and --force writes
	// over one. Say what it should be either way.
	return os.Chmod(dst, perm)
}

// Install makes a staged definition into a cell called name.
//
// An existing cell is refused unless force is set, and force replaces only what
// the repository provides: a .env, a vpn.conf and anything else already in the
// directory are left where they are. That is what makes cloning the same source
// again the way to update a shared cell — the definition moves, and the
// credentials that were never in it stay.
func Install(staged *Staged, name string, force bool) ([]string, error) {
	if err := config.ValidateName(name); err != nil {
		return nil, err
	}

	dir, err := config.CellDir(name)
	if err != nil {
		return nil, err
	}

	if !force {
		if err := Vacant(name); err != nil {
			return nil, err
		}
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("creating %s: %w", dir, err)
	}

	var written []string
	err = filepath.WalkDir(staged.Dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(staged.Dir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		target := filepath.Join(dir, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o700)
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		if err := copyFile(path, target, mode(info.Mode())); err != nil {
			return err
		}
		written = append(written, filepath.ToSlash(rel))

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("installing into %s: %w", dir, err)
	}

	return written, nil
}

// Vacant reports that no cell of this name is defined yet, and says what to do
// about it when one is.
//
// Install calls it, and so does the caller before cloning anything: a name that
// is taken is known from the source alone, and there is no reason to fetch a
// repository, let alone ask a question, only to refuse afterwards.
func Vacant(name string) error {
	path, err := config.CellFile(name)
	if err != nil {
		return err
	}

	switch _, err := os.Stat(path); {
	case errors.Is(err, fs.ErrNotExist):
		return nil
	case err != nil:
		return fmt.Errorf("checking %s: %w", path, err)
	}

	dir, err := config.CellDir(name)
	if err != nil {
		return err
	}

	return fmt.Errorf("cell %q already exists at %s\n"+
		"clone it under another name with --as, or replace its definition with --force,\n"+
		"which keeps the .env beside it and everything else you have added", name, dir)
}
