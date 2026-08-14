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

// Attach runs a command inside a machine with the terminal attached, so that
// interactive programs work. The command's exit status is returned as an error.
func Attach(name string, args ...string) error {
	bin, err := limactl()
	if err != nil {
		return err
	}

	full := append([]string{"shell", "--workdir=/", name}, args...)
	cmd := exec.Command(bin, full...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
