//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package models

import (
	"fmt"
	"strings"
	"unicode/utf8"

	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
)

// maxTitleChars bounds the title of a converted task, in characters.
const maxTitleChars = 120

// emptyDescription replaces a blank Jira description in the suggested prompt.
const emptyDescription = "(sin descripción)"

// ConversionDraft is the suggested content of the iworkflow task, shown for editing before it is created.
type ConversionDraft struct {
	Title    string
	Prompt   string
	Priority PriorityHint
}

// DraftFromIssue builds the suggested task content of an issue.
func DraftFromIssue(issue Issue) ConversionDraft {
	remote := issue.Remote
	title := truncateRunes(fmt.Sprintf("%s: %s", remote.Key, remote.Summary), maxTitleChars)
	description := strings.TrimSpace(remote.Description)
	if description == "" {
		description = emptyDescription
	}
	priority := "sin prioridad"
	if remote.Priority != nil {
		priority = *remote.Priority
	}
	prompt := fmt.Sprintf(
		"Jira %s: %s\n\nTipo: %s · Estado: %s · Prioridad: %s\nEnlace: %s\n\n## Descripción\n\n%s\n",
		remote.Key, remote.Summary, remote.IssueType, remote.Status, priority, remote.URL, description,
	)
	return ConversionDraft{
		Title:    title,
		Prompt:   prompt,
		Priority: PriorityHintFromJira(derefOrEmpty(remote.Priority)),
	}
}

// ConversionRequest is the user-confirmed mapping of an issue onto a new iworkflow task.
type ConversionRequest struct {
	Key         string
	WorkspaceID string
	BaseBranch  *string
	Title       string
	Prompt      string
	Priority    PriorityHint
	StartNow    bool
}

// Validated trims the user input and checks the required fields. The title is cut to the maximum
// length and a blank base branch becomes nil.
func (r ConversionRequest) Validated() (ConversionRequest, error) {
	workspaceID := strings.TrimSpace(r.WorkspaceID)
	if workspaceID == "" {
		return ConversionRequest{}, jiraerrors.MissingWorkspace()
	}
	title := strings.TrimSpace(r.Title)
	if title == "" {
		return ConversionRequest{}, jiraerrors.EmptyTitle()
	}
	prompt := strings.TrimSpace(r.Prompt)
	if prompt == "" {
		return ConversionRequest{}, jiraerrors.EmptyPrompt()
	}
	var baseBranch *string
	if r.BaseBranch != nil {
		trimmed := strings.TrimSpace(*r.BaseBranch)
		if trimmed != "" {
			baseBranch = &trimmed
		}
	}
	return ConversionRequest{
		Key:         r.Key,
		WorkspaceID: workspaceID,
		BaseBranch:  baseBranch,
		Title:       truncateRunes(title, maxTitleChars),
		Prompt:      prompt,
		Priority:    r.Priority,
		StartNow:    r.StartNow,
	}, nil
}

// truncateRunes keeps the first limit characters (runes), never splitting a multi-byte character.
func truncateRunes(value string, limit int) string {
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit])
}

func derefOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
