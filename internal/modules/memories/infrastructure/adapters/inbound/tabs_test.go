package inbound

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/dock"
	"iworkflow/internal/core/i18n"
	memerrors "iworkflow/internal/modules/memories/domain/errors"
	"iworkflow/internal/modules/memories/domain/models"
)

type stubList struct {
	workspace models.Workspace
	feed      models.Feed
	err       error
}

func (s stubList) Execute(context.Context, string) (models.Workspace, models.Feed, error) {
	return s.workspace, s.feed, s.err
}

type stubTasks struct {
	task      models.TaskRef
	workspace models.Workspace
	feed      models.TaskFeed
	err       error
}

func (s stubTasks) Execute(context.Context, string) (models.TaskRef, models.Workspace, models.TaskFeed, error) {
	return s.task, s.workspace, s.feed, s.err
}

type stubRead struct {
	workspace models.Workspace
	entry     models.Entry
	err       error
	gotID     *int
}

func (s stubRead) Execute(_ context.Context, _ string, id int) (models.Workspace, models.Entry, error) {
	if s.gotID != nil {
		*s.gotID = id
	}
	return s.workspace, s.entry, s.err
}

func render(t *testing.T, component templ.Component) string {
	t.Helper()
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: i18n.Es, Chosen: true})
	var buffer bytes.Buffer
	require.NoError(t, component.Render(ctx, &buffer))
	return buffer.String()
}

var testWorkspace = models.Workspace{ID: "w1", Name: "proj", Path: "/p"}

func TestListTabRendersRowsLinkedToTheReader(t *testing.T) {
	feed := models.Feed{State: models.StateAvailable, Memories: []models.Memory{
		{ID: 7, Type: "bugfix", Title: "<b>Fix</b>", UpdatedAt: time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC)},
	}}
	handlers := NewHandlers(stubList{workspace: testWorkspace, feed: feed}, stubRead{}, nil)

	body := render(t, handlers.ListTab(context.Background(), dock.Params{"id": "w1"}))

	assert.Contains(t, body, `href="/workspaces/w1/memories/7"`)
	assert.Contains(t, body, "&lt;b&gt;Fix&lt;/b&gt;")
	assert.Contains(t, body, "Solo lectura")
	assert.Contains(t, body, `href="/board/w1"`)
	assert.NotContains(t, body, "<form")
	assert.NotContains(t, body, "hx-post")
}

func TestListTabStates(t *testing.T) {
	tests := map[string]struct {
		feed models.Feed
		want string
	}{
		"unavailable": {models.Feed{State: models.StateUnavailable}, "no está disponible"},
		"empty":       {models.Feed{State: models.StateEmpty}, "aún no tiene memorias"},
		"no rows":     {models.Feed{State: models.StateAvailable}, "aún no tiene memorias"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			handlers := NewHandlers(stubList{workspace: testWorkspace, feed: tc.feed}, stubRead{}, nil)
			body := render(t, handlers.ListTab(context.Background(), dock.Params{"id": "w1"}))
			assert.Contains(t, body, tc.want)
			assert.NotContains(t, body, "data-memory-row")
		})
	}
}

func TestListTabMapsErrorsWithoutLeakingDetail(t *testing.T) {
	notFound := NewHandlers(stubList{err: memerrors.ErrWorkspaceNotFound}, stubRead{}, nil)
	assert.Contains(t, render(t, notFound.ListTab(context.Background(), dock.Params{"id": "x"})), "workspace not found")

	broken := NewHandlers(stubList{err: errors.New("dial tcp 10.0.0.1: secret")}, stubRead{}, nil)
	body := render(t, broken.ListTab(context.Background(), dock.Params{"id": "x"}))
	assert.Contains(t, body, "could not be read")
	assert.NotContains(t, body, "secret")
}

func TestDetailTabRendersMarkdownAndEscapesHTML(t *testing.T) {
	var got int
	entry := models.Entry{State: models.StateAvailable, Memory: models.Memory{ID: 9, Type: "decision", Title: "Why"}, Content: "<script>x</script>\nline"}
	handlers := NewHandlers(stubList{}, stubRead{workspace: testWorkspace, entry: entry, gotID: &got}, nil)

	body := render(t, handlers.DetailTab(context.Background(), dock.Params{"id": "w1", "memoryId": "9"}))

	assert.Equal(t, 9, got)
	assert.Contains(t, body, "&lt;script&gt;x&lt;/script&gt;")
	assert.NotContains(t, body, "<script>")
	assert.NotContains(t, body, "<pre", "the content is rendered, not dumped in a pre block")
	assert.Contains(t, body, `href="/workspaces/w1/memories"`)
}

func TestDetailTabRejectsBadIDAndMissingMemory(t *testing.T) {
	called := false
	reader := stubRead{err: memerrors.ErrMemoryNotFound}
	handlers := NewHandlers(stubList{}, reader, nil)
	for _, id := range []string{"abc", "0", "-3", ""} {
		body := render(t, handlers.DetailTab(context.Background(), dock.Params{"id": "w1", "memoryId": id}))
		assert.Contains(t, body, "memory not found", id)
	}
	assert.False(t, called)

	body := render(t, handlers.DetailTab(context.Background(), dock.Params{"id": "w1", "memoryId": "5"}))
	assert.Contains(t, body, "memory not found")
}

