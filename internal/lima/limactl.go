package lima

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ErrNotInstalled is returned when limactl is not on PATH.
var ErrNotInstalled = errors.New("limactl not found on PATH: install Lima from https://lima-vm.io")

// Instance is one Lima machine, as reported by limactl list --json.
type Instance struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Dir    string `json:"dir"`
}

// Machine statuses reported by Lima.
const (
	StatusRunning = "Running"
	StatusStopped = "Stopped"
	StatusBroken  = "Broken"
)

func limactl() (string, error) {
	path, err := exec.LookPath("limactl")
	if err != nil {
		return "", ErrNotInstalled
	}
	return path, nil
}

// run executes limactl with output discarded unless it fails, in which case
// stderr is folded into the error.
func run(args ...string) error {
	bin, err := limactl()
	if err != nil {
		return err
	}

	cmd := exec.Command(bin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return fmt.Errorf("limactl %s: %w", strings.Join(args, " "), err)
		}
		return fmt.Errorf("limactl %s: %w\n%s", strings.Join(args, " "), err, msg)
	}

	return nil
}

// runVerbose executes limactl with its output attached to the terminal, for
// commands where progress matters.
func runVerbose(args ...string) error {
	bin, err := limactl()
	if err != nil {
		return err
	}

	cmd := exec.Command(bin, args...)
	cmd.Stdout = os.Stderr // progress is not output; keep stdout clean
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("limactl %s: %w", strings.Join(args, " "), err)
	}

	return nil
}

// List returns every Lima machine on the host. limactl emits one JSON object
// per line rather than a JSON array.
func List() ([]Instance, error) {
	bin, err := limactl()
	if err != nil {
		return nil, err
	}

	out, err := exec.Command(bin, "list", "--json").Output()
	if err != nil {
		return nil, fmt.Errorf("listing Lima machines: %w", err)
	}

	var instances []Instance
	scanner := bufio.NewScanner(bytes.NewReader(out))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024) // definitions are large
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var inst Instance
		if err := json.Unmarshal(line, &inst); err != nil {
			return nil, fmt.Errorf("parsing limactl list output: %w", err)
		}
		instances = append(instances, inst)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading limactl list output: %w", err)
	}

	return instances, nil
}

// Lookup returns the named machine, or nil when it does not exist.
func Lookup(name string) (*Instance, error) {
	instances, err := List()
	if err != nil {
		return nil, err
	}
	for i := range instances {
		if instances[i].Name == name {
			return &instances[i], nil
		}
	}
	return nil, nil
}

// Create builds a machine from a definition file and starts it.
func Create(name, definitionPath string) error {
	return runVerbose("start", "--tty=false", "--name="+name, definitionPath)
}

// startAttempts is how many times Start retries. limactl stop can return
// before QEMU has released the instance's sockets, and a start that lands in
// that window fails outright rather than waiting.
const startAttempts = 3

// startRetryDelay is how long to wait between attempts.
var startRetryDelay = 3 * time.Second

// Start boots an existing machine.
func Start(name string) error {
	var err error
	for attempt := 1; attempt <= startAttempts; attempt++ {
		if err = runVerbose("start", "--tty=false", name); err == nil {
			return nil
		}
		if attempt < startAttempts {
			time.Sleep(startRetryDelay)
		}
	}

	return fmt.Errorf("starting %s after %d attempts: %w", name, startAttempts, err)
}

// Stop shuts a machine down, keeping its disk, and waits for it to actually be
// gone so that a start straight afterwards does not race the teardown.
func Stop(name string) error {
	if err := runVerbose("stop", name); err != nil {
		return err
	}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		inst, err := Lookup(name)
		if err != nil {
			return err
		}
		if inst == nil || inst.Status != StatusRunning {
			return nil
		}
		time.Sleep(time.Second)
	}

	return fmt.Errorf("%s did not stop within 30s", name)
}

// Copy places a host directory inside a machine. The directory itself is
// created under target, not merged into it.
func Copy(hostPath, name, target string) error {
	return run("copy", "--recursive", hostPath, name+":"+target)
}

// Delete destroys a machine and its disk.
func Delete(name string) error {
	return run("delete", "--force", name)
}

