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
	"regexp"
	"sort"
	"strings"

	"golang.org/x/term"

	"github.com/balakin/solitary/internal/lima"
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

// userLabel records the user cell.yaml declared work happens as, so that
// changing it replaces the container the way changing the image does.
const userLabel = "solitary.user"

// deviceLabel records the devices cell.yaml passed in, for the same reason: a
// device list is fixed once a container is created, so a changed one is a
// different container.
const deviceLabel = "solitary.devices"

// DeviceList is how a set of devices is written down, both for the label and
// for comparing what is running against what is asked for. Order is kept: it
// is the order the definition wrote them in, and podman is given them that way.
func DeviceList(devices []string) string {
	return strings.Join(devices, " ")
}

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
	// User is the cell's user, as cell.yaml declared it. Empty means the
	// container runs everything as root, which is the ordinary case.
	User string
	// Devices is the device list the container was created with, as
	// DeviceList writes it. Empty means the container reaches no device of
	// the machine's own.
	Devices string
}

// Inspect reports the state of a cell's container.
func Inspect(instance string) (State, error) {
	out, err := lima.Exec(instance,
		"podman", "container", "inspect", Container,
		"--format", "{{.State.Status}}\t{{index .Config.Labels \""+imageLabel+"\"}}\t{{index .Config.Labels \""+envLabel+"\"}}\t{{index .Config.Labels \""+userLabel+"\"}}\t{{index .Config.Labels \""+deviceLabel+"\"}}",
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

	fields := strings.SplitN(strings.TrimSpace(string(out)), "\t", 5)
	state := State{Exists: true, Running: fields[0] == "running"}
	if len(fields) > 1 {
		state.Image = fields[1]
	}
	if len(fields) > 2 {
		state.EnvDigest = fields[2]
	}
	if len(fields) > 3 {
		state.User = fields[3]
	}
	if len(fields) > 4 {
		state.Devices = fields[4]
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
	// User is the user work happens as inside the cell, as cell.yaml
	// declared it. Empty means the image's own, which is root for almost
	// every image.
	User string
	// Devices are device nodes in the machine to pass into the container.
	// They have to exist there and be openable by the machine's user, which
	// is what the caller arranges before starting the container.
	Devices []string
}

// Run creates and starts a cell's container, replacing any existing one.
func Run(instance string, opts RunOptions) error {
	// The mode is set rather than left to the machine's umask, which is 002
	// on Ubuntu: a group-writable home is one an sshd refuses to read a key
	// out of, and this is the home of every cell that has an sshd.
	if _, err := lima.Exec(instance, "sh", "-c",
		"mkdir -p "+opts.HostHome+" && chmod 0755 "+opts.HostHome,
	); err != nil {
		return fmt.Errorf("creating %s in the machine: %w", opts.HostHome, err)
	}

	mapping, err := homeMapping(instance, opts)
	if err != nil {
		return err
	}

	if _, err := lima.Exec(instance, runArgs(opts, mapping)...); err != nil {
		return fmt.Errorf("starting the container: %w", err)
	}

	return nil
}

// runArgs is the podman invocation that creates a cell's container.
func runArgs(opts RunOptions, mapping []string) []string {
	args := []string{
		"podman", "run",
		"--detach",
		"--replace",
		"--name", Container,
		"--label", imageLabel + "=" + opts.Identity,
		"--label", envLabel + "=" + EnvDigest(opts.Env),
		"--label", userLabel + "=" + opts.User,
		"--label", deviceLabel + "=" + DeviceList(opts.Devices),
		// The machine is the boundary, so the container shares its network
		// rather than adding a second one to reason about.
		"--network", "host",
		"--volume", opts.HostHome + ":" + HomeDir,
		"--workdir", HomeDir,
		"--env", "HOME=" + HomeDir,
	}
	args = append(args, mapping...)
	for _, device := range opts.Devices {
		args = append(args, "--device", device)
	}
	for _, kv := range opts.Env {
		args = append(args, "--env", kv)
	}

	// A fixed entrypoint keeps behaviour the same across images, whatever
	// ENTRYPOINT or CMD they happen to declare.
	return append(args, "--entrypoint", "/bin/sh", opts.Image, "-c", opts.Command)
}

// homeMapping gives a cell's user the home that is mounted into it.
//
// The home is a directory in the machine, so it belongs to the machine's user,
// and a rootless container maps that user onto its own root. A cell that runs
// everything as root — every cell that does not serve a login of its own —
// therefore finds its home writable and needs nothing here.
//
// A cell with a user of its own does need something: the home would be root's,
// and everything that user did in it would fail on a permission. So that user
// is mapped onto the machine's user instead, which is what owns the directory
// on the other side of the mount. A mapping rather than a chown, because the
// machine keeps writing into that home from outside the container — fetch,
// send and the artifact tool all do — and a home chowned away from the machine
// user would take that with it.
func homeMapping(instance string, opts RunOptions) ([]string, error) {
	image, err := imageUser(instance, opts.Image)
	if err != nil {
		return nil, err
	}

	user := opts.User
	if user == "" {
		user = image
	}
	if isRoot(user) {
		return nil, nil
	}

	uid, gid, err := lookupUser(instance, opts.Image, user)
	if err != nil {
		return nil, err
	}

	// keep-id runs the container as the mapped user as well, and the command
	// of a cell that has a user is the one thing there that still wants root:
	// it is an sshd, or an editor server, or whatever else hands that user
	// their session. So the process stays whoever the image said it was.
	process := image
	if process == "" {
		process = "0"
	}

	return []string{
		"--userns", fmt.Sprintf("keep-id:uid=%d,gid=%d", uid, gid),
		"--user", process,
	}, nil
}

// imageUser reports the user an image declares, empty when it declares none.
func imageUser(instance, image string) (string, error) {
	out, err := lima.Exec(instance, "podman", "image", "inspect", image, "--format", "{{.Config.User}}")
	if err != nil {
		return "", fmt.Errorf("reading the user of %s: %w", image, err)
	}

	return strings.TrimSpace(string(out)), nil
}

// isRoot reports whether a user specification means root, which is what an
// image that names no user runs as.
func isRoot(user string) bool {
	name, _, _ := strings.Cut(user, ":")
	switch name {
	case "", "0", "root":
		return true
	}

	return false
}

// plainName is what a user or group has to look like to be handed to a shell
// in the image.
var plainName = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]*$`)

// lookupUser resolves a user specification against the image's own passwd
// file, since a name is only a name inside the image that defines it.
func lookupUser(instance, image, user string) (uid, gid int, err error) {
	name, group, _ := strings.Cut(user, ":")
	if group == "" {
		group = name
	}

	// The name reaches a shell in the image, so it has to be a name and
	// nothing else. A cell's own is checked when its definition is read; this
	// is the one an image declared.
	if !plainName.MatchString(name) || !plainName.MatchString(group) {
		return 0, 0, fmt.Errorf("%s declares the user %q, which is not a name or an id", image, user)
	}

	// The image is asked rather than parsed: id knows about every source of
	// users the image has, and a passwd file is not the only one.
	out, err := lima.Exec(instance, "podman", "run", "--rm", "--entrypoint", "/bin/sh", image,
		"-c", "id -u "+name+" && id -g "+group)
	if err != nil {
		return 0, 0, fmt.Errorf("the image runs no user %q, which the cell asks work to happen as: %w", user, err)
	}

	if _, err := fmt.Sscan(string(out), &uid, &gid); err != nil {
		return 0, 0, fmt.Errorf("reading the id of %q in %s: %w", user, image, err)
	}

	return uid, gid, nil
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

// shellCommand is the command a shell session runs: bash where the image has
// it, sh everywhere else.
var shellCommand = []string{
	"/bin/sh", "-c", "if command -v bash >/dev/null 2>&1; then exec bash; else exec sh; fi",
}

// Shell opens an interactive shell in a cell's container, as the cell's user.
func Shell(instance, user string) error {
	return Exec(instance, user, shellCommand)
}

// ShellCommand prepares the same shell session as Shell, without running it,
// for a caller that runs the process itself. It always asks for a terminal:
// there is no reason to open a shell that has none.
func ShellCommand(instance, user string) (*exec.Cmd, error) {
	return lima.Command(instance, execArgs(true, user, shellCommand)...)
}

// Exec runs one command in a cell's container and returns when it is done. The
// command's streams are the caller's, so it can be piped into and out of like
// any other command.
func Exec(instance, user string, command []string) error {
	// podman refuses --tty when there is no terminal to allocate, which is
	// the case when solitary is driven from a script.
	return lima.Attach(instance, execArgs(term.IsTerminal(int(os.Stdin.Fd())), user, command)...)
}

// execArgs builds the podman command line for one session. A session driven by
// a person carries the terminal it is being watched from; one driven by a
// script carries nothing and runs the command as given.
func execArgs(tty bool, user string, command []string) []string {
	args := []string{"podman", "exec", "--interactive"}
	// A session belongs to whoever works in the cell, so that what it leaves
	// in the home belongs to them too. Empty is the container's own user,
	// which is the image's, which is root for almost every image.
	if user != "" {
		args = append(args, "--user", user)
	}
	if !tty {
		args = append(args, "--workdir", HomeDir, Container)

		return append(args, command...)
	}

	args = append(args, "--tty")
	args = append(args, terminalEnv()...)
	args = append(args, "--workdir", HomeDir, Container)
	// The bootstrap runs before the command and replaces itself with it, so
	// the command still receives exactly the arguments it was given.
	args = append(args, "/bin/sh", "-c", terminfoBootstrap, "solitary")

	return append(args, command...)
}

// terminfoEnv is the variable the description of the host's terminal is passed
// in, for terminfoBootstrap to install.
const terminfoEnv = "SOLITARY_TERMINFO"

// terminfoBootstrap teaches a cell the terminal it is being watched from, then
// runs the command it was given.
//
// Passing TERM alone is not enough: a name the cell has no description for is
// worse than none, and programs refuse to start ("missing or unsuitable
// terminal"). Terminals that ship their own description — ghostty, kitty,
// wezterm — are exactly the ones no distribution knows.
//
// The compiled description lands in the cell's home, which is a directory on
// the machine rather than in the container, so it is written once and then
// survives new containers. If it cannot be installed the session falls back to
// a terminal every cell knows, which loses capabilities but always works.
const terminfoBootstrap = `
if [ -n "${SOLITARY_TERMINFO:-}" ]; then
	marker="$HOME/.terminfo/.solitary/$TERM"
	if [ ! -f "$marker" ]; then
		if printf '%s' "$SOLITARY_TERMINFO" | tic -x -o "$HOME/.terminfo" - 2>/dev/null; then
			mkdir -p "${marker%/*}" && : > "$marker"
		else
			TERM=xterm-256color
			export TERM
		fi
	fi
	unset SOLITARY_TERMINFO
fi
exec "$@"
`

// terminalEnv passes the host terminal into the container: its name, whether it
// does true colour, and its description. Without them a cell sees no TERM at
// all and everything inside falls back to a dumb terminal, so colours are
// approximated to the nearest basic one and full-screen programs misdraw.
//
// This describes the terminal a person is sitting at, not the cell, which is
// why it belongs to a session rather than to the container's own environment.
func terminalEnv() []string {
	var env []string
	for _, name := range []string{"TERM", "COLORTERM"} {
		if v := os.Getenv(name); v != "" {
			env = append(env, "--env", name+"="+v)
		}
	}
	if description := terminfo(os.Getenv("TERM")); description != "" {
		env = append(env, "--env", terminfoEnv+"="+description)
	}

	return env
}

// terminfo returns the host's description of a terminal, in the source form tic
// compiles. An empty string means the host cannot describe it, which is not a
// problem worth reporting: the session falls back to a terminal every cell
// knows.
func terminfo(name string) string {
	if name == "" {
		return ""
	}

	out, err := exec.Command("infocmp", "-x", "-q", name).Output()
	if err != nil {
		return ""
	}

	return string(out)
}
