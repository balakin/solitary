// Package cli wires up the solitary command tree.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is overwritten at build time via -ldflags.
var version = "dev"

// errNotImplemented is returned by every command until the corresponding
// package under internal/ is written.
var errNotImplemented = fmt.Errorf("not implemented yet")

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "solitary",
		Short:         "Hypervisor-isolated cells for running coding agents off the leash",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(
		newInitCmd(),
		newUpCmd(),
		newShellCmd(),
		newDownCmd(),
		newRmCmd(),
		newLsCmd(),
		newSecretsCmd(),
	)

	return root
}

// Main runs the CLI and exits with a non-zero status on failure.
func Main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "solitary:", err)
		os.Exit(1)
	}
}
