package state

import (
	"strings"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/jira/infrastructure/adapters/inbound/dto"
)

// IssueDetailView contains the data needed to render the Jira issue detail page.
type IssueDetailView struct {
	Issue              dto.JiraIssueDTO
	Comments           []dto.JiraCommentDTO
	CommentsErr        string
	Transitions        []dto.JiraTransitionDTO
	TransitionsErr     string
	ActionErr          string
	CommentText        string
	SelectedTransition string
}

var msgNoDesc = i18n.Msg{
	En: "(no description)",
	Es: "(sin descripción)",
	Zh: "（无描述）",
	De: "(keine Beschreibung)",
	Fr: "(sans description)",
}

// DescriptionText returns the issue description or the fallback localized "(no description)".
func (v IssueDetailView) DescriptionText(locale i18n.Locale) string {
	if strings.TrimSpace(v.Issue.Description) == "" {
		return msgNoDesc.In(locale)
	}
	return v.Issue.Description
}

// AssigneeText returns the assignee name or "—".
func (v IssueDetailView) AssigneeText() string {
	if v.Issue.Assignee == nil || strings.TrimSpace(*v.Issue.Assignee) == "" {
		return "—"
	}
	return *v.Issue.Assignee
}

// ReporterText returns the reporter name or "—".
func (v IssueDetailView) ReporterText() string {
	if v.Issue.Reporter == nil || strings.TrimSpace(*v.Issue.Reporter) == "" {
		return "—"
	}
	return *v.Issue.Reporter
}

// LabelsText returns comma-separated labels or "—".
func (v IssueDetailView) LabelsText() string {
	if len(v.Issue.Labels) == 0 {
		return "—"
	}
	return strings.Join(v.Issue.Labels, ", ")
}

// ConvertPath returns the URL path to convert this issue into a task.
func (v IssueDetailView) ConvertPath() string {
	return "/jira/" + v.Issue.Key + "/convert"
}

// TaskPath returns the URL path to the converted iworkflow task if present.
func (v IssueDetailView) TaskPath() string {
	if v.Issue.TaskID == nil || *v.Issue.TaskID == "" {
		return ""
	}
	return "/tasks/" + *v.Issue.TaskID
}
