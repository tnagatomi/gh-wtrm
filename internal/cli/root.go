// Package cli wires the gh-wtrm command-line entry point.
package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/tnagatomi/gh-wtrm/internal/gh"
	"github.com/tnagatomi/gh-wtrm/internal/loader"
	"github.com/tnagatomi/gh-wtrm/internal/tui"
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
	cmd.Flags().String("state", "merged", "PR state that counts as deletable: 'merged' (default) or 'closed' (also accepts merged)")
	return cmd
}

func runTUI(cmd *cobra.Command, args []string) error {
	stateStr, _ := cmd.Flags().GetString("state")
	state, err := parseState(stateStr)
	if err != nil {
		return err
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	repo, err := loader.Load(ctx, wd, state)
	if err != nil {
		return err
	}

	// reload re-queries git and PRs on the `r` key and after a delete.
	reload := func() tui.ReloadResult {
		r, err := loader.Load(ctx, wd, state)
		if err != nil {
			return tui.ReloadResult{FatalErr: err}
		}
		return tui.ReloadResult{Worktrees: r.Worktrees, PRError: r.PRError}
	}

	m := tui.NewModel(repo.Path, repo.Worktrees, repo.PRError).WithReload(reload)
	_, err = tea.NewProgram(m).Run()
	return err
}

// parseState maps the --state flag to a PR scan state. Only 'merged' and
// 'closed' are valid; 'closed' is the opt-in that also accepts merged PRs.
func parseState(s string) (gh.PullRequestState, error) {
	switch strings.ToLower(s) {
	case "merged":
		return gh.Merged, nil
	case "closed":
		return gh.Closed, nil
	default:
		return 0, fmt.Errorf("invalid --state %q: want 'merged' or 'closed'", s)
	}
}
