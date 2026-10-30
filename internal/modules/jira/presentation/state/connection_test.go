package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/jira/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/jira/presentation/state"
)

func TestCanSubmit(t *testing.T) {
	stored := "https://jira.example.com"
	storedWithSlash := "https://jira.example.com/"

	tests := []struct {
		name      string
		url       string
		token     string
		storedURL *string
		expected  bool
	}{
		{
			name:      "needs a url and a token when none is stored",
			url:       "https://j",
			token:     "tok",
			storedURL: nil,
			expected:  true,
		},
		{
			name:      "empty token rejected when none is stored",
			url:       "https://j",
			token:     "  ",
			storedURL: nil,
			expected:  false,
		},
		{
			name:      "empty token accepted when url matches stored url",
			url:       "https://j",
			token:     "",
			storedURL: ptr("https://j"),
			expected:  true,
		},
		{
			name:      "blank url rejected even if token and stored url exist",
			url:       "  ",
			token:     "tok",
			storedURL: ptr("https://j"),
			expected:  false,
		},
		{
			name:      "trailing slash differences are ignored for same server",
			url:       "https://j/",
			token:     "",
			storedURL: ptr("https://j"),
			expected:  true,
		},
		{
			name:      "trailing slash on stored url is ignored for same server",
			url:       "https://jira.example.com",
			token:     "",
			storedURL: &storedWithSlash,
			expected:  true,
		},
		{
			name:      "whitespace in url is trimmed for same server check",
			url:       "  https://jira.example.com/  ",
			token:     "",
			storedURL: &stored,
			expected:  true,
		},
		{
			name:      "stored token is not reused for another server",
			url:       "https://otro.example.com",
			token:     "",
			storedURL: ptr("https://j"),
			expected:  false,
		},
		{
			name:      "new server accepted if new token provided",
			url:       "https://otro.example.com",
			token:     "nuevo",
			storedURL: ptr("https://j"),
			expected:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := state.CanSubmit(tc.url, tc.token, tc.storedURL)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestShortTimestamp(t *testing.T) {
	assert.Equal(t, "2024-05-01 10:00", state.ShortTimestamp("2024-05-01T10:00:00+00:00"))
	assert.Equal(t, "2024-05-01 10:00", state.ShortTimestamp("2024-05-01T10:00:15Z"))
	assert.Equal(t, "2024-05-01 10:00", state.ShortTimestamp("2024-05-01 10:00:00"))
	assert.Equal(t, "short", state.ShortTimestamp("short"))
	assert.Equal(t, "", state.ShortTimestamp(""))
}

func TestConnectionCardView(t *testing.T) {
	stored := &dto.ConnectionDTO{
		BaseURL:            "https://jira.example.com",
		AcceptInvalidCerts: true,
		LastSyncAt:         ptr("2024-05-01T10:00:00Z"),
	}

	view := state.NewConnectionCardView(stored)
	assert.False(t, view.Editing)
	assert.False(t, view.ConfirmingDelete)
	assert.Equal(t, stored, view.Stored)
	assert.Equal(t, "https://jira.example.com", *view.StoredURL())
	assert.Equal(t, "2024-05-01 10:00", view.LastSyncAtFormatted())

	editView := state.NewEditConnectionCardView(stored)
	assert.True(t, editView.Editing)
	assert.Equal(t, "https://jira.example.com", editView.FormURL)
	assert.True(t, editView.FormAcceptInvalidCerts)
	assert.Equal(t, stored, editView.Stored)
}

func ptr(s string) *string {
	return &s
}
