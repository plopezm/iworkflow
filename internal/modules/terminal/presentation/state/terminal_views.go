// Package state builds the view-models of the terminal screens. The handlers fill them from the use
// cases; the templ files only render them.
package state

import (
	"context"
	"time"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/terminal/infrastructure/adapters/inbound/dto"
)

// shortIDLength is how many characters of a session id are shown: enough to tell sessions apart.
const shortIDLength int = 8

// User-facing texts of the terminal state. Each one copies the Rust `Msg` constant it replaces
// (src/modules/terminal/presentation/**), so every language keeps the original wording.
var (
	msgSessionTitle  = i18n.Msg{En: "Terminal {id}", Es: "Terminal {id}", Zh: "终端 {id}", De: "Terminal {id}", Fr: "Terminal {id}"}
	msgTerminalTitle = i18n.Msg{En: "Terminal", Es: "Terminal", Zh: "终端", De: "Terminal", Fr: "Terminal"}
	msgRunning       = i18n.Msg{En: "Running", Es: "Running", Zh: "运行中", De: "Läuft", Fr: "En cours"}
	msgExited        = i18n.Msg{En: "Exited", Es: "Exited", Zh: "已退出", De: "Beendet", Fr: "Terminé"}
)

// ShortID returns the first characters of a session id.
func ShortID(id string) string {
	runes := []rune(id)
	if len(runes) > shortIDLength {
		return string(runes[:shortIDLength])
	}
	return id
}

// StartedAtLabel humanizes an RFC 3339 instant as "dd/mm/yyyy hh:mm UTC", keeping the offset of the
// stored value as the Rust version did. A value that does not parse is shown as it is.
func StartedAtLabel(createdAt string) string {
	parsed, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return createdAt
	}
	return parsed.Format("02/01/2006 15:04") + " UTC"
}

// StatusLabel is the text of a session status.
func StatusLabel(ctx context.Context, status dto.TerminalStatusDTO) string {
	if status == dto.TerminalStatusExitedDTO {
		return msgExited.T(ctx)
	}
	return msgRunning.T(ctx)
}

// StatusClass is the badge class of a session status.
func StatusClass(status dto.TerminalStatusDTO) string {
	if status == dto.TerminalStatusExitedDTO {
		return "badge badge-muted"
	}
	return "badge badge-ok"
}

// SessionTitle is the header of a session page: the workspace name when it is known, otherwise
// "Terminal <short id>".
func SessionTitle(ctx context.Context, workspaceName string, id string) string {
	if workspaceName != "" {
		return workspaceName + " · " + ShortID(id)
	}
	return msgSessionTitle.Fmt(ctx, map[string]string{"id": ShortID(id)})
}

// TabTitle is the default title of a session tab. The page does not rename it, as in the Rust version.
func TabTitle(ctx context.Context, id string) string {
	return TerminalTitle(ctx) + " " + ShortID(id)
}

// TerminalTitle is the word "Terminal" in the language of the request.
func TerminalTitle(ctx context.Context) string {
	return msgTerminalTitle.T(ctx)
}

// SessionRow is one line of the sessions table.
type SessionRow struct {
	ID            string
	ShortID       string
	WorkspaceName string
	StartedAt     string
	StatusLabel   string
	StatusClass   string
	Href          string
}

// ListView is the sessions table. ActionError is the error of the last action on the list and LoadError
// replaces the table when the sessions could not be read.
type ListView struct {
	ActionError string
	LoadError   string
	Rows        []SessionRow
}

// NewListView maps the sessions to table rows, newest first as the use case returns them.
func NewListView(ctx context.Context, sessions []dto.TerminalSessionDTO, actionError string, loadError string) ListView {
	rows := make([]SessionRow, 0, len(sessions))
	for _, session := range sessions {
		rows = append(rows, SessionRow{
			ID:            session.ID,
			ShortID:       ShortID(session.ID),
			WorkspaceName: session.WorkspaceName,
			StartedAt:     StartedAtLabel(session.CreatedAt),
			StatusLabel:   StatusLabel(ctx, session.Status),
			StatusClass:   StatusClass(session.Status),
			Href:          SessionPath(session.ID),
		})
	}
	return ListView{ActionError: actionError, LoadError: loadError, Rows: rows}
}

// SessionPage is the header and the terminal of one session.
type SessionPage struct {
	ID    string
	Title string
}