func TestTaskTabShowsBranchAndEarlierGroupsLinkedToTheReader(t *testing.T) {
	when := time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC)
	tasks := stubTasks{
		task:      models.TaskRef{ID: "t1", WorkspaceID: "w1", Title: "Mi tarea", Branch: "pi/mi-tarea-ab12", Started: true},
		workspace: testWorkspace,
		feed: models.TaskFeed{
			State:   models.StateAvailable,
			Branch:  []models.Memory{{ID: 8, Type: "bugfix", Title: "Nueva <i>x</i>", UpdatedAt: when}},
			Earlier: []models.Memory{{ID: 2, Type: "decision", Title: "Vieja", UpdatedAt: when}},
		},
	}
	body := render(t, NewHandlers(nil, nil, tasks).TaskTab(context.Background(), dock.Params{"taskId": "t1"}))

	assert.Contains(t, body, "pi/mi-tarea-ab12")
	assert.Contains(t, body, "De esta rama")
	assert.Contains(t, body, "Anteriores")
	assert.Contains(t, body, `href="/workspaces/w1/memories/8"`)
	assert.Contains(t, body, `href="/workspaces/w1/memories/2"`)
	assert.Contains(t, body, "&lt;i&gt;x&lt;/i&gt;")
	assert.Less(t, strings.Index(body, "De esta rama"), strings.Index(body, "Anteriores"), "branch group first")
	assert.Contains(t, body, `href="/tasks/t1"`)
	assert.NotContains(t, body, "<form")
}

func TestTaskTabWithoutBranchMemoriesSaysSoAndKeepsTheEarlierOnes(t *testing.T) {
	tasks := stubTasks{
		task:      models.TaskRef{ID: "t1", WorkspaceID: "w1", Title: "T", Branch: "b", Started: true},
		workspace: testWorkspace,
		feed:      models.TaskFeed{State: models.StateAvailable, Earlier: []models.Memory{{ID: 2, Title: "Vieja"}}},
	}
	body := render(t, NewHandlers(nil, nil, tasks).TaskTab(context.Background(), dock.Params{"taskId": "t1"}))

	assert.Contains(t, body, "Aún no hay memorias escritas en esta rama")
	assert.Contains(t, body, "Vieja")
}

func TestTaskTabStatesAndErrors(t *testing.T) {
	unavailable := stubTasks{task: models.TaskRef{ID: "t1", Title: "T"}, workspace: testWorkspace, feed: models.TaskFeed{State: models.StateUnavailable}}
	assert.Contains(t, render(t, NewHandlers(nil, nil, unavailable).TaskTab(context.Background(), dock.Params{"taskId": "t1"})), "no está disponible")

	missing := stubTasks{err: memerrors.ErrTaskNotFound}
	assert.Contains(t, render(t, NewHandlers(nil, nil, missing).TaskTab(context.Background(), dock.Params{"taskId": "x"})), "task not found")
}

func TestDetailTabRendersMarkdownFormattingAndNeutralizesUnsafeLinks(t *testing.T) {
	content := "**What**: fixed it\n\n- one\n- two\n\n[bad](javascript:alert(1)) and [ok](https://example.com)"
	entry := models.Entry{State: models.StateAvailable, Memory: models.Memory{ID: 1, Title: "T"}, Content: content}
	handlers := NewHandlers(stubList{}, stubRead{workspace: testWorkspace, entry: entry}, nil)

	body := render(t, handlers.DetailTab(context.Background(), dock.Params{"id": "w1", "memoryId": "1"}))

	assert.Contains(t, body, "<strong>What</strong>")
	assert.Contains(t, body, "<li>one</li>")
	assert.NotContains(t, body, "javascript:")
	assert.Contains(t, body, `href="https://example.com"`)
	assert.NotContains(t, body, "**What**", "the Markdown markers are gone")
}

func TestListTabMakesTheWholeCardTheLink(t *testing.T) {
	feed := models.Feed{State: models.StateAvailable, Memories: []models.Memory{{ID: 7, Type: "bugfix", Title: "Fix", UpdatedAt: time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC)}}}
	handlers := NewHandlers(stubList{workspace: testWorkspace, feed: feed}, stubRead{}, nil)

	body := render(t, handlers.ListTab(context.Background(), dock.Params{"id": "w1"}))

	row := body[strings.Index(body, "data-memory-row"):]
	row = row[:strings.Index(row, "</li>")]
	assert.Contains(t, row, `href="/workspaces/w1/memories/7"`)
	assert.Contains(t, row, "bugfix", "type and date are inside the link, so tapping them opens the memory too")
	assert.Less(t, strings.Index(row, "<a "), strings.Index(row, "bugfix"))
	assert.Greater(t, strings.Index(row, "</a>"), strings.Index(row, "bugfix"))
}
