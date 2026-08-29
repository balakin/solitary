// Package cell orchestrates the pieces of a cell: its definition on disk, the
// Lima machine it runs in and the container inside that machine.
package cell

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/balakin/solitary/internal/config"
	"github.com/balakin/solitary/internal/host"
	"github.com/balakin/solitary/internal/lima"
	"github.com/balakin/solitary/internal/podman"
	"github.com/balakin/solitary/internal/secrets"
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

	rendered, err := lima.Render(c.VM, c.Ports, c.Network)
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
		if err := verifyMemory(c.VM.Memory, progress); err != nil {
			return err
		}
		fmt.Fprintf(progress, "Creating cell %q (this takes a few minutes the first time)...\n", name)
		if err := createMachine(instance, rendered); err != nil {
			return err
		}
		if err := config.WriteApplied(name, config.NewApplied(rendered, c.VM.Provision)); err != nil {
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
		warnDrift(name, c, rendered, progress)

	default:
		// A stopped machine reads its definition again when it starts, so
		// this is the moment a change to the vm block can take effect
		// without destroying the disk that machine carries. The one such
		// change a machine cannot take is asked about before the
		// definition is replaced.
		if err := verifyDisk(name, instance, c.VM.Disk); err != nil {
			return err
		}
		if err := applyDrift(name, instance, c, rendered, progress); err != nil {
			return err
		}
		if err := verifyMemory(c.VM.Memory, progress); err != nil {
			return err
		}
		fmt.Fprintf(progress, "Starting cell %q...\n", name)
		if err := lima.Start(instance); err != nil {
			return err
		}
	}

	if err := installTunnel(instance, c.Network, progress); err != nil {
		return err
	}

	env, err := resolveSecrets(name, c, progress)
	if err != nil {
		return err
	}
	// The identity is configuration rather than a secret, so it is not
	// declared in secrets: and does not live in .env.
	env = append(env, c.Git.Env()...)
	// Which cell this is. A container has the machine's hostname, which says
	// nothing useful, so an image that wants to name the cell — in a prompt,
	// say — has nowhere else to read it from.
	env = append(env, "SOLITARY_CELL="+name)

	if err := ensureContainer(name, instance, c, env, progress); err != nil {
		return err
	}

	// After the container, since the tool has to be linked onto the PATH of
	// whatever is running now, and unconditionally, since a container that
	// did not need replacing may still predate this.
	return installArtifacts(instance)
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

	// An optional secret is asked about but never blocks: only the required
	// ones decide whether this cell can start at all.
	if unset := secrets.Missing(c.Secrets.Names(), values); len(unset) > 0 && secrets.CanPrompt() {
		fmt.Fprintf(progress, "Cell %q needs %d secret(s). Input is hidden.\n", name, len(unset))
		changed, err := secrets.Prompt(progress, fields(c.Secrets, unset), values)
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

	if blocking := secrets.Missing(c.Secrets.RequiredNames(), values); len(blocking) > 0 {
		return nil, fmt.Errorf("cell %q needs values for %s; set them with 'solitary secrets %s'",
			name, strings.Join(blocking, ", "), name)
	}

	return secrets.Env(c.Secrets.Names(), values), nil
}

