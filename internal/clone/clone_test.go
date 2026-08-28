package clone

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/balakin/solitary/internal/config"
)

// definition is the smallest thing that is a cell.
const definition = "image: docker.io/library/ubuntu:24.04\n"

// write puts a file into a tree, creating what it needs on the way.
func write(t *testing.T, path, content string, perm os.FileMode) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), perm); err != nil {
		t.Fatal(err)
	}
}

// isolate points the config at a temporary directory, so a test never reads or
// writes the cells of whoever is running it.
func isolate(t *testing.T) {
	t.Helper()

	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
}

func TestWithheld(t *testing.T) {
	for _, name := range []string{".env", ".ENV", "vpn.conf", "prod.env", "staging.Env"} {
		if !withheld(name) {
			t.Errorf("%q would have been copied", name)
		}
	}
	for _, name := range []string{"cell.yaml", "Containerfile", "env.md", "readme.env.example"} {
		if withheld(name) {
			t.Errorf("%q was refused, and holds no credential", name)
		}
	}
}

func TestCopyTree(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()

	write(t, filepath.Join(src, "cell.yaml"), definition, 0o644)
	write(t, filepath.Join(src, "Containerfile"), "FROM ubuntu\n", 0o644)
	write(t, filepath.Join(src, "setup.sh"), "#!/bin/sh\n", 0o755)
	write(t, filepath.Join(src, "files", "nested", "vimrc"), "set nu\n", 0o644)
	write(t, filepath.Join(src, ".env"), "GITHUB_TOKEN=leaked\n", 0o600)
	write(t, filepath.Join(src, "vpn.conf"), "[Interface]\n", 0o600)
	if err := os.Symlink("/etc/passwd", filepath.Join(src, "passwd")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	refused, err := copyTree(src, dst)
	if err != nil {
		t.Fatalf("copyTree: %v", err)
	}

	for _, want := range []string{"cell.yaml", "Containerfile", "setup.sh", filepath.Join("files", "nested", "vimrc")} {
		if _, err := os.Stat(filepath.Join(dst, want)); err != nil {
			t.Errorf("%s did not arrive: %v", want, err)
		}
	}
	for _, gone := range []string{".env", "vpn.conf", "passwd"} {
		if _, err := os.Lstat(filepath.Join(dst, gone)); err == nil {
			t.Errorf("%s arrived, and should not have", gone)
		}
	}

	// Refused by name, not silently: the repository meant to hand these
	// over and they are not coming.
	if len(refused) != 3 {
		t.Fatalf("refused %d entries, want 3: %v", len(refused), refused)
	}
	for _, name := range []string{".env", "vpn.conf", "passwd"} {
		if !strings.Contains(strings.Join(refused, "\n"), name) {
			t.Errorf("nothing said about %s: %v", name, refused)
		}
	}

	modes := map[string]os.FileMode{"cell.yaml": 0o600, "setup.sh": 0o700}
	for name, want := range modes {
		info, err := os.Stat(filepath.Join(dst, name))
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Errorf("%s is %o, want %o", name, got, want)
		}
	}
}

func TestCopyTreeSkipsGit(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()

	write(t, filepath.Join(src, "cell.yaml"), definition, 0o644)
	write(t, filepath.Join(src, ".git", "config"), "[core]\n", 0o644)

	refused, err := copyTree(src, dst)
	if err != nil {
		t.Fatalf("copyTree: %v", err)
	}
	if len(refused) != 0 {
		t.Errorf("complained about .git, which is not the repository's doing: %v", refused)
	}
	if _, err := os.Stat(filepath.Join(dst, ".git")); err == nil {
		t.Error(".git arrived")
	}
}

