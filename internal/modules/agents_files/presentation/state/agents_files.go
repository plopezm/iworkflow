// Package state holds the view-models of the AGENTS.md screen. Handlers build them from the use
// cases; the templ components only render them. They carry data and texts, and no business rules.
package state

import (
	"context"
	"time"

	"iworkflow/internal/modules/agents_files/infrastructure/adapters/inbound/dto"
)

// serverFnErrorPrefix is the text the Rust (Leptos) UI put in front of every server error. It is
// kept so the messages stay byte-identical with the Rust version.
const serverFnErrorPrefix = "error running server function: "

// createdAtLayout is the date shown on each file: day/month/year, 24h time, and the literal "UTC".
const createdAtLayout = "02/01/2006 15:04 UTC"

// FeedbackKind is the outcome of the last "add to project" of one file.
type FeedbackKind uint8

// Feedback kinds. FeedbackNone means nothing to show.
const (
	FeedbackNone FeedbackKind = iota
	FeedbackCommitted
	FeedbackNotCommitted
	FeedbackTargetExists
)

// ProjectOption is a project offered in the "Project" selector of each file.
type ProjectOption struct {
	ID   string
	Name string
}

// UploadForm is the upload card: the description as typed and the error of the last attempt.
type UploadForm struct {
	Description string
	Error       string
}

// FileItem is one stored AGENTS.md with its "add to project" controls.
type FileItem struct {
	ID          string
	Description string
	Path        string
	// CreatedAt is RFC 3339, as stored in the DTO.
	CreatedAt string
	Projects  []ProjectOption
	// SelectedProjectID is the project chosen in the selector. It survives re-renders, as in the Rust UI.
	SelectedProjectID string
	Feedback          FeedbackKind
	// ApplyError is the error of the last "add to project" of this file.
	ApplyError string
}

// FileList is the list of stored files. Error replaces the list messages when set.
type FileList struct {
	Items []FileItem
	Error string
}

// Page is the whole AGENTS.md screen.
type Page struct {
	Upload UploadForm
	Files  FileList
}

// ErrorText is the message shown for a failed action: the Rust prefix plus the error text.
func ErrorText(err error) string {
	return serverFnErrorPrefix + err.Error()
}

// FormatCreatedAt shows an RFC 3339 timestamp as "02/01/2006 15:04 UTC". A value that does not parse
// is returned unchanged, as the Rust version does.
func FormatCreatedAt(raw string) string {
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return raw
	}
	return parsed.Format(createdAtLayout)
}

// FeedbackFromOutcome maps the result of "add to project" to the feedback to show. Without
// overwrite, an existing AGENTS.md is a typed answer (ask for confirmation), not an error.
func FeedbackFromOutcome(outcome dto.ApplyAgentsFileOutcomeDTO) FeedbackKind {
	switch {
	case !outcome.Applied:
		return FeedbackTargetExists
	case outcome.Committed:
		return FeedbackCommitted
	default:
		return FeedbackNotCommitted
	}
}

// FeedbackMessage is the text of a feedback kind in the language of the request. FeedbackNone is empty.
func FeedbackMessage(ctx context.Context, kind FeedbackKind) string {
	switch kind {
	case FeedbackCommitted:
		return msgAddedCommitted.T(ctx)
	case FeedbackNotCommitted:
		return msgAddedNotCommitted.T(ctx)
	case FeedbackTargetExists:
		return msgTargetExists.T(ctx)
	default:
		return ""
	}
}
