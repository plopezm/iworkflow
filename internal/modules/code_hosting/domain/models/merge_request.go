package models

// NewMergeRequest holds the data needed to open a merge (GitLab) or pull (GitHub) request.
type NewMergeRequest struct {
	SourceBranch string
	TargetBranch string
	Title        string
	Description  string
}

// MergeRequest is the provider-neutral view of a merge or pull request.
type MergeRequest struct {
	IID   uint64
	Title string
	// State is `opened`, `merged` or `closed` (`locked` on GitLab).
	State        string
	SourceBranch string
	TargetBranch string
	WebURL       string
	// MergeStatus is the neutral merge state: `mergeable`, `conflict`, `checking`...
	MergeStatus  string
	HasConflicts bool
}

// IsOpen reports whether the request still accepts changes.
func (m MergeRequest) IsOpen() bool {
	return m.State == "opened"
}

// PipelineJob is one CI job of a pipeline.
type PipelineJob struct {
	ID     uint64
	Name   string
	Stage  string
	Status string
	WebURL string
}

// Pipeline is the CI run of a merge request, with its jobs. The status uses the neutral vocabulary
// (`pending`, `running`, `success`, `failed`, `canceled`, `skipped`, `manual`).
type Pipeline struct {
	ID     uint64
	Status string
	WebURL string
	Jobs   []PipelineJob
}

// MergeRequestStatus aggregates a merge request with the latest pipeline of its source branch.
type MergeRequestStatus struct {
	// Provider decides the vocabulary shown to the user.
	Provider    HostingProvider
	ProjectPath string
	// MergeRequest is the request itself.
	MergeRequest MergeRequest
	// Pipeline is nil when the request has none.
	Pipeline *Pipeline
}
