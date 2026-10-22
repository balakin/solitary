package cell

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/balakin/solitary/internal/config"
	"github.com/balakin/solitary/internal/host"
	"github.com/balakin/solitary/internal/lima"
	"github.com/balakin/solitary/internal/podman"
)

// A home on its way to a new machine is in one of three files, and which one
// says how far the move got:
//
//   - partial: being written, or left by a save that never finished. It is
//     worth nothing and always thrown away.
//   - saved: read back in full and found complete, with the old machine
//     possibly still there.
//   - pending (the archive's own name): the old machine is gone, so this is
//     the only copy of the home. up restores it before anything else is
//     allowed to start, and only removes it once it is in the new machine.
func homeArchives(name string) (partial, saved, pending string, err error) {
	pending, err = config.HomeArchive(name)
	if err != nil {
		return "", "", "", err
	}

	return pending + ".partial", pending + ".saved", pending, nil
}

// NewMachine replaces a cell's machine with a new one, carrying its home
// across: the home is saved to the host, the old machine destroyed, a new one
// created from the base solitary uses now, and the home put back before the
// container starts. It is how a machine gets a new release, or sheds what a
// provision script and years of patches left on its disk, without losing the
// work in it.
//
// Nothing is destroyed until the saved home has been read back in full on the
// host, and nothing removes it until the new machine has it. A move that stops
// halfway — a failed create, an interrupted restore — is finished by running
// it again, or by any up, which restores a waiting home before it starts the
// container.
func NewMachine(name string, progress io.Writer) error {
	c, err := config.LoadCell(name)
	if err != nil {
		return err
	}
	partial, saved, pending, err := homeArchives(name)
	if err != nil {
		return err
	}
	instance := config.Instance(name)
	inst, err := lima.Lookup(instance)
	if err != nil {
		return err
	}

	switch {
	case exists(pending):
		fmt.Fprintf(progress, "The home saved from %q's old machine is still waiting for its new one; finishing that.\n", name)
		return Up(name, false, progress)

	case exists(saved) && inst == nil:
		// Saved and verified, and the machine destroyed, but stopped before
		// the archive could say so.
		if err := os.Rename(saved, pending); err != nil {
			return fmt.Errorf("keeping the saved home: %w", err)
		}
		fmt.Fprintf(progress, "The home saved from %q's old machine is still waiting for its new one; finishing that.\n", name)
		return Up(name, false, progress)

	case inst == nil:
		return fmt.Errorf("cell %q has no machine to replace; 'solitary up %s' creates one", name, name)

	case inst.Status == lima.StatusBroken:
		return fmt.Errorf("cell %q is broken, so its home cannot be read out of it; 'limactl shell %s' is the way in", name, instance)
	}

	// Whatever is here is older than the machine it would be replaced from.
	for _, stale := range []string{partial, saved} {
		if err := os.Remove(stale); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("removing %s: %w", stale, err)
		}
	}

	if err := saveHome(name, instance, c, inst.Status == lima.StatusRunning, partial, saved, progress); err != nil {
		return err
	}

	fmt.Fprintf(progress, "Destroying the old machine behind %q...\n", name)
	if err := lima.Delete(instance); err != nil {
		return err
	}
	if err := config.RemoveApplied(name); err != nil {
		return err
	}
	if err := os.Rename(saved, pending); err != nil {
		return fmt.Errorf("keeping the saved home: %w", err)
	}

	return Up(name, false, progress)
}