// fields describes the secrets to ask about, keeping the cell's declaration
// order. A nil only limits the set to those names; passing every declared
// secret is what rotating them all looks like.
func fields(declared config.Secrets, only []string) []secrets.Field {
	wanted := make(map[string]bool, len(only))
	for _, name := range only {
		wanted[name] = true
	}

	list := make([]secrets.Field, 0, len(declared))
	for _, secret := range declared {
		if only != nil && !wanted[secret.Name] {
			continue
		}
		list = append(list, secrets.Field{
			Name:        secret.Name,
			Optional:    !secret.Required,
			Description: secret.Description,
		})
	}
	return list
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
	devices := podman.DeviceList(c.Devices)
	if state.Running && state.Image == identity && state.EnvDigest == digest &&
		state.User == c.User && state.Devices == devices && state.ShmSize == c.VM.Memory {
		return nil
	}

	switch {
	case state.Running && state.Image != identity:
		fmt.Fprintln(progress, "Image changed; replacing the container.")
	case state.Running && state.EnvDigest != digest:
		// A container's environment is fixed once it is running, so secrets
		// that changed only reach the cell by replacing it.
		fmt.Fprintln(progress, "Secrets changed; restarting the container.")
	case state.Running && state.User != c.User:
		// The home is mapped for the cell's user when the container starts,
		// so a different user is a different container.
		fmt.Fprintln(progress, "User changed; replacing the container.")
	case state.Running && state.Devices != devices:
		// Devices are handed to a container when it is created and cannot be
		// added to it afterwards.
		fmt.Fprintln(progress, "Devices changed; replacing the container.")
	case state.Running && state.ShmSize != c.VM.Memory:
		// The container's shared memory follows the machine's, and it is
		// sized when the container is created.
		fmt.Fprintln(progress, "Machine memory changed; replacing the container.")
	}

	if err := ensureDevices(instance, c.Devices); err != nil {
		return err
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
		User:     c.User,
		Devices:  c.Devices,
		ShmSize:  c.VM.Memory,
	})
}

