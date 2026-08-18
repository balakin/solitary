package cli

import (
	"github.com/spf13/cobra"

	"github.com/balakin/solitary/internal/dashboard"
)

func newDashboardCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dashboard",
		Short: "Manage cells in a live view",
		Long: "Opens a terminal interface listing every cell and its state, refreshed\n" +
			"as it changes. From it you can start, stop and destroy a cell, set the\n" +
			"secrets it is allowed to see, and open a shell in it.\n\n" +
			"It does nothing the other commands cannot: the slow ones it runs as\n" +
			"those commands, so a build prints what a build prints.",
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return dashboard.Run()
		},
	}
}
