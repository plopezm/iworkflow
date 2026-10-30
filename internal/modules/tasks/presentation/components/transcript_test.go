package components_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/components"
	"iworkflow/internal/modules/tasks/presentation/state"
)

func renderTranscript(t *testing.T, component templ.Component) string {
	t.Helper()
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: i18n.Es, Chosen: true})
	var out bytes.Buffer
	require.NoError(t, component.Render(ctx, &out))
	return out.String()
}

func bodyOf(t *testing.T, events ...dto.TranscriptEventDTO) string {
	t.Helper()
	transcript, _ := state.FoldTranscript(events)
	return renderTranscript(t, components.TranscriptBody("t1", transcript))
}

func agentEvent(event dto.TranscriptAgentEvent) dto.TranscriptEventDTO {
	return dto.TranscriptEventDTO{Agent: event}
}

func TestTranscriptBodyShowsTheNoActivityNoteWhenEmpty(t *testing.T) {
	t.Parallel()

	assert.Contains(t, bodyOf(t), "Sin actividad todavía.")
}

func TestTranscriptBodyShowsTheImageCountAndEscapesPlainText(t *testing.T) {
	t.Parallel()

	html := bodyOf(t, agentEvent(dto.TranscriptUserMessage{Text: "<script>x</script>", Images: 2}))

	assert.Contains(t, html, "📎 2 imágenes")
	assert.Contains(t, html, "&lt;script&gt;x&lt;/script&gt;")
	assert.NotContains(t, html, "<script>x</script>")
}

// Rule: the agent's messages are rendered as markdown, and the sanitizer keeps raw HTML out.
func TestTranscriptBodyRendersTheAgentMessageAsMarkdown(t *testing.T) {
	t.Parallel()

	html := bodyOf(t, agentEvent(dto.TranscriptAssistantMessage{Text: "**eco**: jobs-wait"}))

	assert.Contains(t, html, "<strong>eco</strong>: jobs-wait")
	assert.Contains(t, html, "Agente")
}

func TestTranscriptBodyShowsAToolWithItsSummaryAndState(t *testing.T) {
	t.Parallel()

	html := bodyOf(t,
		agentEvent(dto.TranscriptToolStarted{CallID: "c1", ToolName: "read", Args: `{"path":"src/a.rs","limit":3}`}),
		agentEvent(dto.TranscriptToolFinished{CallID: "c1", ToolName: "read", Output: "fin", IsError: false}),
	)

	assert.Contains(t, html, "🔧 read · ok · src/a.rs")
	assert.Contains(t, html, `class="tool tool-ok"`)
	assert.Contains(t, html, `<dt>limit</dt>`)
	assert.Contains(t, html, `class="tool-output`)
}

func TestTranscriptBodyShowsARunningToolUntilItFinishes(t *testing.T) {
	t.Parallel()

	html := bodyOf(t, agentEvent(dto.TranscriptToolStarted{CallID: "c1", ToolName: "bash", Args: `{"command":"ls"}`}))

	assert.Contains(t, html, "🔧 bash · en curso · ls")
	assert.Contains(t, html, `class="tool tool-running"`)
}

func TestTranscriptBodyCollapsesTheLaunchNotice(t *testing.T) {
	t.Parallel()

	html := bodyOf(t, agentEvent(dto.TranscriptNotice{Level: "info", Message: "Arrancando args=[--x] cwd=/wt"}))

	assert.Contains(t, html, "<summary")
	assert.Contains(t, html, "Detalles de arranque")
	assert.Contains(t, html, "Arrancando args=[--x] cwd=/wt")
}

func TestTranscriptBodyShowsADialogAsAnsweredOrPending(t *testing.T) {
	t.Parallel()
	dialog := dto.TranscriptDialogRequested{Request: dto.TranscriptDialog{ID: "d1", Title: "¿Seguir?"}}

	pending := bodyOf(t, agentEvent(dialog))
	answered := bodyOf(t, agentEvent(dialog), agentEvent(dto.TranscriptDialogResolved{DialogID: "d1"}))

	assert.Contains(t, pending, "❓ ¿Seguir? (pendiente)")
	assert.Contains(t, answered, "❓ ¿Seguir? (respondido)")
}

func TestTranscriptBodyShowsTheExitCodeOrItsAbsence(t *testing.T) {
	t.Parallel()
	code := int32(2)

	assert.Contains(t, bodyOf(t, agentEvent(dto.TranscriptProcessExited{Code: &code})), "Agente detenido (código 2)")
	assert.Contains(t, bodyOf(t, agentEvent(dto.TranscriptProcessExited{Code: nil})), "Agente detenido (sin código)")
}

// Rule: the page keeps the element that listens to the task stream and swaps only its body. The stream starts after
// the cursor and the refresh is asked again on each event, after a short pause.
func TestTranscriptPanelListensToTheTaskStreamAndSwapsOnlyItsBody(t *testing.T) {
	t.Parallel()
	cursor := int64(5)
	view := state.TaskDetailView{Task: dto.TaskDTO{ID: "t-1"}, TranscriptCursor: &cursor}

	html := renderTranscript(t, components.TranscriptPanel(view))

	assert.Contains(t, html, `sse-connect="/api/tasks/t-1/events?after=5"`)
	assert.Contains(t, html, `hx-get="/tasks/t-1/transcript"`)
	assert.Contains(t, html, `hx-trigger="sse:message delay:300ms"`)
	assert.Contains(t, html, `hx-swap="morph:innerHTML"`)
	assert.Contains(t, html, `hx-sync="this:replace"`)
}

func TestTranscriptPanelShowsTheReasonWhenTheHistoryCannotBeRead(t *testing.T) {
	t.Parallel()
	view := state.TaskDetailView{Task: dto.TaskDTO{ID: "t-1"}, TranscriptError: "base de datos bloqueada"}

	html := renderTranscript(t, components.TranscriptPanel(view))

	assert.Contains(t, html, "base de datos bloqueada")
	assert.NotContains(t, html, "Sin actividad todavía.")
}

// Rule: a refresh morphs the transcript and every item keeps a stable id, so a details the reader opened stays open.
func TestTranscriptBodyGivesEveryItemAStableID(t *testing.T) {
	t.Parallel()

	html := bodyOf(t,
		agentEvent(dto.TranscriptUserMessage{Text: "uno"}),
		agentEvent(dto.TranscriptToolStarted{CallID: "c1", ToolName: "bash", Args: `{"command":"ls"}`}),
	)

	assert.Contains(t, html, `id="tr-t1-0"`)
	assert.Contains(t, html, `id="tr-t1-1"`)
}