func TestLocate(t *testing.T) {
	t.Run("repository that is one cell", func(t *testing.T) {
		repo := t.TempDir()
		write(t, filepath.Join(repo, "cell.yaml"), definition, 0o644)

		dir, err := locate(repo, Source{Display: "one"})
		if err != nil {
			t.Fatalf("locate: %v", err)
		}
		if dir != repo {
			t.Errorf("located %s, want the root %s", dir, repo)
		}
	})

	t.Run("catalogue with none named", func(t *testing.T) {
		repo := t.TempDir()
		for _, name := range []string{"rust", "claude", "go"} {
			write(t, filepath.Join(repo, name, "cell.yaml"), definition, 0o644)
		}
		write(t, filepath.Join(repo, "docs", "README.md"), "hi\n", 0o644)

		_, err := locate(repo, Source{Display: "many"})

		var catalogue *Catalogue
		if !errors.As(err, &catalogue) {
			t.Fatalf("locate returned %v, want a catalogue", err)
		}
		if got := strings.Join(catalogue.Cells, ","); got != "claude,go,rust" {
			t.Errorf("cells = %q, want the three of them in order", got)
		}
	})

	t.Run("catalogue holding nothing", func(t *testing.T) {
		repo := t.TempDir()
		write(t, filepath.Join(repo, "README.md"), "hi\n", 0o644)

		_, err := locate(repo, Source{Display: "empty"})

		var catalogue *Catalogue
		if !errors.As(err, &catalogue) || len(catalogue.Cells) != 0 {
			t.Fatalf("locate returned %v, want an empty catalogue", err)
		}
	})

	t.Run("named cell that is not there", func(t *testing.T) {
		repo := t.TempDir()
		write(t, filepath.Join(repo, "rust", "cell.yaml"), definition, 0o644)

		_, err := locate(repo, Source{Display: "many", Path: "claude"})
		if err == nil {
			t.Fatal("locate found a cell that is not there")
		}
		// The error is the only place someone learns what is there.
		if !strings.Contains(err.Error(), "rust") {
			t.Errorf("error does not say what the repository holds: %v", err)
		}
	})
}

// repository makes a git repository on disk, so that Stage can be exercised
// end to end without a network.
func repository(t *testing.T, build func(dir string)) string {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}

	dir := t.TempDir()
	build(dir)

	for _, args := range [][]string{
		{"init", "--quiet"},
		{"-c", "user.email=t@example.com", "-c", "user.name=t", "add", "-A"},
		{"-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "--quiet", "-m", "cells"},
	} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	return dir
}

func TestStageAndInstall(t *testing.T) {
	isolate(t)

	repo := repository(t, func(dir string) {
		write(t, filepath.Join(dir, "claude", "cell.yaml"),
			"build: ./Containerfile\nsecrets:\n  GITHUB_TOKEN:\nnetwork:\n  allow:\n    - github.com\n", 0o644)
		write(t, filepath.Join(dir, "claude", "Containerfile"), "FROM ubuntu\n", 0o644)
		write(t, filepath.Join(dir, "claude", ".env"), "GITHUB_TOKEN=leaked\n", 0o600)
		write(t, filepath.Join(dir, "rust", "cell.yaml"), definition, 0o644)
	})

	source, err := Parse(repo + "#claude")
	if err != nil {
		t.Fatal(err)
	}

	staged, err := Stage(source, os.Stderr)
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}
	defer staged.Discard()

	if staged.Name != "claude" {
		t.Errorf("default name %q, want claude", staged.Name)
	}
	if staged.Cell.Build != "./Containerfile" {
		t.Errorf("build = %q, want ./Containerfile", staged.Cell.Build)
	}
	if len(staged.Refused) != 1 || !strings.Contains(staged.Refused[0], ".env") {
		t.Errorf("refused = %v, want the .env named", staged.Refused)
	}

	written, err := Install(staged, "claude", false)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(written) != 2 {
		t.Errorf("wrote %v, want cell.yaml and Containerfile", written)
	}

	// It is a cell now, by the same reading every other command does.
	loaded, err := config.LoadCell("claude")
	if err != nil {
		t.Fatalf("LoadCell after install: %v", err)
	}
	if len(loaded.Secrets) != 1 || loaded.Secrets[0].Name != "GITHUB_TOKEN" {
		t.Errorf("secrets = %v, want GITHUB_TOKEN", loaded.Secrets)
	}

	env, err := config.EnvFile("claude")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(env); err == nil {
		t.Error("the repository's .env was installed")
	}
}

