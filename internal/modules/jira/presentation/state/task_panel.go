package state

import (
	"iworkflow/internal/modules/jira/infrastructure/adapters/inbound/dto"
)

// TaskPanelView is everything the Jira panel of a task shows. The handler builds it from the use cases; the template
// only renders it. The form fields keep what the user typed when an action fails, so nothing is lost.
type TaskPanelView struct {
	TaskID string

	// Issue is the issue linked to the task; nil when the task has none.
	Issue          *dto.IssueDTO
	Comments       []dto.CommentDTO
	CommentsErr    string
	Transitions    []dto.TransitionDTO
	TransitionsErr string
	// SelectedTransition is the transition the user chose when it failed, so the select keeps it.
	SelectedTransition string
	// LinkKey is the key the user typed when linking failed.
	LinkKey string
	// ActionErr is the outcome of the last action when it failed; shown inline in the panel.
	ActionErr string

	// Finished is true when the task has ended its work; the worklog is then offered as a banner.
	Finished bool
	// Pending is true when there is time not logged yet. Nothing pending disables the register button.
	Pending bool
	// ProposedTime is the proposed time in Jira format (`1h 30m`); empty when nothing is pending.
	ProposedTime string
	// TimeText and Description are the worklog form values; AlsoComment is its optional checkbox.
	TimeText    string
	Description string
	AlsoComment bool
	// Logged is the total logged for the task in Jira format; empty when nothing was logged yet.
	Logged   string
	Worklogs []dto.WorklogDTO
	// WorklogErr is a failure to read the worklog data; the form is still shown.
	WorklogErr string
}

// RootID is the element the panel renders into. Every action answers with it, swapped as a whole.
func (v TaskPanelView) RootID() string {
	return "jira-task-panel-" + v.TaskID
}

// Path is the address of one action of the panel of this task.
func (v TaskPanelView) Path(action string) string {
	return "/jira/tasks/" + v.TaskID + "/" + action
}

// Linked tells whether the task has an issue. The worklog form and the refresh need one.
func (v TaskPanelView) Linked() bool {
	return v.Issue != nil
}

// ShowBanner tells whether the finished-task banner offers a worklog: the task ended and time is pending.
func (v TaskPanelView) ShowBanner() bool {
	return v.Linked() && v.Finished && v.Pending
}

// HasLogged tells whether any worklog was registered for the task.
func (v TaskPanelView) HasLogged() bool {
	return v.Logged != ""
}

// latestCommentCount bounds the comments the panel lists: the panel is a summary, the page has the rest.
const latestCommentCount = 3

// LatestComments returns the most recent comments of the issue, oldest first, at most latestCommentCount of them.
func (v TaskPanelView) LatestComments() []dto.CommentDTO {
	if len(v.Comments) <= latestCommentCount {
		return v.Comments
	}
	return v.Comments[len(v.Comments)-latestCommentCount:]
}
