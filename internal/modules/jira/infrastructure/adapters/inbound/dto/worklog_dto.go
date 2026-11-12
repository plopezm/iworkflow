package dto

import (
	"time"

	"iworkflow/internal/modules/jira/domain/models"
)

// WorklogDTO is a worklog already sent to Jira for a task, as the Jira panel lists it.
type WorklogDTO struct {
	JiraID   string `json:"jira_id"`
	Time     string `json:"time"`
	Comment  string `json:"comment"`
	LoggedAt string `json:"logged_at"`
}

// WorklogsFromDomain maps the stored worklogs of a task, keeping their order.
func WorklogsFromDomain(records []models.WorklogRecord) []WorklogDTO {
	out := make([]WorklogDTO, 0, len(records))
	for _, record := range records {
		out = append(out, WorklogDTO{
			JiraID:   record.JiraID,
			Time:     models.FormatJiraDuration(time.Duration(record.Seconds) * time.Second),
			Comment:  record.Comment,
			LoggedAt: record.LoggedAt.Local().Format("2006-01-02 15:04"),
		})
	}
	return out
}