func TestInstallRefusesExistingCell(t *testing.T) {
	isolate(t)

	repo := repository(t, func(dir string) {
		write(t, filepath.Join(dir, "cell.yaml"), definition, 0o644)
	})

	source, err := Parse(repo)
	if err != nil {
		t.Fatal(err)
	}
	staged, err := Stage(source, os.Stderr)
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}
	defer staged.Discard()

	if _, err := Install(staged, "work", false); err != nil {
		t.Fatalf("first install: %v", err)
	}

	// A second clone of the same source has to be asked for: either as
	// another cell, or as a replacement.
	_, err = Install(staged, "work", false)
	if err == nil {
		t.Fatal("the second install overwrote the first without being asked")
	}
	for _, mention := range []string{"--as", "--force"} {
		if !strings.Contains(err.Error(), mention) {
			t.Errorf("error does not offer %s: %v", mention, err)
		}
	}

	if _, err := Install(staged, "second", false); err != nil {
		t.Fatalf("installing under another name: %v", err)
	}
}

func TestInstallForceKeepsSecrets(t *testing.T) {
	isolate(t)

	repo := repository(t, func(dir string) {
		write(t, filepath.Join(dir, "cell.yaml"), definition, 0o644)
	})

	source, err := Parse(repo)
	if err != nil {
		t.Fatal(err)
	}
	staged, err := Stage(source, os.Stderr)
	if err != nil {
		t.Fatalf("Stage: %v", err)
	}
	defer staged.Discard()

	if _, err := Install(staged, "work", false); err != nil {
		t.Fatalf("first install: %v", err)
	}

	// What the host holds and the repository never did: a secret and a
	// tunnel, both of which have to survive an update.
	dir, err := config.CellDir("work")
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, ".env"), "GITHUB_TOKEN=mine\n", 0o600)
	write(t, filepath.Join(dir, "vpn.conf"), "[Interface]\n", 0o600)
	write(t, filepath.Join(dir, "cell.yaml"), "image: stale\n", 0o600)

	if _, err := Install(staged, "work", true); err != nil {
		t.Fatalf("replacing with --force: %v", err)
	}

	updated, err := os.ReadFile(filepath.Join(dir, "cell.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(updated) != definition {
		t.Errorf("cell.yaml = %q, want the repository's copy", updated)
	}

	kept, err := os.ReadFile(filepath.Join(dir, ".env"))
	if err != nil {
		t.Fatalf("the .env is gone: %v", err)
	}
	if string(kept) != "GITHUB_TOKEN=mine\n" {
		t.Errorf(".env = %q, want it untouched", kept)
	}
	if _, err := os.Stat(filepath.Join(dir, "vpn.conf")); err != nil {
		t.Errorf("the vpn.conf is gone: %v", err)
	}
}

func TestStageAcceptsAMissingTunnel(t *testing.T) {
	isolate(t)

	// A published definition names a vpn.conf it deliberately does not
	// carry. That is the shape sharing takes, not a broken cell.
	repo := repository(t, func(dir string) {
		write(t, filepath.Join(dir, "cell.yaml"),
			"image: docker.io/library/ubuntu:24.04\nnetwork:\n  vpn: ./vpn.conf\n  allow:\n    - github.com\n", 0o644)
	})

	source, err := Parse(repo)
	if err != nil {
		t.Fatal(err)
	}
	staged, err := Stage(source, os.Stderr)
	if err != nil {
		t.Fatalf("Stage refused a definition whose tunnel is the runner's: %v", err)
	}
	defer staged.Discard()

	if staged.Cell.Network.VPN != "./vpn.conf" {
		t.Errorf("vpn = %q, want ./vpn.conf", staged.Cell.Network.VPN)
	}
}