// ensureDevices makes the devices a cell declares openable from inside it.
//
// Passing a node through is only half of it: the container is rootless, so its
// root is the machine's user on the other side, and a node the guest kernel
// left to root — /dev/kvm is root:kvm and group-readable — is a node the cell
// gets and cannot open. So each one is handed to that user.
//
// This runs on every up rather than once, because the nodes do not survive the
// machine being rebooted: udev makes them again at boot, owned as the
// distribution says. Ownership is the whole of it — the group is left alone,
// and nothing is made world-readable.
func ensureDevices(instance string, devices []string) error {
	for _, device := range devices {
		if _, err := lima.Exec(instance, "test", "-e", device); err != nil {
			return fmt.Errorf("the machine has no %s: a cell can only be given a device its guest kernel has", device)
		}
		// The path is checked against config.devicePath when the definition
		// is read, so it holds nothing a shell would take for more than a
		// path.
		if _, err := lima.Exec(instance, "sh", "-c",
			`sudo chown "$(id -un)" `+device,
		); err != nil {
			return fmt.Errorf("handing %s to the machine's user: %w", device, err)
		}
	}

	return nil
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

// tunnelStaging is where the configuration lands before it is installed. The
// machine is copied into as the unprivileged user Lima logs in as, which cannot
// write /etc, so it arrives here and is moved with sudo.
const tunnelStaging = "/tmp/solitary-tunnel.conf"

// installTunnel places a cell's WireGuard configuration in its machine and
// brings the tunnel up.
//
// This happens here rather than in the machine definition because the file
// holds a private key. A definition is kept for as long as the machine exists,
// is handed to the guest through cloud-init, and is the part of a cell meant to
// be read and copied by other people; a credential belongs in none of that.
//
// It is also why the file is compared before it is written: an up on a running
// cell is a routine thing, and re-installing the same configuration would drop
// every connection the cell has open for no reason.
func installTunnel(instance string, network config.Network, progress io.Writer) error {
	if network.Tunnel == nil {
		return nil
	}
	if network.HostResolverOutsideTunnel() {
		fmt.Fprint(progress, "Warning: network.resolvers names host, which the tunnel does not carry.\n"+
			"  The cell's traffic leaves through the tunnel, but every name it looks up is\n"+
			"  still asked of your own network's resolver. Name an address instead to keep\n"+
			"  what this cell resolves inside the tunnel.\n")
	}

	// A missing file is the ordinary case on a machine that has never had
	// one, and reads as an error here; whether the tunnel is installed is
	// decided by what came back, not by whether the command succeeded.
	out, _ := lima.Exec(instance, "sudo", "sha256sum", config.VPNConfigFile)
	if installed, _, _ := strings.Cut(strings.TrimSpace(string(out)), " "); installed == network.Tunnel.Digest {
		return nil
	}

	fmt.Fprintf(progress, "Bringing up the tunnel to %s...\n", network.Tunnel.EndpointHost)

	// Whatever is there is not what is about to be copied, and a copy lands
	// inside a directory that happens to have the staging path's name rather
	// than replacing it.
	if _, err := lima.Exec(instance, "rm", "-rf", tunnelStaging); err != nil {
		return fmt.Errorf("bringing up the tunnel: %w", err)
	}
	if err := lima.CopyFile(network.VPNPath, instance, tunnelStaging); err != nil {
		return fmt.Errorf("copying the tunnel configuration into the machine: %w", err)
	}
	steps := [][]string{
		{"sudo", "install", "-D", "-m", "600", "-o", "root", "-g", "root", tunnelStaging, config.VPNConfigFile},
		{"rm", "-f", tunnelStaging},
		{"sudo", "systemctl", "enable", "wg-quick@" + config.VPNInterface},
		// restart rather than start: this runs when the configuration
		// changed, and a tunnel already up would otherwise keep the old one.
		{"sudo", "systemctl", "restart", "wg-quick@" + config.VPNInterface},
	}
	for _, step := range steps {
		if _, err := lima.Exec(instance, step...); err != nil {
			return fmt.Errorf("bringing up the tunnel: %w", err)
		}
	}

	return nil
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
	instance, user, err := attachable(name)
	if err != nil {
		return err
	}

	return exitStatus(podman.Shell(instance, user))
}

// ShellCommand prepares the shell Shell would open, without opening it, for a
// caller that has to run the process itself — the dashboard hands it the
// terminal and takes it back when it exits. The cell has to be usable, checked
// the same way as for Shell.
func ShellCommand(name string) (*exec.Cmd, error) {
	instance, user, err := attachable(name)
	if err != nil {
		return nil, err
	}

	return podman.ShellCommand(instance, user)
}

// TrafficCommand prepares a command that follows what a cell's network is
// doing: every name it asks about, every answer it gets, and every connection
// the firewall refuses.
//
// It reads the machine's log rather than the container's, because that is where
// both halves of the allow list live — and because a container cannot see, let
// alone edit, what is recorded about it.
//
// The command runs until it is killed, so the caller owns its lifetime.
func TrafficCommand(name string) (*exec.Cmd, error) {
	if _, err := config.LoadCell(name); err != nil {
		return nil, err
	}

	instance := config.Instance(name)
	inst, err := lima.Lookup(instance)
	if err != nil {
		return nil, err
	}
	if inst == nil || inst.Status != lima.StatusRunning {
		return nil, fmt.Errorf("%w: run 'solitary up %s'", ErrNotRunning, name)
	}

	// Filtered in the machine: following the whole journal would send far
	// more over the connection than is ever displayed. Matched on who wrote
	// each line rather than on its text — the resolver's name is in the
	// identifier, not in the message, so a text match finds none of it — and
	// "+" is how journalctl is asked for either.
	return lima.Command(instance, "sudo", "journalctl", "--follow", "--lines=50",
		"--output=short-iso", "--no-pager",
		"SYSLOG_IDENTIFIER=kernel", "+", "SYSLOG_IDENTIFIER=dnsmasq")
}

// Traffic is one thing a cell's network did.
type Traffic struct {
	At     string
	Kind   TrafficKind
	Detail string
}

// TrafficKind is what happened, which is what decides how a line reads.
type TrafficKind string

const (
	// TrafficQuery is a name the cell asked about.
	TrafficQuery TrafficKind = "query"
	// TrafficResolved is a name it was given an address for, which is also
	// what opens the firewall for that address.
	TrafficResolved TrafficKind = "resolved"
	// TrafficRefused is a name that is not on the allow list, so it does not
	// resolve at all.
	TrafficRefused TrafficKind = "refused"
	// TrafficDenied is a connection the firewall dropped.
	TrafficDenied TrafficKind = "denied"
)

var (
	queryLine    = regexp.MustCompile(`query\[[A-Z]+\] (\S+) from`)
	answerLine   = regexp.MustCompile(`(?:reply|cached) (\S+) is (\S+)`)
	refusedLine  = regexp.MustCompile(`config (\S+) is NXDOMAIN`)
	deniedFields = regexp.MustCompile(`DST=(\S+).*?(?:DPT=(\d+))?`)
	deniedPort   = regexp.MustCompile(`DPT=(\d+)`)
	logTime      = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}T(\d{2}:\d{2}:\d{2}))`)
)

// ParseTraffic reads one journal line. It reports ok=false for a line that says
// nothing about the network, which most lines do.
func ParseTraffic(line string) (Traffic, bool) {
	at := ""
	if m := logTime.FindStringSubmatch(line); m != nil {
		at = m[2]
	}

	switch {
	case strings.Contains(line, "solitary-deny"):
		target := ""
		if m := deniedFields.FindStringSubmatch(line); m != nil {
			target = m[1]
		}
		if m := deniedPort.FindStringSubmatch(line); m != nil {
			target += ":" + m[1]
		}
		if target == "" {
			return Traffic{}, false
		}

		return Traffic{At: at, Kind: TrafficDenied, Detail: target}, true

	case refusedLine.MatchString(line):
		m := refusedLine.FindStringSubmatch(line)

		return Traffic{At: at, Kind: TrafficRefused, Detail: m[1]}, true

	case answerLine.MatchString(line):
		m := answerLine.FindStringSubmatch(line)
		// filter-AAAA answers every AAAA question with nothing, which is
		// an implementation detail rather than something that happened.
		if strings.HasPrefix(m[2], "NODATA") || strings.HasPrefix(m[2], "NXDOMAIN") {
			return Traffic{}, false
		}

		return Traffic{At: at, Kind: TrafficResolved, Detail: m[1] + " → " + m[2]}, true

	case queryLine.MatchString(line):
		m := queryLine.FindStringSubmatch(line)

		return Traffic{At: at, Kind: TrafficQuery, Detail: m[1]}, true
	}

	return Traffic{}, false
}

// SecretState is whether one of a cell's declared secrets has a value. The
// value itself is deliberately absent: this describes secrets for display, and
// nothing displaying them should be able to reveal one.
type SecretState struct {
	Name        string
	Set         bool
	Required    bool
	Description string
}

// Detail is what a cell's definition says about it, alongside which of its
// secrets have values.
//
// A cell's state is deliberately not here: it costs a call into Lima to learn,
// while this is read from files. List reports state for every cell at once, so
// a caller showing both already has it.
type Detail struct {
	Name        string
	Description string
	Image       string
	VM          config.VM
	Ports       []int
	Devices     []string
	Network     config.Network
	Secrets     []SecretState

	// ProvisionChanged reports that vm.provision differs from the script the
	// machine was given. Unlike every other setting, restarting the machine
	// does not make it match the file: the old script has already run, and
	// only destroying the machine discards what it did.
	ProvisionChanged bool
}

// Describe reads a cell's definition.
func Describe(name string) (Detail, error) {
	c, err := config.LoadCell(name)
	if err != nil {
		return Detail{}, err
	}

	detail := Detail{
		Name:        name,
		Description: c.Description,
		Image:       c.Image,
		VM:          c.VM,
		Ports:       c.Ports,
		Devices:     c.Devices,
		Network:     c.Network,
	}
	if c.Build != "" {
		detail.Image = "build:" + c.Build
	}

	record, err := config.ReadApplied(name)
	if err != nil {
		return Detail{}, err
	}
	detail.ProvisionChanged = record.ProvisionChanged(c.VM.Provision)

	if len(c.Secrets) > 0 {
		path, err := config.EnvFile(name)
		if err != nil {
			return Detail{}, err
		}
		values, err := secrets.Load(path)
		if err != nil {
			return Detail{}, err
		}
		for _, declared := range c.Secrets {
			detail.Secrets = append(detail.Secrets, SecretState{
				Name:        declared.Name,
				Set:         values[declared.Name] != "",
				Required:    declared.Required,
				Description: declared.Description,
			})
		}
	}

	return detail, nil
}

// ExitError reports that the command run by Exec failed. It carries the status
// the command exited with, so that solitary can exit with it too and stay
// usable from a script.
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("command exited with status %d", e.Code)
}

// Exec runs one command inside a running cell. Like Shell it never changes
// state: it is for asking a cell something, not for setting it up.
func Exec(name string, command []string) error {
	instance, user, err := attachable(name)
	if err != nil {
		return err
	}

	return exitStatus(podman.Exec(instance, user, command))
}

// exitStatus recognises a command that ran and failed. That is the command's
// own result rather than something going wrong with solitary, so it is reported
// as a status to exit with, leaving the explaining to whatever the command
// already printed.
func exitStatus(err error) error {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return &ExitError{Code: exit.ExitCode()}
	}

	return err
}

// attachable returns the machine to run in and the user to run there as, or
// explains why the cell cannot be attached to. It changes nothing: a cell that
// is not up stays that way.
//
// The user comes from the running container rather than from cell.yaml: it is
// the one the home was mapped for, and a definition edited since the cell came
// up has not reached the container yet.
func attachable(name string) (instance, user string, err error) {
	if _, err := config.LoadCell(name); err != nil {
		return "", "", err
	}

	instance = config.Instance(name)
	inst, err := lima.Lookup(instance)
	if err != nil {
		return "", "", err
	}
	switch {
	case inst == nil:
		return "", "", fmt.Errorf("cell %q does not exist yet: run 'solitary up %s'", name, name)
	case inst.Status != lima.StatusRunning:
		return "", "", fmt.Errorf("%w: run 'solitary up %s'", ErrNotRunning, name)
	}

	state, err := podman.Inspect(instance)
	if err != nil {
		return "", "", err
	}
	if !state.Running {
		return "", "", fmt.Errorf("the container in %q is not running: run 'solitary up %s'", name, name)
	}

	return instance, state.User, nil
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
//
// Only call this before a machine takes its memory. A running machine's memory
// is a file in the backing store, so measuring free space while it is up counts
// the machine's own memory as missing and warns that what is already running
// will not fit.
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

// verifyDisk refuses a vm.disk smaller than the disk the machine already has.
//
// Lima grows a disk when a machine starts and gives up on a smaller one with
// "disk shrinking is not supported" — after the new definition has been written
// in, which leaves the machine holding a definition that cannot start it and
// says nothing about which setting was at fault. Asking first turns that into a
// sentence naming the cell, both sizes and the two ways out.
//
// A disk whose size cannot be read is not a reason to refuse: the check is
// skipped rather than guessed at, and Lima still has the last word.
func verifyDisk(name, instance, disk string) error {
	actual := lima.DiskSize(instance)
	if actual == 0 {
		return nil
	}

	// A size that does not parse reads as zero and is left alone: what
	// vm.disk may say is Lima's to rule on, and refusing here would stop a
	// machine over a string solitary only passes through.
	want, _ := host.ParseSize(disk)
	if want == 0 || want >= actual {
		return nil
	}

	path, err := config.CellFile(name)
	if err != nil {
		return err
	}

	return diskRefusal(name, path, want, actual)
}

// diskRefusal is the sentence verifyDisk refuses with, separated from the host
// it measures so that what it says can be tested.
func diskRefusal(name, path string, want, actual uint64) error {
	return fmt.Errorf("cell %q asks for a %s disk, but its machine already has %s, and a disk cannot be shrunk.\n"+
		"Set vm.disk to %s or more in %s,\n"+
		"or discard the machine and everything on it with 'solitary rm %s' and run up again",
		name, host.FormatSize(want), host.FormatSize(actual), wholeGiB(actual), path, name)
}

// wholeGiB renders a size as a whole number of GiB, rounded up, for a value
// someone is being asked to write into cell.yaml. FormatSize is right for
// reporting what was measured; a setting reads better without a decimal, and
// rounding up keeps the suggestion above what it has to clear.
func wholeGiB(b uint64) string {
	const gib = 1 << 30

	return fmt.Sprintf("%dGiB", (b+gib-1)/gib)
}

// warnDrift reports settings that changed since the machine started. Lima
// cannot apply them to a running machine, and silently ignoring them would
// leave the cell running a definition the file no longer describes.
func warnDrift(name string, c *config.Cell, rendered string, w io.Writer) {
	record, err := config.ReadApplied(name)
	if err != nil || !record.Recorded() {
		return
	}
	if record.Definition != config.Digest(rendered) {
		fmt.Fprintf(w, "Warning: the machine settings for %q changed since it started.\n", name)
		fmt.Fprintf(w, "         vm, ports and network are read when the machine boots, so the running\n")
		fmt.Fprintf(w, "         cell still uses the old ones. To apply the change:\n")
		fmt.Fprintf(w, "           solitary down %s && solitary up %s\n", name, name)
	}
	warnProvision(name, c, record, w)
}

// warnProvision reports a vm.provision script that changed since the machine
// was given one.
//
// Every other setting can be applied to a machine by restarting it. This one
// cannot be taken back: the old script has already installed what it installs,
// and a machine keeps its disk across every change short of being destroyed. So
// the new script runs at the next start, on top of whatever the old one left —
// which is worth saying, because it is the one case where doing what solitary
// tells you is not enough to make the machine match the file.
func warnProvision(name string, c *config.Cell, record config.Applied, w io.Writer) {
	if !record.ProvisionChanged(c.VM.Provision) {
		return
	}

	fmt.Fprintf(w, "Warning: vm.provision for %q changed since its machine was provisioned.\n", name)
	fmt.Fprintf(w, "         The new script runs at the next start, but what the old one did is\n")
	fmt.Fprintf(w, "         already on the machine's disk and nothing undoes it. For a machine\n")
	fmt.Fprintf(w, "         built by this script alone — discarding the disk, and the cell's home\n")
	fmt.Fprintf(w, "         with it:\n")
	fmt.Fprintf(w, "           solitary rm %s && solitary up %s\n", name, name)
}

// applyDrift gives a stopped machine the definition the cell now describes.
//
// vm, ports and network are settings a machine reads at boot, so a stopped one
// can be handed new ones and simply start with them. Only what solitary
// generates is replaced, and only when there is a reason to: a machine whose
// definition is known to match what the cell says is not rewritten.
func applyDrift(name, instance string, c *config.Cell, rendered string, progress io.Writer) error {
	record, err := config.ReadApplied(name)
	if err != nil {
		return err
	}

	// A machine with no record is written and recorded rather than assumed to
	// match. Nothing is known about it — it was created before solitary kept
	// the record, or the state directory was cleared — and reading that as
	// "unchanged" is how a new memory size, cpu count or allow list silently
	// never reaches it.
	changed := record.Definition != config.Digest(rendered)
	if record.Recorded() && !changed {
		if record.Provision == "" {
			// An older record, which kept the definition's digest and
			// nothing else. The definition it describes is the one being
			// rendered now, and vm.provision is part of that definition, so
			// the script the machine was given is the script in hand: the
			// record can be completed without touching the machine.
			return config.WriteApplied(name, config.NewApplied(rendered, c.VM.Provision))
		}

		return nil
	}

	if changed && record.Recorded() {
		fmt.Fprintf(progress, "The machine settings for %q changed; applying them to its machine.\n", name)
	}
	// Said before the machine starts, since it is about what starting it will
	// not do. The record is replaced below, so this is said once, when the
	// script changes, rather than at every start afterwards.
	warnProvision(name, c, record, progress)

	if err := lima.Apply(instance, rendered); err != nil {
		return err
	}

	return config.WriteApplied(name, config.NewApplied(rendered, c.VM.Provision))
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

	changed, err := secrets.Prompt(progress, fields(c.Secrets, nil), values)
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
