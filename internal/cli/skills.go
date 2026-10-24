package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/balakin/solitary/internal/skills"
	shipped "github.com/balakin/solitary/skills"
)

func newSkillsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Teach the coding agents on this host to drive solitary",
		Long: "Installs solitary's agent skills — creating a cell, restricting its network,\n" +
			"troubleshooting one — into the directories coding agents on this host read\n" +
			"skills from. They come from this binary, so they describe the commands it has;\n" +
			"update refreshes them along with itself.",
		Args: cobra.NoArgs,
	}

	cmd.AddCommand(newSkillsInstallCmd(), newSkillsLsCmd(), newSkillsRmCmd())

	return cmd
}

func newSkillsInstallCmd() *cobra.Command {
	var into []string
	var yes, refresh bool

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the skills for every agent found, or into --into",
		Long: "Installs into the skills directory of each coding agent configured in your\n" +
			"home — the agents `npx skills add -g` knows — and into every directory installed\n" +
			"into before. --into names one more, for an agent that is not found. Only\n" +
			"solitary's own skills, the ones named solitary-*, are ever written or removed.\n\n" +
			"A directory not installed into before is asked about first.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()

			recorded, err := skills.Recorded()
			if err != nil {
				return err
			}

			// A refresh is update handing over to the binary it installed,
			// and touches only what was asked for before: a newly configured
			// agent is the user's to opt in.
			var targets []skills.Target
			if !refresh {
				home, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("locating home directory: %w", err)
				}
				targets = skills.Detect(home)
			}
			for _, dir := range recorded {
				targets = addTarget(targets, skills.Target{Dir: dir})
			}
			for _, dir := range into {
				abs, err := filepath.Abs(dir)
				if err != nil {
					return fmt.Errorf("resolving %s: %w", dir, err)
				}
				targets = addTarget(targets, skills.Target{Dir: abs})
			}

			if len(targets) == 0 {
				if refresh {
					return nil
				}
				return errors.New("no coding agent found in your home directory; name the directory your agent reads skills from with --into")
			}

			names, err := skills.Names(shipped.Host())
			if err != nil {
				return err
			}

			// Writing into an agent's configuration is asked for once per
			// directory; after that, keeping what is there current is what
			// was agreed to.
			var fresh []skills.Target
			for _, target := range targets {
				if !slices.Contains(recorded, target.Dir) {
					fresh = append(fresh, target)
				}
			}
			if len(fresh) > 0 && !yes {
				if !term.IsTerminal(int(os.Stdin.Fd())) {
					return fmt.Errorf("no terminal to confirm on; run it with --yes to install into %s", joinDirs(fresh))
				}
				describeInstall(cmd.ErrOrStderr(), names, fresh)
				ok, err := confirm(cmd, "Install them? [y/N] ")
				if err != nil {
					return err
				}
				if !ok {
					fmt.Fprintln(cmd.ErrOrStderr(), "Cancelled.")
					return nil
				}
			}

			dirs := recorded
			for _, target := range targets {
				if err := skills.Install(shipped.Host(), target.Dir); err != nil {
					return err
				}
				if !slices.Contains(dirs, target.Dir) {
					dirs = append(dirs, target.Dir)
				}
			}
			if err := skills.Record(dirs); err != nil {
				return err
			}

			if refresh {
				fmt.Fprintf(out, "Refreshed solitary's agent skills in %s.\n", joinDirs(targets))
				return nil
			}
			fmt.Fprintf(out, "Installed into %s.\n", joinDirs(targets))
			if len(fresh) > 0 {
				fmt.Fprintln(out, "An agent session that was already running picks them up when it starts again.")
				fmt.Fprintln(out, "For an agent that was not found, name its directory: solitary skills install --into <dir>")
			}

			return nil
		},
	}

	cmd.Flags().StringArrayVar(&into, "into", nil, "also install into this skills directory (repeatable)")
	cmd.Flags().BoolVar(&yes, "yes", false, "install without asking")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "rewrite only the directories installed into before")
	_ = cmd.Flags().MarkHidden("refresh")

	return cmd
}

// describeInstall says what install is about to write, and where, before it is
// asked for.
func describeInstall(w io.Writer, names []string, targets []skills.Target) {
	fmt.Fprintln(w, "solitary's skills teach a coding agent on this host to create a cell, restrict")
	fmt.Fprintln(w, "its network and troubleshoot one:")
	for _, name := range names {
		fmt.Fprintf(w, "  %s\n", name)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "They go into:")
	width := 0
	for _, target := range targets {
		width = max(width, len(tilde(target.Dir)))
	}
	for _, target := range targets {
		readers := "named with --into"
		if len(target.Agents) > 0 {
			readers = "read by " + strings.Join(target.Agents, ", ")
		}
		fmt.Fprintf(w, "  %-*s  %s\n", width, tilde(target.Dir), readers)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Nothing else in those directories is touched.")
	fmt.Fprintln(w)
}

func joinDirs(targets []skills.Target) string {
	dirs := make([]string, len(targets))
	for i, target := range targets {
		dirs[i] = tilde(target.Dir)
	}
	return strings.Join(dirs, ", ")
}

func newSkillsLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List where the skills are installed, and whether they match this binary",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()

			dirs, err := skills.Recorded()
			if err != nil {
				return err
			}
			if len(dirs) == 0 {
				fmt.Fprintln(out, "Not installed. Install them with: solitary skills install")
				return nil
			}

			stale := false
			for _, dir := range dirs {
				fmt.Fprintln(out, tilde(dir))
				status, err := skills.Status(shipped.Host(), dir)
				if err != nil {
					return err
				}
				for _, skill := range status {
					fmt.Fprintf(out, "  %-34s %s\n", skill.Name, skill.State)
					stale = stale || skill.State != skills.Current
				}
			}
			if stale {
				fmt.Fprintln(out, "Bring them in line with this binary with: solitary skills install")
			}

			return nil
		},
	}
}

func newSkillsRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm",
		Short: "Remove solitary's skills from every directory they were installed into",
		Long: "Removes the solitary-* skills from every directory install wrote to and from\n" +
			"the skills directory of every agent found, and nothing else in them.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()

			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("locating home directory: %w", err)
			}
			recorded, err := skills.Recorded()
			if err != nil {
				return err
			}

			// Detected directories as well as recorded ones, so that a
			// record lost with the state directory does not strand a copy
			// that update would then never refresh.
			targets := skills.Detect(home)
			for _, dir := range recorded {
				targets = addTarget(targets, skills.Target{Dir: dir})
			}

			removed := false
			for _, target := range targets {
				names, err := skills.Remove(target.Dir)
				if err != nil {
					return err
				}
				for _, name := range names {
					fmt.Fprintf(out, "Removed %s\n", tilde(filepath.Join(target.Dir, name)))
					removed = true
				}
			}
			if err := skills.Record(nil); err != nil {
				return err
			}
			if !removed {
				fmt.Fprintln(out, "No solitary skills were installed.")
			}

			return nil
		},
	}
}

func addTarget(targets []skills.Target, target skills.Target) []skills.Target {
	if slices.ContainsFunc(targets, func(t skills.Target) bool { return t.Dir == target.Dir }) {
		return targets
	}
	return append(targets, target)
}

// tilde shortens a path under the home directory the way it would be typed.
func tilde(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if rel, err := filepath.Rel(home, path); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		return filepath.Join("~", rel)
	}
	return path
}
