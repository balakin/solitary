// Package podman drives the container inside a cell's machine.
//
// Every command is run through limactl, so podman itself never has to exist on
// the host. The container is rootless: its root maps to an unprivileged user in
// the machine, which is what keeps the firewall and the machine's own
// configuration out of reach of whatever runs inside.
package podman

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"golang.org/x/term"

	"github.com/dm-balakin/solitary/internal/lima"
)

// Container is the name of the single container each cell runs.
const Container = "solitary"

// imageLabel records the image as written in cell.yaml, so that a change can be
// detected without having to guess how podman normalises references.
const imageLabel = "solitary.image"

// envLabel records a digest of the environment the container was started with.
// A container's environment is fixed once it is running, so this is what tells
// up that secrets have changed and the container has to be replaced. It is a
// digest rather than the values themselves, which anyone able to inspect the
// container would otherwise be able to read back.
const envLabel = "solitary.env"

// EnvDigest summarises a set of KEY=VALUE entries. Order does not matter.
func EnvDigest(env []string) string {
	sorted := append([]string(nil), env...)
	sort.Strings(sorted)

	sum := sha256.Sum256([]byte(strings.Join(sorted, "\x00")))

	return hex.EncodeToString(sum[:])
}

// HomeDir is where the container's home lives inside the container. It is a
// path no base image is likely to use, so a bind mount over it hides nothing
// the image shipped.
const HomeDir = "/home/cell"

// State describes the container inside a cell.
type State struct {
	// Exists is false when no container has been created yet.
	Exists bool
	// Running is true when the container is up.
	Running bool
	// Image is the identity the container was created from: see
	// RunOptions.Identity.
	Image string
	// EnvDigest identifies the environment the container was started with.
	EnvDigest string
}

// Inspect reports the state of a cell's container.
func Inspect(instance string) (State, error) {
	out, err := lima.Exec(instance,
		"podman", "container", "inspect", Container,
		"--format", "{{.State.Status}}\t{{index .Config.Labels \""+imageLabel+"\"}}\t{{index .Config.Labels \""+envLabel+"\"}}",
	)
	if err != nil {
		// podman exits non-zero when the container does not exist, which is
		// a normal state here. Anything that is not the command reporting
		// failure — an unreachable machine, a missing limactl — is not.
		if isCommandFailure(err) {
			return State{}, nil
		}
		return State{}, err
	}

	fields := strings.SplitN(strings.TrimSpace(string(out)), "\t", 3)
	state := State{Exists: true, Running: fields[0] == "running"}
	if len(fields) > 1 {
		state.Image = fields[1]
	}
	if len(fields) > 2 {
		state.EnvDigest = fields[2]
	}

	return state, nil
}

// RunOptions describes the container to start.
type RunOptions struct {
	// Image is the reference to run.
	Image string
	// Identity distinguishes what the container was started from. For a
	// pulled image it is the reference; for a built one it covers the build
	// context, so an edited Containerfile counts as a different image even
	// though the tag is unchanged.
	Identity string
	// Command is the shell command the container runs. It has to keep
	// running: the container's lifetime is the lifetime of this process, and
	// shells are attached to it separately with Shell.
	Command string
	// Env is passed with -e, one KEY=VALUE per entry.
	Env []string
	// HostHome is the path inside the machine that is bind-mounted over the
	// container's home, so that work survives the container being replaced.
	HostHome string
}

// Run creates and starts a cell's container, replacing any existing one.
func Run(instance string, opts RunOptions) error {
	if _, err := lima.Exec(instance, "mkdir", "-p", opts.HostHome); err != nil {
		return fmt.Errorf("creating %s in the machine: %w", opts.HostHome, err)
	}

	args := []string{
		"podman", "run",
		"--detach",
		"--replace",
		"--name", Container,
		"--label", imageLabel + "=" + opts.Identity,
		"--label", envLabel + "=" + EnvDigest(opts.Env),
		// The machine is the boundary, so the container shares its network
		// rather than adding a second one to reason about.
		"--network", "host",
		"--volume", opts.HostHome + ":" + HomeDir,
		"--workdir", HomeDir,
		"--env", "HOME=" + HomeDir,
	}
	for _, kv := range opts.Env {
		args = append(args, "--env", kv)
	}
	// A fixed entrypoint keeps behaviour the same across images, whatever
	// ENTRYPOINT or CMD they happen to declare.
	args = append(args, "--entrypoint", "/bin/sh", opts.Image, "-c", opts.Command)

	if _, err := lima.Exec(instance, args...); err != nil {
		return fmt.Errorf("starting the container: %w", err)
	}

	return nil
}

// ImageExists reports whether an image is already present inside the machine.
func ImageExists(instance, image string) (bool, error) {
	if _, err := lima.Exec(instance, "podman", "image", "exists", image); err != nil {
		if isCommandFailure(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// isCommandFailure reports whether err is a command that ran and exited
// non-zero, as opposed to one that could not be run at all.
func isCommandFailure(err error) bool {
	var exit *exec.ExitError
	return errors.As(err, &exit)
}

// Pull fetches an image inside the machine, showing progress.
func Pull(instance, image string) error {
	return lima.Attach(instance, "podman", "pull", image)
}

// Shell opens an interactive shell in a cell's container.
func Shell(instance string) error {
	args := []string{"podman", "exec", "--interactive"}
	// podman refuses --tty when there is no terminal to allocate, which is
	// the case when solitary is driven from a script.
	if term.IsTerminal(int(os.Stdin.Fd())) {
		args = append(args, "--tty")
	}
	args = append(args,
		"--workdir", HomeDir,
		Container,
		// bash where the image has it, sh everywhere else.
		"/bin/sh", "-c", "if command -v bash >/dev/null 2>&1; then exec bash; else exec sh; fi",
	)

	return lima.Attach(instance, args...)
}
