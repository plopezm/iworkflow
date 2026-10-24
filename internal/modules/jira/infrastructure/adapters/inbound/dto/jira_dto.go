// Package dto holds the data shapes that cross the HTTP boundary of the jira module. Domain models
// never reach the browser: every answer is mapped to one of these structs first.
package dto

import (
	"strings"
	"time"

	"iworkflow/internal/modules/jira/application"
	"iworkflow/internal/modules/jira/domain/models"
)

// ConnectionDTO is the connection as the browser sees it. The token is never part of it.
type ConnectionDTO struct {
	BaseURL            string  `json:"base_url"`
	AcceptInvalidCerts bool    `json:"accept_invalid_certs"`
	LastSyncAt         *string `json:"last_sync_at"`
	LastSyncError      *string `json:"last_sync_error"`
}

// SaveConnectionDTO is the payload of the connection form. A blank token keeps the stored one.
type SaveConnectionDTO struct {
	BaseURL            string `json:"base_url"`
	Token              string `json:"token"`
	AcceptInvalidCerts bool   `json:"accept_invalid_certs"`
}

// IssueFilterDTO is the filter of the issue list. An empty status or state does not filter.
type IssueFilterDTO struct {
	Search     string  `json:"search"`
	Status     *string `json:"status"`
	LocalState *string `json:"local_state"`
}

// IssueDTO is one imported issue.
type IssueDTO struct {
	Key           string   `json:"key"`
	Summary       string   `json:"summary"`
	Description   string   `json:"description"`
	Status        string   `json:"status"`
	IssueType     string   `json:"issue_type"`
	Priority      *string  `json:"priority"`
	Assignee      *string  `json:"assignee"`
	Reporter      *string  `json:"reporter"`
	Labels        []string `json:"labels"`
	URL           string   `json:"url"`
	JiraUpdatedAt string   `json:"jira_updated_at"`
	LocalState    string   `json:"local_state"`
	TaskID        *string  `json:"task_id"`
}

// LaneDTO is one lane of the Jira page.
type LaneDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	JQL       string `json:"jql"`
	IsDefault bool   `json:"is_default"`
	Collapsed bool   `json:"collapsed"`
}

// LaneListingDTO is a lane with its filtered issues and their unfiltered count.
type LaneListingDTO struct {
	Lane   LaneDTO    `json:"lane"`
	Issues []IssueDTO `json:"issues"`
	Total  int        `json:"total"`
}

// IssueListDTO is the whole Jira page.
type IssueListDTO struct {
	Lanes    []LaneListingDTO `json:"lanes"`
	Statuses []string         `json:"statuses"`
	Total    int              `json:"total"`
}

// SavedLaneDTO is a stored lane with the failure of its first sync, if any.
type SavedLaneDTO struct {
	Lane      LaneDTO `json:"lane"`
	SyncError *string `json:"sync_error"`
}

// SyncOutcomeDTO summarises a sync.
type SyncOutcomeDTO struct {
	Total       int      `json:"total"`
	Added       int      `json:"added"`
	FailedLanes []string `json:"failed_lanes"`
}

