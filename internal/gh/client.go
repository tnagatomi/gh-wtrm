package gh

import (
	"context"
	"fmt"

	"github.com/cli/go-gh/v2/pkg/api"
)

// searchResponse mirrors the `data` payload of the PR search query. go-gh's
// Do strips the outer `data` envelope and unmarshals into this struct.
type (
	searchResponse struct {
		Search struct {
			IssueCount int
			Edges      []searchEdge
		}
	}

	searchEdge struct {
		Node struct {
			Number      int
			HeadRefName string
			URL         string `json:"url"`
			State       string
			IsDraft     bool
			Commits     struct {
				Nodes []commitNode
			}
			Author struct {
				Login string
			}
		}
	}

	commitNode struct {
		Commit struct {
			Oid string
		}
	}
)

// prSearchQuery finds pull requests whose commit set contains any of the
// queried hashes, scoped to the given repositories. Ported from gh-poi
// (the org qualifier is dropped — gh-wtrm is single-repository).
const prSearchQuery = `query($q: String!) {
  search(type: ISSUE, query: $q, last: 100) {
    issueCount
    edges {
      node {
        ... on PullRequest {
          number
          url
          state
          isDraft
          headRefName
          commits(last: 100) {
            nodes {
              commit {
                oid
              }
            }
          }
          author { login }
        }
      }
    }
  }
}`

// Client fetches pull requests from a GitHub host via the gh CLI's
// authentication and GraphQL endpoint.
type Client struct {
	gql *api.GraphQLClient
}

// NewClient builds a GraphQL client for host, reusing gh's stored
// credentials and host resolution. An empty host uses gh's default host.
func NewClient(host string) (*Client, error) {
	gql, err := api.NewGraphQLClient(api.ClientOptions{Host: host})
	if err != nil {
		return nil, err
	}
	return &Client{gql: gql}, nil
}

// SearchPullRequests runs the PR search for one batch of hash qualifiers
// within the given repo qualifiers and returns the parsed pull requests.
func (c *Client) SearchPullRequests(ctx context.Context, repos, queryHashes string) ([]PullRequest, error) {
	q := fmt.Sprintf("is:pr %s %s", repos, queryHashes)
	var resp searchResponse
	if err := c.gql.DoWithContext(ctx, prSearchQuery, map[string]interface{}{"q": q}, &resp); err != nil {
		return nil, err
	}
	return toPullRequests(resp)
}

// toPullRequests converts a search response into PullRequests, mapping each
// node's state and flattening its commit OIDs. An unrecognized state is a
// hard error so the caller can fall back to the safe side. Ported from
// gh-poi's toPullRequests.
func toPullRequests(resp searchResponse) ([]PullRequest, error) {
	results := []PullRequest{}
	for _, edge := range resp.Search.Edges {
		state, err := ToPullRequestState(edge.Node.State)
		if err != nil {
			return nil, fmt.Errorf("unexpected pull request state: %s", edge.Node.State)
		}

		commits := []string{}
		for _, node := range edge.Node.Commits.Nodes {
			commits = append(commits, node.Commit.Oid)
		}

		results = append(results, PullRequest{
			Name:    edge.Node.HeadRefName,
			State:   state,
			IsDraft: edge.Node.IsDraft,
			Number:  edge.Node.Number,
			Commits: commits,
			URL:     edge.Node.URL,
			Author:  edge.Node.Author.Login,
		})
	}
	return results, nil
}
