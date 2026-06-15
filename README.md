# gh-wtrm

A [gh](https://cli.github.com/) CLI extension to safely list and remove stale
git worktrees, using **PR merge status** (gh-poi style) to avoid deleting
worktrees whose work has not actually been merged.

> Status: scaffolding. See [HANDOFF.md](HANDOFF.md) for the full design and the
> migration plan from [`tnagatomi/wtclean`](https://github.com/tnagatomi/wtclean).

## Why

`wtclean` inferred "safe to delete" from local heuristics (`git branch --merged`
plus an `upstream-gone` proxy). `upstream-gone` fires whenever a branch's remote
tracking ref is deleted — including PRs closed without merging and manual
deletions — so a clean worktree holding unmerged local commits could be selected
and removed, losing that work.

`gh-wtrm` replaces those heuristics with GitHub's real PR merge state: a worktree
is only safe to remove when its branch has a **merged** PR whose commit set
contains the branch's local HEAD commit. See [HANDOFF.md](HANDOFF.md) §3.

## Install

```
gh extension install tnagatomi/gh-wtrm
```

## Usage

```
gh wtrm
```

Operates on the repository containing the current working directory.
