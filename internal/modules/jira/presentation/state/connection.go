package state

import (
	"strings"

	"iworkflow/internal/modules/jira/infrastructure/adapters/inbound/dto"
)

// sameServer reports whether two URLs point at the same server, ignoring trailing slashes and spaces.
func sameServer(a, b string) bool {
	return strings.TrimRight(strings.TrimSpace(a), "/") == strings.TrimRight(strings.TrimSpace(b), "/")
}

// CanSubmit reports whether the connection form is valid to submit.
// The form can be sent with a URL and either a typed token or the stored one. The stored token
// is only reused for the server it was saved for: pointing at another URL needs a new token.
func CanSubmit(url string, token string, storedURL *string) bool {
	trimmedURL := strings.TrimSpace(url)
	if trimmedURL == "" {
		return false
	}
	if strings.TrimSpace(token) != "" {
		return true
	}
	return storedURL != nil && sameServer(*storedURL, trimmedURL)
}

// ShortTimestamp formats an RFC3339 timestamp string to "YYYY-MM-DD HH:MM".
// Example: "2024-05-01T10:00:00+00:00" -> "2024-05-01 10:00".
func ShortTimestamp(raw string) string {
	runes := []rune(raw)
	if len(runes) > 16 {
		runes = runes[:16]
	}
	return strings.ReplaceAll(string(runes), "T", " ")
}

// ConnectionCardView holds the state needed to render the Jira connection card.
type ConnectionCardView struct {
	Editing                bool
	ConfirmingDelete       bool
	Stored                 *dto.ConnectionDTO
	FormURL                string
	FormAcceptInvalidCerts bool
	Err                    string
}

// NewConnectionCardView builds a view for viewing the connection summary.
func NewConnectionCardView(stored *dto.ConnectionDTO) ConnectionCardView {
	return ConnectionCardView{
		Stored: stored,
	}
}

// NewEditConnectionCardView builds a view for editing the connection, prefilling stored values.
func NewEditConnectionCardView(stored *dto.ConnectionDTO) ConnectionCardView {
	view := ConnectionCardView{
		Editing: true,
		Stored:  stored,
	}
	if stored != nil {
		view.FormURL = stored.BaseURL
		view.FormAcceptInvalidCerts = stored.AcceptInvalidCerts
	}
	return view
}

// StoredURL returns the stored base URL if a connection is configured.
func (v ConnectionCardView) StoredURL() *string {
	if v.Stored != nil {
		return &v.Stored.BaseURL
	}
	return nil
}

// LastSyncAtFormatted returns the formatted timestamp of the last sync, or empty string if none.
func (v ConnectionCardView) LastSyncAtFormatted() string {
	if v.Stored != nil && v.Stored.LastSyncAt != nil && *v.Stored.LastSyncAt != "" {
		return ShortTimestamp(*v.Stored.LastSyncAt)
	}
	return ""
}
