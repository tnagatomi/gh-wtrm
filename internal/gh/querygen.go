package gh

import (
	"fmt"
	"strings"
)

// GetQueryRepos renders the `repo:<owner>/<name>` qualifiers for a GitHub
// search query. gh-wtrm targets a single repository (plus its parent when a
// fork), so the caller passes the resolved repo names.
func GetQueryRepos(repoNames []string) string {
	var b strings.Builder
	for _, name := range repoNames {
		fmt.Fprintf(&b, "repo:%s ", name)
	}
	return strings.TrimSpace(b.String())
}

// GetQueryHashes batches commit OIDs into `hash:<oid>` qualifier strings,
// each kept under GitHub's 256-character search-query limit. Ported from
// gh-poi; the input is the per-worktree local HEAD OIDs to search PRs for.
//
// https://docs.github.com/en/rest/reference/search#limitations-on-query-length
func GetQueryHashes(oids []string) []string {
	results := []string{}

	var hashes strings.Builder
	for i, oid := range oids {
		separator := " "
		if i == len(oids)-1 {
			separator = ""
		}
		hash := fmt.Sprintf("hash:%s%s", oid, separator)

		if len(hashes.String())+len(hash) > 256 {
			results = append(results, hashes.String())
			hashes.Reset()
		}

		hashes.WriteString(hash)
	}
	if len(hashes.String()) > 0 {
		results = append(results, hashes.String())
	}

	return results
}
