package tasks_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/domain/models"
)

// The message box of the detail page: it sits below the transcript of a task that still takes messages.

func TestDetailOfALiveTaskShowsTheComposerBelowTheTranscript(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	running := newTask(t, workspaceID, "composer-running", models.StatusRunning)
	insertTask(t, repo, running)
	module, _ := boardRouter(t, repo)
	id := running.ID.String()

	body := renderDetailTab(t, module, i18n.En, id)

	assert.Contains(t, body, `id="task-composer-`, "a live agent takes messages, so the composer is on the page")
	assert.Contains(t, body, `hx-post="/tasks/`+id+`/messages?scope=`, "the send answers this page instance")
	assert.Contains(t, body, `placeholder="Write a message for the agent…`)
	assert.Less(t, strings.Index(body, `id="task-transcript-scroll-`), strings.Index(body, `id="task-composer-`),
		"the composer sits below the transcript")
}

func TestDetailOfATaskThatTakesNoMessagesShowsNoComposer(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, t.TempDir())
	reviewed := newTask(t, workspaceID, "composer-review", models.StatusAwaitingReview)
	insertTask(t, repo, reviewed)
	module, _ := boardRouter(t, repo)

	body := renderDetailTab(t, module, i18n.En, reviewed.ID.String())

	assert.NotContains(t, body, `id="task-composer-`, "no live agent: nothing to send the message to")
	assert.NotContains(t, body, `/messages?scope=`)
}
