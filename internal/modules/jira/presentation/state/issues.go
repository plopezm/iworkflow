package state

import (
	"strings"

	"iworkflow/internal/modules/jira/infrastructure/adapters/inbound/dto"
)

// LaneFormMode describes whether the lane form is closed, creating, or editing.
type LaneFormMode int

const (
	// LaneFormClosed means the form is not displayed.
	LaneFormClosed LaneFormMode = iota
	// LaneFormCreating means the form is creating a new lane.
	LaneFormCreating
	// LaneFormEditing means the form is editing an existing lane.
	LaneFormEditing
)

// CanSaveLane returns whether the lane name and JQL query are non-empty.
func CanSaveLane(name, jql string) bool {
	return strings.TrimSpace(name) != "" && strings.TrimSpace(jql) != ""
}

// BuildFilter creates an IssueFilterDTO from raw input strings, trimming and converting empty values to nil.
func BuildFilter(search, status, localState string) dto.IssueFilterDTO {
	var statusPtr *string
	trimmedStatus := strings.TrimSpace(status)
	if trimmedStatus != "" {
		statusPtr = &trimmedStatus
	}

	var localPtr *string
	trimmedLocal := strings.TrimSpace(localState)
	if trimmedLocal != "" {
		localPtr = &trimmedLocal
	}

	return dto.IssueFilterDTO{
		Search:     strings.TrimSpace(search),
		Status:     statusPtr,
		LocalState: localPtr,
	}
}

// FilterIssues applies search, Jira status, and local state filters to a slice of issues.
func FilterIssues(issues []dto.IssueDTO, filter dto.IssueFilterDTO) []dto.IssueDTO {
	searchLower := strings.ToLower(strings.TrimSpace(filter.Search))
	var statusFilter string
	if filter.Status != nil {
		statusFilter = strings.ToLower(strings.TrimSpace(*filter.Status))
	}
	var localFilter string
	if filter.LocalState != nil {
		localFilter = strings.ToLower(strings.TrimSpace(*filter.LocalState))
	}

	filtered := make([]dto.IssueDTO, 0, len(issues))
	for _, issue := range issues {
		// 1. Search filter: key, summary, description, assignee, labels
		if searchLower != "" {
			matches := strings.Contains(strings.ToLower(issue.Key), searchLower) ||
				strings.Contains(strings.ToLower(issue.Summary), searchLower) ||
				strings.Contains(strings.ToLower(issue.Description), searchLower)
			if !matches && issue.Assignee != nil {
				matches = strings.Contains(strings.ToLower(*issue.Assignee), searchLower)
			}
			if !matches {
				for _, label := range issue.Labels {
					if strings.Contains(strings.ToLower(label), searchLower) {
						matches = true
						break
					}
				}
			}
			if !matches {
				continue
			}
		}

		// 2. Status filter
		if statusFilter != "" {
			if strings.ToLower(strings.TrimSpace(issue.Status)) != statusFilter {
				continue
			}
		}

		// 3. Local state filter ("new", "converted", "ignored")
		if localFilter != "" {
			if strings.ToLower(strings.TrimSpace(issue.LocalState)) != localFilter {
				continue
			}
		}

		filtered = append(filtered, issue)
	}

	return filtered
}

// LaneFormView holds the presentation state for the lane form.
type LaneFormView struct {
	Mode    LaneFormMode
	LaneID  string
	Name    string
	JQL     string
	Err     string
	Pending bool
}

// LaneView represents a lane and its filtered issues ready to be displayed.
type LaneView struct {
	Lane             dto.LaneDTO
	Issues           []dto.IssueDTO
	Total            int
	ConfirmingDelete bool
}

// IssuesPanelView holds all state needed to render the imported issues panel.
type IssuesPanelView struct {
	Syncing     bool
	SyncMessage *string
	FailedLanes []string
	LaneNotice  *string
	Filters     dto.IssueFilterDTO
	Statuses    []string
	Lanes       []LaneView
	LaneForm    LaneFormView
}
