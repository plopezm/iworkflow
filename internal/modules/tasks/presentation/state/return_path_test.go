package state_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/tasks/presentation/state"
)

// SafeReturnPath keeps only local dock paths: the create tab must never send the user out of the app.
func TestSafeReturnPathKeepsOnlyLocalPaths(t *testing.T) {
	cases := []struct {
		name   string
		origin string
		want   string
	}{
		{name: "root", origin: "/", want: "/"},
		{name: "board", origin: "/board/x", want: "/board/x"},
		{name: "empty", origin: "", want: ""},
		{name: "relative", origin: "x", want: ""},
		{name: "protocol relative host", origin: "//evil", want: ""},
		{name: "backslash", origin: "/a\\b", want: ""},
		{name: "backslash host", origin: "/\\evil.com", want: ""},
		{name: "control character", origin: "/a\x01b", want: ""},
		{name: "newline", origin: "/a\nb", want: ""},
		{name: "delete character", origin: "/a\x7fb", want: ""},
		{name: "absolute url", origin: "http://x", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, state.SafeReturnPath(tc.origin))
		})
	}
}

// NewTaskPathFrom carries the origin in the path, escaped as one segment, so the dock can open it.
func TestNewTaskPathFromCarriesTheEscapedOrigin(t *testing.T) {
	cases := []struct {
		name        string
		workspaceID string
		origin      string
		want        string
	}{
		{name: "dashboard origin without workspace", workspaceID: "", origin: "/", want: "/board/new/from/%2F"},
		{name: "dashboard origin with workspace", workspaceID: "w1", origin: "/", want: "/board/w1/new/from/%2F"},
		{name: "board origin", workspaceID: "w1", origin: "/board/w2", want: "/board/w1/new/from/%2Fboard%2Fw2"},
		{name: "unsafe origin without workspace", workspaceID: "", origin: "//evil.com", want: "/board/new"},
		{name: "unsafe origin with workspace", workspaceID: "w1", origin: "//evil.com", want: "/board/w1/new"},
		{name: "no origin", workspaceID: "w1", origin: "", want: "/board/w1/new"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, state.NewTaskPathFrom(tc.workspaceID, tc.origin))
		})
	}
}

func TestNewTaskPathFromEscapesTheOriginIntoOneSegment(t *testing.T) {
	path := state.NewTaskPathFrom("w1", "/board/w2?x=1")

	assert.NotContains(t, path[len("/board/w1/new/from/"):], "/", "the origin must stay one path segment")
	segment := path[len("/board/w1/new/from/"):]
	decoded, err := url.PathUnescape(segment)
	assert.NoError(t, err)
	assert.Equal(t, "/board/w2?x=1", decoded)
}

// ReturnPath is where the create tab goes when it ends: the origin when it is safe, else the workspace board.
func TestReturnPathPrefersASafeOriginAndFallsBackToTheBoard(t *testing.T) {
	cases := []struct {
		name        string
		returnTo    string
		workspaceID string
		want        string
	}{
		{name: "safe origin wins", returnTo: "/", workspaceID: "w1", want: "/"},
		{name: "safe board origin wins", returnTo: "/board/w2", workspaceID: "w1", want: "/board/w2"},
		{name: "unsafe origin falls back to the board", returnTo: "//evil", workspaceID: "w1", want: "/board/w1"},
		{name: "no origin falls back to the board", returnTo: "", workspaceID: "w1", want: "/board/w1"},
		{name: "no origin and no workspace goes to the dashboard", returnTo: "", workspaceID: "", want: "/"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, state.ReturnPath(tc.returnTo, tc.workspaceID))
		})
	}
}
