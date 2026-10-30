package models

// IssueLocalState records what iworkflow has done with an imported issue; it is independent from the
// Jira status.
type IssueLocalState string

const (
	IssueLocalStateNew       IssueLocalState = "new"
	IssueLocalStateConverted IssueLocalState = "converted"
	IssueLocalStateIgnored   IssueLocalState = "ignored"
)

// ParseIssueLocalState maps a stored value back to its state. Unknown values fall back to New so a
// hand-edited row never hides an issue.
func ParseIssueLocalState(value string) IssueLocalState {
	switch IssueLocalState(value) {
	case IssueLocalStateConverted, IssueLocalStateIgnored:
		return IssueLocalState(value)
	default:
		return IssueLocalStateNew
	}
}

// String returns the stored form of the state.
func (s IssueLocalState) String() string {
	return string(s)
}