// saveHome reads a cell's home out of its machine into an archive on the host,
// and reads the archive back to make sure all of it arrived. The archive is
// never unpacked on the host: what a cell wrote is the cell's, and a file
// named to land somewhere it should not stays inside a compressed stream.
func saveHome(name, instance string, c *config.Cell, running bool, partial, saved string, progress io.Writer) (err error) {
	if !running {
		// The machine as it is, with the definition it already has: it is
		// about to be destroyed, and what the cell describes now — a smaller
		// disk, say — may be exactly what this one cannot start with.
		fmt.Fprintf(progress, "Cell %q is stopped; starting its machine alone to save its home.\n", name)
		if err := lima.Start(instance, chooseGPU(name, c, io.Discard)); err != nil {
			return err
		}
		defer func() {
			if err != nil {
				fmt.Fprintf(progress, "Stopping cell %q again; it is as it was.\n", name)
				_ = lima.Stop(instance)
			}
		}()
	}

	home, err := machineHome(instance)
	if err != nil {
		return err
	}

	// Nothing writes to the home while it is read, so what arrives is one
	// moment of it rather than a mix of several.
	if _, err := lima.Exec(instance, "podman", "rm", "--force", "--ignore", podman.Container); err != nil {
		return fmt.Errorf("stopping the container: %w", err)
	}

	out, err := guestRun(instance, nil, "sudo", "du", "-sb", home)
	if err != nil {
		return fmt.Errorf("measuring the home: %w", err)
	}
	size, _, _ := strings.Cut(out, "\t")
	bytesUsed, err := strconv.ParseUint(strings.TrimSpace(size), 10, 64)
	if err != nil {
		return fmt.Errorf("measuring the home: unexpected du output %q", out)
	}

	dir := filepath.Dir(partial)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	// Compressed it is smaller, by how much nobody can say beforehand; its
	// size uncompressed is the most it can need.
	free, err := host.DiskFree(dir)
	if err != nil {
		return err
	}
	if bytesUsed >= free {
		return fmt.Errorf("the home of %q holds %s and %s has %s free; nothing was changed",
			name, host.FormatSize(bytesUsed), dir, host.FormatSize(free))
	}

	// Sockets are left out by tar, so they are left out of the count too.
	out, err = guestRun(instance, nil, "sudo", "sh", "-c", "cd "+shellQuote(home)+" && find . ! -type s | wc -l")
	if err != nil {
		return fmt.Errorf("counting the home's files: %w", err)
	}
	want, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return fmt.Errorf("counting the home's files: unexpected output %q", out)
	}

	fmt.Fprintf(progress, "Saving the home of %q (%s, %d entries) to the host...\n", name, host.FormatSize(bytesUsed), want)
	if err := streamOut(instance, home, partial); err != nil {
		return err
	}

	got, err := countEntries(partial)
	if err != nil {
		return fmt.Errorf("reading back the saved home: %w; the old machine is untouched", err)
	}
	if got != want {
		return fmt.Errorf("the saved home holds %d entries and the machine's %d; the old machine is untouched", got, want)
	}
	if err := os.Rename(partial, saved); err != nil {
		return fmt.Errorf("keeping the saved home: %w", err)
	}

	return nil
}

// streamOut writes the home as a compressed tar to path. Run as root with
// numeric owners, so that a file a cell's other users own keeps its owner: the
// new machine maps the same ids the old one did.
func streamOut(instance, home, path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("creating %s: %w", path, err)
	}

	cmd, err := lima.Command(instance, "sudo", "tar", "--numeric-owner", "--xattrs", "-C", home, "-czf", "-", ".")
	if err != nil {
		f.Close()
		return err
	}
	var stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = f, &stderr
	runErr := cmd.Run()
	syncErr := f.Sync()
	closeErr := f.Close()
	if runErr != nil {
		return fmt.Errorf("saving the home: %w\n%s", runErr, strings.TrimSpace(stderr.String()))
	}
	if err := errors.Join(syncErr, closeErr); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}

// countEntries reads a compressed tar to its end and counts what is in it,
// which is what makes it the archive of a whole home and not of the part of
// one that arrived before something went wrong: a cut-off stream fails here.
func countEntries(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	z, err := gzip.NewReader(f)
	if err != nil {
		return 0, err
	}
	r := tar.NewReader(z)
	n := 0
	for {
		_, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return 0, err
		}
		n++
	}
	// The gzip trailer carries a checksum of everything before it, and it is
	// only read once the stream is drained past the tar's own end.
	if _, err := io.Copy(io.Discard, z); err != nil {
		return 0, err
	}

	return n, nil
}

// restoreHome gives a new machine the home saved from the old one, before any
// container is started over it, and only then lets the archive go.
func restoreHome(name, instance, pending string, progress io.Writer) error {
	home, err := machineHome(instance)
	if err != nil {
		return err
	}

	fmt.Fprintf(progress, "Restoring the home of %q into its new machine...\n", name)
	if _, err := lima.Exec(instance, "podman", "rm", "--force", "--ignore", podman.Container); err != nil {
		return fmt.Errorf("stopping the container: %w", err)
	}
	// Emptied first, so that a restore run again after an interrupted one
	// leaves the home as it was saved and not as two attempts left it.
	if _, err := guestRun(instance, nil, "sudo", "sh", "-c",
		"rm -rf "+shellQuote(home)+" && mkdir -p "+shellQuote(home)); err != nil {
		return fmt.Errorf("preparing the home: %w", err)
	}

	f, err := os.Open(pending)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := guestRun(instance, f, "sudo", "tar", "--numeric-owner", "--xattrs", "-xpzf", "-", "-C", home); err != nil {
		return fmt.Errorf("restoring the home: %w; it is kept at %s, and up tries again", err, pending)
	}

	if err := os.Remove(pending); err != nil {
		return fmt.Errorf("removing %s: %w", pending, err)
	}
	fmt.Fprintf(progress, "The home of %q is in its new machine.\n", name)

	return nil
}

// guestRun runs a command in a machine for as long as it takes, which is what
// sets it apart from lima.Exec: walking or copying a home has no size it can
// be expected to finish within.
func guestRun(instance string, input io.Reader, args ...string) (string, error) {
	cmd, err := lima.Command(instance, args...)
	if err != nil {
		return "", err
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdin, cmd.Stdout, cmd.Stderr = input, &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("%w\n%s", err, msg)
		}
		return "", err
	}

	return stdout.String(), nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
