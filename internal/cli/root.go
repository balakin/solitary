// Package cli wires up the solitary command tree.
package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/balakin/solitary/internal/cell"
	"github.com/balakin/solitary/internal/update"
)

// version is overwritten at build time via -ldflags.
var version = "dev"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "solitary",
		Short:         "Run coding agents on a VM, not on your machine",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(
		newInitCmd(),
		newCloneCmd(),
		newUpCmd(),
		newShellCmd(),
		newExecCmd(),
		newDownCmd(),
		newRmCmd(),
		newLsCmd(),
		newDoctorCmd(),
		newFetchCmd(),
		newSendCmd(),
		newDashboardCmd(),
		newSecretsCmd(),
		newUpdateCmd(),
	)

	return root
}

// confirm asks a yes/no question, defaulting to no. It is asked before the two
// things worth being sure of: destroying a machine, and installing a definition
// someone else wrote.
func confirm(cmd *cobra.Command, prompt string) (bool, error) {
	fmt.Fprint(cmd.ErrOrStderr(), prompt)

	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil {
		if errors.Is(err, io.EOF) {
			return false, nil // nothing to read from: refuse rather than guess
		}
		return false, fmt.Errorf("reading answer: %w", err)
	}

	answer := strings.ToLower(strings.TrimSpace(line))

	return answer == "y" || answer == "yes", nil
}

// Main runs the CLI and exits with a non-zero status on failure.
func Main() {
	cmd, err := newRootCmd().ExecuteC()
	if err == nil {
		notify(cmd)
		return
	}

	// A command run inside a cell that exits non-zero is not solitary
	// failing: pass its status through and say nothing, so that exec can
	// stand in for the command it ran.
	var exit *cell.ExitError
	if errors.As(err, &exit) {
		os.Exit(exit.Code)
	}

	fmt.Fprintln(os.Stderr, "solitary:", err)
	os.Exit(1)
}

// notify mentions a newer release, at most once a day and only to a person:
// piped output belongs to whatever is reading it, and update says this itself.
//
// exec is left out along with update: it stands in for the command it ran, and
// nothing solitary has to say belongs in that command's output.
func notify(cmd *cobra.Command) {
	switch cmd.Name() {
	case "update", "exec", "completion":
		return
	}
	if !term.IsTerminal(int(os.Stderr.Fd())) {
		return
	}

	if notice := update.Notice(cmd.Context(), version); notice != "" {
		fmt.Fprintln(os.Stderr, notice)
	}
}