// Apply replaces a stopped machine's definition with a new one, so that
// settings a machine takes at boot — how much memory it gets, which ports reach
// the host — can change without destroying the disk that machine carries.
//
// Lima keeps the definition it was created from inside the machine's own
// directory and reads it again on every start. Writing it there is what
// 'limactl edit' does; this does the same thing without an editor.
//
// The machine must be stopped: a running one has already read the file.
func Apply(name, definition string) error {
	// What a machine stores is not the template it was created from: Lima
	// resolves base: into the image list it picked, and panics on a stored
	// definition that still refers to a template. Resolve it here, which
	// also validates it — a machine must never be left holding a definition
	// that cannot start it.
	resolved, err := resolve(definition)
	if err != nil {
		return err
	}

	dir, err := dirOf(name)
	if err != nil {
		return err
	}

	path := filepath.Join(dir, "lima.yaml")
	// Written beside the file it replaces and renamed over it, so that an
	// interrupted write cannot leave a machine with half a definition.
	temp, err := os.CreateTemp(dir, "lima.yaml.*")
	if err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	defer os.Remove(temp.Name())

	if _, err := temp.WriteString(resolved); err != nil {
		temp.Close()
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := os.Chmod(temp.Name(), 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := os.Rename(temp.Name(), path); err != nil {
		return fmt.Errorf("replacing %s: %w", path, err)
	}

	return nil
}

// resolve turns a template into the form a machine stores: every reference it
// names — the base template, the images it chooses — replaced by what it refers
// to. It fails when the definition is not valid, which is the point of doing it
// before anything is replaced.
func resolve(definition string) (string, error) {
	bin, err := limactl()
	if err != nil {
		return "", err
	}

	temp, err := os.CreateTemp("", "solitary-*.yaml")
	if err != nil {
		return "", fmt.Errorf("resolving the machine definition: %w", err)
	}
	defer os.Remove(temp.Name())

	if _, err := temp.WriteString(definition); err != nil {
		temp.Close()
		return "", fmt.Errorf("resolving the machine definition: %w", err)
	}
	if err := temp.Close(); err != nil {
		return "", fmt.Errorf("resolving the machine definition: %w", err)
	}

	cmd := exec.Command(bin, "template", "copy", "--embed-all", temp.Name(), "-")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		return "", fmt.Errorf("resolving the machine definition: %w\n%s", err, msg)
	}
	if stdout.Len() == 0 {
		return "", errors.New("resolving the machine definition produced nothing")
	}

	return stdout.String(), nil
}

// dirOf asks Lima where a machine keeps its files, rather than assuming the
// default location: LIMA_HOME moves it.
func dirOf(name string) (string, error) {
	bin, err := limactl()
	if err != nil {
		return "", err
	}

	out, err := exec.Command(bin, "list", "--format", "{{.Dir}}", name).Output()
	if err != nil {
		return "", fmt.Errorf("finding the directory of machine %q: %w", name, err)
	}

	dir := strings.TrimSpace(string(out))
	if dir == "" {
		return "", fmt.Errorf("machine %q has no directory", name)
	}

	return dir, nil
}

// ErrUnreachable means a machine Lima considers running did not answer.
//
// A guest can die in a way that leaves its process alive and its status
// unchanged — memory the host cannot back is one way — and every command then
// blocks forever rather than failing. Commands are given a deadline so that
// state surfaces as an error instead of a hang.
var ErrUnreachable = errors.New("machine is not responding")

// execTimeout bounds a single command inside a machine.
var execTimeout = 30 * time.Second

// Exec runs a command inside a machine, without a terminal, and returns its
// standard output.
func Exec(name string, args ...string) ([]byte, error) {
	bin, err := limactl()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), execTimeout)
	defer cancel()

	full := append([]string{"shell", "--workdir=/", name}, args...)
	cmd := exec.CommandContext(ctx, bin, full...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return stdout.Bytes(), fmt.Errorf("%s: %w after %s", name, ErrUnreachable, execTimeout)
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return stdout.Bytes(), fmt.Errorf("%s: %w", strings.Join(args, " "), err)
		}
		return stdout.Bytes(), fmt.Errorf("%s: %w\n%s", strings.Join(args, " "), err, msg)
	}

	return stdout.Bytes(), nil
}

// Reachable reports whether a running machine answers a trivial command.
func Reachable(name string) bool {
	_, err := Exec(name, "true")
	return err == nil
}

// Command prepares a command to run inside a machine, without running it. It is
// for callers that have to own the process themselves — handing the terminal to
// it and taking it back, say — rather than waiting for it like Attach does.
func Command(name string, args ...string) (*exec.Cmd, error) {
	bin, err := limactl()
	if err != nil {
		return nil, err
	}

	full := append([]string{"shell", "--workdir=/", name}, args...)

	return exec.Command(bin, full...), nil
}

// Attach runs a command inside a machine with the terminal attached, so that
// interactive programs work. The command's exit status is returned as an error.
func Attach(name string, args ...string) error {
	cmd, err := Command(name, args...)
	if err != nil {
		return err
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
