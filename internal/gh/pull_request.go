package gh

import "errors"

// PullRequestState is the GitHub PR state. The ordering matches gh-poi so
// the safety model's state comparisons port unchanged.
type PullRequestState int

const (
	Closed PullRequestState = iota
	Merged
	Open
)

// PullRequest is the subset of a GitHub pull request gh-wtrm needs to decide
// deletability: its head branch name, state, and the commit OIDs it contains.
type PullRequest struct {
	Name    string
	State   PullRequestState
	IsDraft bool
	Number  int
	Commits []string
	URL     string
	Author  string
}

// ErrUnknownState is returned when GitHub reports a PR state gh-wtrm does not
// recognize. The loader treats this as a fetch failure and falls back to the
// safe (not-deletable) side.
var ErrUnknownState = errors.New("unknown pull request state")

// ToPullRequestState maps a GraphQL state string to a PullRequestState.
func ToPullRequestState(state string) (PullRequestState, error) {
	switch state {
	case "CLOSED":
		return Closed, nil
	case "MERGED":
		return Merged, nil
	case "OPEN":
		return Open, nil
	default:
		return 0, ErrUnknownState
	}
}
