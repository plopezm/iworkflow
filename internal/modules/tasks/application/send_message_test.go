package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

// messageImage is a valid PNG attachment for the message tests.
func messageImage() models.ImageAttachment {
	return models.ImageAttachment{Name: "a.png", MimeType: "image/png", Data: "aGk="}
}

// messageFixture returns the use case over a single task with the given status and runtime state.
func messageFixture(t *testing.T, status models.TaskStatus, running bool, capabilities models.AgentCapabilities) (*application.SendMessage, *fakeRepository, *fakeRuntime, models.TaskID) {
	t.Helper()
	task := newTask(status)
	repo := newFakeRepository(task)
	lifecycle, _ := newLifecycle(repo)
	runtime := &fakeRuntime{running: running, capabilities: capabilities}
	return application.NewSendMessage(lifecycle, runtime), repo, runtime, task.ID
}

// Rust: send_message::idle_agent_receives_prompt
func TestSendMessageIdleAgentReceivesPrompt(t *testing.T) {
	t.Parallel()
	useCase, _, runtime, id := messageFixture(t, models.StatusIdle, false, models.MinimalAgentCapabilities())

	err := useCase.Execute(context.Background(), id, "  hola ", nil, models.MessageModeSteer)

	require.NoError(t, err)
	assert.Equal(t, []models.AgentCommand{models.PromptCommand{Message: "hola", Images: []models.ImageAttachment{}}}, runtime.sent)
}

// Rust: send_message::running_agent_receives_steer_or_follow_up
func TestSendMessageRunningAgentReceivesSteerOrFollowUp(t *testing.T) {
	t.Parallel()
	useCase, _, runtime, id := messageFixture(t, models.StatusRunning, true, models.FullAgentCapabilities())
	require.NoError(t, useCase.Execute(context.Background(), id, "  hola ", nil, models.MessageModeSteer))

	startingUseCase, _, startingRuntime, startingID := messageFixture(t, models.StatusStarting, true, models.FullAgentCapabilities())
	require.NoError(t, startingUseCase.Execute(context.Background(), startingID, "  hola ", nil, models.MessageModeFollowUp))

	assert.Equal(t, []models.AgentCommand{models.SteerCommand{Message: "hola", Images: []models.ImageAttachment{}}}, runtime.sent)
	assert.Equal(t, []models.AgentCommand{models.FollowUpCommand{Message: "hola", Images: []models.ImageAttachment{}}}, startingRuntime.sent)
}

// Rust: send_message::rejects_messages_when_no_agent_is_live
func TestSendMessageRejectsMessagesWhenNoAgentIsLive(t *testing.T) {
	t.Parallel()
	useCase, _, runtime, id := messageFixture(t, models.StatusAwaitingReview, false, models.FullAgentCapabilities())

	err := useCase.Execute(context.Background(), id, "hola", nil, models.MessageModeSteer)

	require.ErrorIs(t, err, werrors.InvalidState("enviar mensaje", "awaiting_review"))
	assert.Empty(t, runtime.sent)
}

// Rust: send_message::forwards_images_and_allows_empty_text_with_images
func TestSendMessageForwardsImagesAndAllowsEmptyTextWithImages(t *testing.T) {
	t.Parallel()
	useCase, _, runtime, id := messageFixture(t, models.StatusIdle, false, models.MinimalAgentCapabilities())

	err := useCase.Execute(context.Background(), id, "  ", []models.ImageAttachment{messageImage()}, models.MessageModeSteer)

	require.NoError(t, err)
	assert.Equal(t, []models.AgentCommand{models.PromptCommand{Images: []models.ImageAttachment{messageImage()}}}, runtime.sent)
}

// Rust: send_message::rejects_empty_text_without_images
func TestSendMessageRejectsEmptyTextWithoutImages(t *testing.T) {
	t.Parallel()
	useCase, _, runtime, id := messageFixture(t, models.StatusIdle, false, models.MinimalAgentCapabilities())

	err := useCase.Execute(context.Background(), id, "  ", nil, models.MessageModeSteer)

	require.ErrorIs(t, err, werrors.EmptyMessage())
	assert.Empty(t, runtime.sent)
}

// Rust: send_message::rejects_invalid_images
func TestSendMessageRejectsInvalidImages(t *testing.T) {
	t.Parallel()
	useCase, _, runtime, id := messageFixture(t, models.StatusIdle, false, models.MinimalAgentCapabilities())
	bad := messageImage()
	bad.MimeType = "application/pdf"

	err := useCase.Execute(context.Background(), id, "hola", []models.ImageAttachment{bad}, models.MessageModeSteer)

	require.ErrorIs(t, err, werrors.InvalidAttachment(""))
	assert.Empty(t, runtime.sent)
}

