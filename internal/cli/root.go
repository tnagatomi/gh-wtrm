// Package cli wires the gh-wtrm command-line entry point.
package cli

import (
	"github.com/spf13/cobra"
)

// Version is overridden at build time via -ldflags.
var Version = "dev"

// NewRootCmd builds the `gh wtrm` command. gh-wtrm operates on the single
// repository containing the current working directory; there are no
// subcommands and no config.
func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "wtrm",
		Short:         "Safely list and remove stale git worktrees using PR merge state",
		Long:          "gh-wtrm is a gh CLI extension that lists and removes git worktrees in the current repository, using GitHub PR merge status to avoid deleting unmerged work.",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          runTUI,
	}
	return cmd
}

// runTUI is the root action. Wiring to the loader and TUI lands in a later
// chunk; for now it is a placeholder so the command is runnable.
func runTUI(cmd *cobra.Command, args []string) error {
	return nil
}
