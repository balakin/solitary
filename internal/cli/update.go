package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/balakin/solitary/internal/update"
)

func newUpdateCmd() *cobra.Command {
	var check bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update solitary to the latest release",
		Long: "Replaces the running binary with the newest release, after checking it\n" +
			"against the published checksum. An install a package manager owns is\n" +
			"left to that package manager, which this says rather than fights.\n\n" +
			"Every command also mentions a newer release once a day; set " + update.DisableEnv + "\n" +
			"to keep solitary from asking github anything on its own.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()

			latest, err := update.Latest(cmd.Context())
			if err != nil {
				return err
			}

			// A build from source is ahead of the release it names,
			// not behind it, and replacing it would throw away what
			// was built.
			if !update.IsRelease(version) {
				fmt.Fprintf(out, "This is a build from source. The latest release is %s: %s\n", latest, update.Releases)
				return nil
			}

			if !update.Newer(version, latest) {
				fmt.Fprintf(out, "solitary %s is the latest release.\n", version)
				return nil
			}

			if check {
				fmt.Fprintf(out, "solitary %s is out (this is %s). Update with: solitary update\n", latest, version)
				return nil
			}

			fmt.Fprintf(out, "Updating %s -> %s\n", version, latest)
			if err := update.Install(cmd.Context(), latest); err != nil {
				return err
			}
			fmt.Fprintf(out, "Updated to %s.\n", latest)

			return nil
		},
	}

	cmd.Flags().BoolVar(&check, "check", false, "report whether a newer release exists, without installing it")

	return cmd
}
