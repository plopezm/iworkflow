package components_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/components"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// finishPreviewView is the dialog of a task with the processes the caller lists.
func finishPreviewView(processes ...dto.FinishProcessDTO) state.FinishPreviewView {
	return state.FinishPreviewView{Scope: "abc123", TaskID: "t1", Processes: processes}
}

func TestFinishPreviewWithoutProcessesOnlyAsksToConfirm(t *testing.T) {
	t.Parallel()

	body := renderIn(t, i18n.En, components.FinishPreview(finishPreviewView()))

	assert.Contains(t, body, `id="finish-panel-abc123"`, "the dialog has the id of this page instance")
	assert.Contains(t, body, `role="dialog"`)
	assert.Contains(t, body, "Finish the task? The agent will be closed", "the confirmation text")
	assert.Contains(t, body, `hx-post="/tasks/t1/finish?scope=abc123"`, "the form finishes this task")
	assert.Contains(t, body, `hx-target="#task-detail-abc123"`, "the answer is the page of this instance")
	assert.Contains(t, body, ">Cancel</button>", "cancel only closes the dialog")
	assert.Contains(t, body, `data-modal-close`)
	assert.Contains(t, body, `type="submit"`)
	assert.NotContains(t, body, `name="process"`, "no process, no checkbox")
}

func TestFinishPreviewChecksEachProcessWithItsPidPathAndReason(t *testing.T) {
	t.Parallel()
	view := finishPreviewView(
		dto.FinishProcessDTO{PID: 42, Name: "node", Path: "/wt/t1", Reason: models.ProcessReasonWorkingDirectory, Value: "42|Thu Nov 19 10:00:00 2026"},
		dto.FinishProcessDTO{PID: 77, Name: "vim", Path: "/wt/t1/main.go", Reason: models.ProcessReasonOpenFile, Value: "77|Thu Nov 19 11:00:00 2026"},
	)

	body := renderIn(t, i18n.En, components.FinishPreview(view))

	assert.Equal(t, 2, strings.Count(body, `type="checkbox" name="process"`), "one checkbox per process")
	assert.Equal(t, 2, strings.Count(body, ` checked`), "every process is checked by default")
	assert.Contains(t, body, `value="42|Thu Nov 19 10:00:00 2026"`, "the value keeps the start token with its spaces")
	assert.Contains(t, body, "node · PID 42", "the name and the pid")
	assert.Contains(t, body, "working directory", "the reason of the first process")
	assert.Contains(t, body, "/wt/t1/main.go", "the path of the second process")
	assert.Contains(t, body, "open file", "the reason of the second process")
	assert.Contains(t, body, "Checked processes will be closed. Unchecked ones keep running.", "what checking means")
}

func TestFinishPreviewLabelsTheReasonsInSpanish(t *testing.T) {
	t.Parallel()
	view := finishPreviewView(
		dto.FinishProcessDTO{PID: 5, Name: "bash", Path: "/wt/t1", Reason: models.ProcessReasonExecutable, Value: "5|s"},
	)

	body := renderIn(t, i18n.Es, components.FinishPreview(view))

	assert.Contains(t, body, "ejecutable")
	assert.Contains(t, body, ">Finalizar</button>", "the submit reads Finalizar")
	assert.Contains(t, body, ">Cancelar</button>")
}

func TestFinishPreviewEscapesProcessNamesAndPaths(t *testing.T) {
	t.Parallel()
	view := finishPreviewView(
		dto.FinishProcessDTO{PID: 9, Name: `<img src=x onerror=alert(1)>`, Path: `/wt/"quoted"`, Reason: models.ProcessReasonOpenFile, Value: "9|s"},
	)

	body := renderIn(t, i18n.En, components.FinishPreview(view))

	assert.NotContains(t, body, `<img src=x`, "a process name is text, never markup")
	assert.Contains(t, body, "&lt;img src=x")
}

// The Finish button of the actions tab asks for the dialog into its slot; it no longer confirms natively.
func TestTaskActionsFinishAsksThroughTheDialogSlot(t *testing.T) {
	t.Parallel()
	view := taskActionsView("t1", false, false)
	view.Task.Actions.CanFinish = true

	body := renderIn(t, i18n.En, components.TaskActions(view))

	assert.Contains(t, body, `hx-get="/tasks/t1/finish/preview?scope=abc123"`, "the button asks for the dialog of this page")
	assert.Contains(t, body, `hx-target="#task-finish-abc123"`, "the dialog lands in the slot of this page")
	assert.Contains(t, body, `id="task-finish-abc123"`, "the slot exists in the actions tab")
	assert.NotContains(t, body, `hx-confirm="Finish the task?`, "the native confirm gives way to the dialog")
	assert.NotContains(t, body, `hx-post="/tasks/t1/finish`, "the button does not finish by itself")
}
