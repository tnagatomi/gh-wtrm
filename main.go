// Command gh-wtrm is a gh CLI extension that safely lists and removes stale
// git worktrees in the current repository, using GitHub PR merge state to
// avoid deleting worktrees whose work has not actually been merged.
package main

import (
	"fmt"
	"os"

	"github.com/tnagatomi/gh-wtrm/internal/cli"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "gh-wtrm:", err)
		os.Exit(1)
	}
}