// Rust: send_message::rejects_steer_and_follow_up_when_runtime_lacks_capability
func TestSendMessageRejectsSteerAndFollowUpWhenRuntimeLacksCapability(t *testing.T) {
	t.Parallel()
	cases := []struct {
		mode    models.MessageMode
		refusal string
	}{
		{mode: models.MessageModeSteer, refusal: "el agente no permite redirigir (steer) mientras trabaja"},
		{mode: models.MessageModeFollowUp, refusal: "el agente no permite encolar mensajes (follow-up) mientras trabaja"},
	}
	for _, tc := range cases {
		useCase, _, runtime, id := messageFixture(t, models.StatusRunning, true, models.MinimalAgentCapabilities())

		err := useCase.Execute(context.Background(), id, "hola", nil, tc.mode)

		require.ErrorIs(t, err, werrors.Unsupported(tc.refusal))
		assert.Empty(t, runtime.sent)
	}
}

// Rust: send_message::stopped_task_yields_agent_not_running_before_checking_capabilities
func TestSendMessageStoppedTaskYieldsAgentNotRunningBeforeCheckingCapabilities(t *testing.T) {
	t.Parallel()
	for _, mode := range []models.MessageMode{models.MessageModeSteer, models.MessageModeFollowUp} {
		useCase, _, runtime, id := messageFixture(t, models.StatusRunning, false, models.FullAgentCapabilities())

		err := useCase.Execute(context.Background(), id, "hola", nil, mode)

		require.ErrorIs(t, err, werrors.AgentNotRunning())
		assert.Equal(t, 1, runtime.runningCalls)
		assert.Equal(t, 0, runtime.capabilityCalls, "capabilities are not asked of a stopped agent")
		assert.Empty(t, runtime.sent)
	}
}

// Rust: send_message::idle_prompt_does_not_require_extra_capabilities
func TestSendMessageIdlePromptDoesNotRequireExtraCapabilities(t *testing.T) {
	t.Parallel()
	useCase, _, runtime, id := messageFixture(t, models.StatusIdle, false, models.MinimalAgentCapabilities())

	err := useCase.Execute(context.Background(), id, "hola", nil, models.MessageModeSteer)

	require.NoError(t, err)
	assert.Equal(t, 0, runtime.capabilityCalls)
	assert.Equal(t, 0, runtime.runningCalls, "a prompt to an idle agent does not ask whether it is running")
	assert.Equal(t, []models.AgentCommand{models.PromptCommand{Message: "hola", Images: []models.ImageAttachment{}}}, runtime.sent)
}

// Rule from source: an idle agent takes a plain prompt whatever the mode, because the mode only matters while the
// agent works. Rust only sends steer to an idle agent.
func TestSendMessageIdleAgentReceivesPromptWhateverTheMode(t *testing.T) {
	t.Parallel()
	useCase, _, runtime, id := messageFixture(t, models.StatusIdle, false, models.MinimalAgentCapabilities())

	err := useCase.Execute(context.Background(), id, "hola", nil, models.MessageModeFollowUp)

	require.NoError(t, err)
	assert.Equal(t, []models.AgentCommand{models.PromptCommand{Message: "hola", Images: []models.ImageAttachment{}}}, runtime.sent)
}

// Rule from source: text and images are checked before the task is loaded, so a bad message costs no repository read.
func TestSendMessageRejectsEmptyMessageAndBadImagesBeforeLoadingTheTask(t *testing.T) {
	t.Parallel()
	bad := messageImage()
	bad.MimeType = "application/pdf"
	cases := map[string]struct {
		text   string
		images []models.ImageAttachment
		want   error
	}{
		"empty text without images": {text: "  ", want: werrors.EmptyMessage()},
		"invalid image":             {text: "hola", images: []models.ImageAttachment{bad}, want: werrors.InvalidAttachment("")},
	}
	for name, tc := range cases {
		useCase, repo, runtime, id := messageFixture(t, models.StatusRunning, true, models.FullAgentCapabilities())

		err := useCase.Execute(context.Background(), id, tc.text, tc.images, models.MessageModeSteer)

		require.ErrorIs(t, err, tc.want, name)
		assert.Equal(t, int64(0), repo.finds.Load(), name)
		assert.Empty(t, runtime.sent, name)
	}
}

// Rule from source: a mode outside steer and follow-up is refused before anything is sent. The Go type allows a zero
// value, which Rust's enum does not.
func TestSendMessageRejectsUnknownModeWhileTheAgentWorks(t *testing.T) {
	t.Parallel()
	useCase, _, runtime, id := messageFixture(t, models.StatusRunning, true, models.FullAgentCapabilities())

	err := useCase.Execute(context.Background(), id, "hola", nil, models.MessageMode(0))

	require.ErrorIs(t, err, werrors.Unsupported("modo de mensaje desconocido"))
	assert.Empty(t, runtime.sent)
}
