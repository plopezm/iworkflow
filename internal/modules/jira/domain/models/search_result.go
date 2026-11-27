package models

// SearchResult is the answer of one JQL search.
type SearchResult struct {
	Issues []RemoteIssue
	// Truncated is true when Jira has more issues than the import limit let through.
	Truncated bool
}
