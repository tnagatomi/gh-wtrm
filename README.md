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

Operates on the repository containing the current working directory. It opens
an interactive TUI listing that repo's worktrees with PR-derived badges
(`[pr-merged]`, `[pr-open]`, `[pr-closed]`, `[uncommitted]`, `[locked]`,
`[no-dir]`, `[primary]`).

Keys: `space` select, `s` select all safe-to-remove, `/` filter, `d` delete
(with confirmation), `y` copy branch name, `r` reload, `?` help, `q`/`esc`
quit.

A worktree is "safe to remove" only when its branch has a **merged** PR whose
commit set contains the branch's local HEAD commit (plus no uncommitted work,
not locked, and not the primary or current worktree). A directory that is
already gone is treated as a prune target.

### Flags

```
--state merged   # default: only merged PRs count as deletable
--state closed   # opt-in: also accept PRs closed without merging (and merged)
```
