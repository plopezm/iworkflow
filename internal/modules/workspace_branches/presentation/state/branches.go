// Package state holds the view-models of the branch screen. The inbound handlers build them from the use case
// results; the templ files render them and contain no logic.
package state

// BranchRow is one branch of the list.
type BranchRow struct {
	// Value is the canonical reference the form sends ("local:develop", "remote:origin/develop").
	Value string
	// Name is the name the user sees ("develop", "origin/develop").
	Name   string
	Remote bool
	// Protected rows have a disabled checkbox and button; Reason tells why (a protection reason code).
	Protected bool
	Reason    string
	// DeleteHref is the POST that deletes this one branch; the reference rides in the query so the bulk form's
	// checkboxes are not sent with it.
	DeleteHref string
}

// ResultLine is the outcome of one reference of the last delete request.
type ResultLine struct {
	// Name is the branch as the user knows it.
	Name string
	// Outcome is "deleted", "refused" or "failed".
	Outcome string
	// Reason is the protection reason code, for a refused entry.
	Reason string
	// Failure tells a failed entry apart: "invalid", "not_found" or "git". Error carries git's message for "git".
	Failure string
	Error   string
}

// ListView is the list of branches with the filter it shows and the outcome of the last delete request.
type ListView struct {
	// Query is the filter the list applies; the delete form carries it so the refreshed list keeps it.
	Query string
	// DeleteAction is the POST that deletes the checked branches.
	DeleteAction string
	Rows         []BranchRow
	Results      []ResultLine
	// NoneSelected is set when a bulk delete was sent without any checked branch.
	NoneSelected bool
	// Error is the message of a failed listing.
	Error string
}

// ScreenView is the whole branch tab of one workspace.
type ScreenView struct {
	WorkspaceName string
	// BoardHref opens the task board of the workspace.
	BoardHref string
	// SearchHref is the address the filter box queries (the list fragment of the same path).
	SearchHref string
	List       ListView
}
