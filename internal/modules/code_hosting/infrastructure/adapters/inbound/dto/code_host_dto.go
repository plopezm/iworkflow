// Package dto holds the shapes that cross the boundary between the code hosting adapters and the
// presentation. The token never travels to the browser: settings only report that one is configured.
//
//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package dto

import (
	"fmt"

	"iworkflow/internal/modules/code_hosting/application"
	"iworkflow/internal/modules/code_hosting/domain/models"
)

// CodeHostSettingsDTO is the settings of a provider as the UI sees them.
type CodeHostSettingsDTO struct {
	// Provider is the identifier (`gitlab` or `github`).
	Provider        string `json:"provider"`
	BaseURL         string `json:"base_url"`
	TokenConfigured bool   `json:"token_configured"`
	InsecureTLS     bool   `json:"insecure_tls"`
}

// SettingsFrom masks the token: only the fact that one is stored leaves the module.
func SettingsFrom(settings models.CodeHostSettings) CodeHostSettingsDTO {
	return CodeHostSettingsDTO{
		Provider:        string(settings.Provider()),
		BaseURL:         settings.BaseURL(),
		TokenConfigured: true,
		InsecureTLS:     settings.InsecureTLS,
	}
}

// UnconfiguredSettings is the empty form of a provider that has never been configured.
func UnconfiguredSettings(provider models.HostingProvider) CodeHostSettingsDTO {
	return CodeHostSettingsDTO{Provider: string(provider)}
}

// SaveCodeHostSettingsDTO is the form submitted by the user. A nil or blank Token keeps the stored one.
type SaveCodeHostSettingsDTO struct {
	Provider    string  `json:"provider"`
	BaseURL     string  `json:"base_url"`
	Token       *string `json:"token"`
	InsecureTLS bool    `json:"insecure_tls"`
}

// String masks the token, as the Rust version did, so the form can be logged safely.
func (d SaveCodeHostSettingsDTO) String() string {
	masked := "<nil>"
	if d.Token != nil {
		masked = "***"
	}
	return fmt.Sprintf("SaveCodeHostSettingsDTO{provider: %s, base_url: %s, token: %s, insecure_tls: %t}",
		d.Provider, d.BaseURL, masked, d.InsecureTLS)
}

// GoString is the %#v form, masked the same way.
func (d SaveCodeHostSettingsDTO) GoString() string { return d.String() }

// ToInput parses the provider identifier and builds the use case input.
func (d SaveCodeHostSettingsDTO) ToInput() (application.SaveCodeHostSettingsInput, error) {
	provider, err := models.ParseHostingProvider(d.Provider)
	if err != nil {
		return application.SaveCodeHostSettingsInput{}, err
	}
	token := ""
	if d.Token != nil {
		token = *d.Token
	}
	return application.SaveCodeHostSettingsInput{
		Provider:    provider,
		BaseURL:     d.BaseURL,
		Token:       token,
		InsecureTLS: d.InsecureTLS,
	}, nil
}

// CreateMergeRequestDTO asks for a merge request for a task. A nil or blank title or description uses
// the one of the task.
type CreateMergeRequestDTO struct {
	TaskID      string  `json:"task_id"`
	Title       *string `json:"title"`
	Description *string `json:"description"`
}

// ToInput builds the use case input.
func (d CreateMergeRequestDTO) ToInput() application.CreateTaskMergeRequestInput {
	return application.CreateTaskMergeRequestInput{
		TaskID:      d.TaskID,
		Title:       stringOrEmpty(d.Title),
		Description: stringOrEmpty(d.Description),
	}
}

// MergeRequestDTO is a merge (GitLab) or pull (GitHub) request as the UI sees it.
type MergeRequestDTO struct {
	IID          uint64 `json:"iid"`
	Title        string `json:"title"`
	State        string `json:"state"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
	WebURL       string `json:"web_url"`
	MergeStatus  string `json:"merge_status"`
	HasConflicts bool   `json:"has_conflicts"`
}

// MergeRequestFrom maps the merge request of the domain.
func MergeRequestFrom(mergeRequest models.MergeRequest) MergeRequestDTO {
	return MergeRequestDTO{
		IID:          mergeRequest.IID,
		Title:        mergeRequest.Title,
		State:        mergeRequest.State,
		SourceBranch: mergeRequest.SourceBranch,
		TargetBranch: mergeRequest.TargetBranch,
		WebURL:       mergeRequest.WebURL,
		MergeStatus:  mergeRequest.MergeStatus,
		HasConflicts: mergeRequest.HasConflicts,
	}
}

// IsOpen reports whether the request still accepts changes.
func (d MergeRequestDTO) IsOpen() bool {
	return d.State == "opened"
}

// PipelineJobDTO is one CI job.
type PipelineJobDTO struct {
	ID     uint64 `json:"id"`
	Name   string `json:"name"`
	Stage  string `json:"stage"`
	Status string `json:"status"`
	WebURL string `json:"web_url"`
}

// PipelineDTO is the latest CI run of a merge request, with its jobs.
type PipelineDTO struct {
	ID     uint64           `json:"id"`
	Status string           `json:"status"`
	WebURL string           `json:"web_url"`
	Jobs   []PipelineJobDTO `json:"jobs"`
}

// MergeRequestStatusDTO joins a merge request with its latest pipeline.
type MergeRequestStatusDTO struct {
	// Provider is the identifier (`gitlab` or `github`), which decides the vocabulary of the UI.
	Provider     string          `json:"provider"`
	ProjectPath  string          `json:"project_path"`
	MergeRequest MergeRequestDTO `json:"merge_request"`
	// Pipeline is nil when the request has none.
	Pipeline *PipelineDTO `json:"pipeline"`
}

// MergeRequestStatusFrom maps the status of the domain, pipeline and jobs included.
func MergeRequestStatusFrom(status models.MergeRequestStatus) MergeRequestStatusDTO {
	dto := MergeRequestStatusDTO{
		Provider:     string(status.Provider),
		ProjectPath:  status.ProjectPath,
		MergeRequest: MergeRequestFrom(status.MergeRequest),
	}
	if status.Pipeline != nil {
		pipeline := PipelineDTO{
			ID:     status.Pipeline.ID,
			Status: status.Pipeline.Status,
			WebURL: status.Pipeline.WebURL,
			Jobs:   make([]PipelineJobDTO, 0, len(status.Pipeline.Jobs)),
		}
		for _, job := range status.Pipeline.Jobs {
			pipeline.Jobs = append(pipeline.Jobs, PipelineJobDTO{
				ID:     job.ID,
				Name:   job.Name,
				Stage:  job.Stage,
				Status: job.Status,
				WebURL: job.WebURL,
			})
		}
		dto.Pipeline = &pipeline
	}
	return dto
}

func stringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