// CommentDTO is one comment of an issue.
type CommentDTO struct {
	ID        string `json:"id"`
	Author    string `json:"author"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

// TransitionDTO is one status change Jira allows.
type TransitionDTO struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ToStatus string `json:"to_status"`
}

// WorkspaceDTO is a workspace a conversion can target.
type WorkspaceDTO struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	BaseBranch string `json:"base_branch"`
}

// BranchDTO is a branch a converted task can start from.
type BranchDTO struct {
	Name   string `json:"name"`
	Remote bool   `json:"remote"`
}

// ConversionPreparationDTO is what the conversion screen shows before the user confirms.
type ConversionPreparationDTO struct {
	Issue      IssueDTO       `json:"issue"`
	Title      string         `json:"title"`
	Prompt     string         `json:"prompt"`
	Priority   string         `json:"priority"`
	Workspaces []WorkspaceDTO `json:"workspaces"`
}

// ConvertIssueDTO is the user-confirmed mapping of an issue onto a task.
type ConvertIssueDTO struct {
	Key         string  `json:"key"`
	WorkspaceID string  `json:"workspace_id"`
	BaseBranch  *string `json:"base_branch"`
	Title       string  `json:"title"`
	Prompt      string  `json:"prompt"`
	Priority    string  `json:"priority"`
	StartNow    bool    `json:"start_now"`
}

// ConvertedIssueDTO is the created task and the non-fatal problems around it.
type ConvertedIssueDTO struct {
	TaskID     string  `json:"task_id"`
	StartError *string `json:"start_error"`
}

// ConnectionFromDomain maps the stored connection. The token is dropped here, on purpose.
func ConnectionFromDomain(connection models.Connection) ConnectionDTO {
	return ConnectionDTO{
		BaseURL:            connection.BaseURL(),
		AcceptInvalidCerts: connection.AcceptInvalidCerts(),
		LastSyncAt:         formatOptionalTime(connection.LastSyncAt()),
		LastSyncError:      connection.LastSyncError(),
	}
}

// ToSaveInput maps the connection form to the use case input.
func (d SaveConnectionDTO) ToSaveInput() application.SaveConnectionInput {
	return application.SaveConnectionInput{
		BaseURL:            d.BaseURL,
		Token:              d.Token,
		AcceptInvalidCerts: d.AcceptInvalidCerts,
	}
}

// ToDomain maps the filter form to the domain filter. A blank status is no filter.
func (d IssueFilterDTO) ToDomain() models.IssueFilter {
	filter := models.IssueFilter{Search: d.Search}
	if d.Status != nil && strings.TrimSpace(*d.Status) != "" {
		status := *d.Status
		filter.Status = &status
	}
	if d.LocalState != nil {
		state := parseLocalStateStrict(*d.LocalState)
		if state != nil {
			filter.LocalState = state
		}
	}
	return filter
}

// IssueFromDomain maps one issue to its DTO.
func IssueFromDomain(issue models.Issue) IssueDTO {
	remote := issue.Remote
	labels := remote.Labels
	if labels == nil {
		labels = []string{}
	}
	return IssueDTO{
		Key:           remote.Key,
		Summary:       remote.Summary,
		Description:   remote.Description,
		Status:        remote.Status,
		IssueType:     remote.IssueType,
		Priority:      remote.Priority,
		Assignee:      remote.Assignee,
		Reporter:      remote.Reporter,
		Labels:        labels,
		URL:           remote.URL,
		JiraUpdatedAt: remote.JiraUpdatedAt.Format(time.RFC3339),
		LocalState:    issue.LocalState.String(),
		TaskID:        issue.TaskID,
	}
}

// LaneFromDomain maps a lane to its DTO.
func LaneFromDomain(lane models.Lane) LaneDTO {
	return LaneDTO{ID: lane.ID, Name: lane.Name, JQL: lane.JQL, IsDefault: lane.IsDefault, Collapsed: lane.Collapsed}
}

// ListingFromDomain maps the Jira page to its DTO.
func ListingFromDomain(listing application.IssueListing) IssueListDTO {
	lanes := make([]LaneListingDTO, 0, len(listing.Lanes))
	for _, lane := range listing.Lanes {
		issues := make([]IssueDTO, 0, len(lane.Issues))
		for _, issue := range lane.Issues {
			issues = append(issues, IssueFromDomain(issue))
		}
		lanes = append(lanes, LaneListingDTO{Lane: LaneFromDomain(lane.Lane), Issues: issues, Total: lane.Total})
	}
	return IssueListDTO{Lanes: lanes, Statuses: listing.Statuses, Total: listing.Total}
}

// SavedLaneFromDomain maps a stored lane and its first sync outcome.
func SavedLaneFromDomain(saved application.SavedLane) SavedLaneDTO {
	return SavedLaneDTO{Lane: LaneFromDomain(saved.Lane), SyncError: saved.SyncError}
}

// SyncOutcomeFromDomain maps the outcome of a sync.
func SyncOutcomeFromDomain(outcome application.SyncOutcome) SyncOutcomeDTO {
	failed := outcome.FailedLanes
	if failed == nil {
		failed = []string{}
	}
	return SyncOutcomeDTO{Total: outcome.Total, Added: outcome.Added, FailedLanes: failed}
}

// CommentFromDomain maps a comment.
func CommentFromDomain(comment models.Comment) CommentDTO {
	return CommentDTO{ID: comment.ID, Author: comment.Author, Body: comment.Body, CreatedAt: comment.CreatedAt.Format(time.RFC3339)}
}

// TransitionFromDomain maps a transition.
func TransitionFromDomain(transition models.Transition) TransitionDTO {
	return TransitionDTO{ID: transition.ID, Name: transition.Name, ToStatus: transition.ToStatus}
}

// PreparationFromDomain maps the conversion screen data.
func PreparationFromDomain(prepared application.ConversionPreparation) ConversionPreparationDTO {
	workspaces := make([]WorkspaceDTO, 0, len(prepared.Workspaces))
	for _, workspace := range prepared.Workspaces {
		workspaces = append(workspaces, WorkspaceDTO{ID: workspace.ID, Name: workspace.Name, BaseBranch: workspace.BaseBranch})
	}
	return ConversionPreparationDTO{
		Issue:      IssueFromDomain(prepared.Issue),
		Title:      prepared.Draft.Title,
		Prompt:     prepared.Draft.Prompt,
		Priority:   prepared.Draft.Priority.String(),
		Workspaces: workspaces,
	}
}

// ToConversionRequest maps the confirmed mapping. An unknown priority falls back to normal.
func (d ConvertIssueDTO) ToConversionRequest() models.ConversionRequest {
	return models.ConversionRequest{
		Key:         d.Key,
		WorkspaceID: d.WorkspaceID,
		BaseBranch:  d.BaseBranch,
		Title:       d.Title,
		Prompt:      d.Prompt,
		Priority:    models.ParsePriorityHint(d.Priority),
		StartNow:    d.StartNow,
	}
}

// ConvertedFromDomain maps the conversion result.
func ConvertedFromDomain(converted application.ConvertedIssue) ConvertedIssueDTO {
	return ConvertedIssueDTO{TaskID: converted.TaskID, StartError: converted.StartError}
}

// parseLocalStateStrict accepts only the three stored states; anything else means no filter.
func parseLocalStateStrict(value string) *models.IssueLocalState {
	for _, state := range []models.IssueLocalState{
		models.IssueLocalStateNew,
		models.IssueLocalStateConverted,
		models.IssueLocalStateIgnored,
	} {
		if state.String() == value {
			return &state
		}
	}
	return nil
}

func formatOptionalTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format(time.RFC3339)
	return &formatted
}
