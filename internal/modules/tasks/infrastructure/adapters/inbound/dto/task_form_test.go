package dto

import (
	"bytes"
	"encoding/base64"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
	"iworkflow/internal/modules/tasks/domain/models"
)

// pngBytes is a one-pixel PNG, small enough for a test body.
var pngBytes = mustDecode("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==")

func mustDecode(value string) []byte {
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		panic(err)
	}
	return data
}

// validForm is a create form that passes every check; each test changes one field of it.
func validForm() TaskForm {
	return TaskForm{
		WorkspaceID:  uuid.NewString(),
		Title:        "Fix the login",
		Prompt:       "Investigate the failing login",
		Priority:     "high",
		ShowAdvanced: true,
	}
}

func TestToNewTaskIgnoresLimitsWhileTheAdvancedOptionsAreHidden(t *testing.T) {
	t.Parallel()
	form := validForm()
	form.ShowAdvanced = false
	form.MemoryLimit = "abc"
	form.CPULimit = "0"

	input, _, err := form.ToNewTask()

	require.NoError(t, err, "a hidden limit is not validated: the settings default applies")
	assert.Nil(t, input.Limits.MemoryMB)
	assert.Nil(t, input.Limits.CPUPercent)
}

func TestToNewTaskMapsAValidForm(t *testing.T) {
	t.Parallel()
	form := validForm()
	form.BaseBranch = "  origin/develop  "
	form.MemoryLimit = "512"
	form.CPULimit = "150"
	form.StartNow = true
	form.LinkKind = "next"
	anchor := uuid.NewString()
	form.LinkAnchor = anchor

	input, link, err := form.ToNewTask()

	require.NoError(t, err)
	assert.Equal(t, form.WorkspaceID, input.WorkspaceID)
	assert.Equal(t, models.TaskPriority("high"), input.Priority)
	require.NotNil(t, input.BaseBranch)
	assert.Equal(t, "origin/develop", *input.BaseBranch, "the branch is trimmed")
	assert.Equal(t, uint32(512), *input.Limits.MemoryMB)
	assert.Equal(t, uint32(150), *input.Limits.CPUPercent)
	assert.True(t, input.StartNow, "the start request reaches the use case, which decides for linked tasks")
	require.NotNil(t, link)
	assert.Equal(t, application.LinkNext, link.Kind)
	assert.Equal(t, anchor, link.Anchor.String())
}

func TestToNewTaskUsesTheDefaultsWhenTheOptionalFieldsAreEmpty(t *testing.T) {
	t.Parallel()
	form := validForm()
	form.Priority = ""

	input, link, err := form.ToNewTask()

	require.NoError(t, err)
	assert.Equal(t, models.DefaultTaskPriority, input.Priority, "a form without a priority gets the default")
	assert.Nil(t, input.BaseBranch, "an empty branch is the workspace base branch")
	assert.Nil(t, input.Limits.MemoryMB, "empty limits are the settings defaults")
	assert.Nil(t, input.Limits.CPUPercent)
	assert.Nil(t, link)
}

func TestToNewTaskRejectsEveryInvalidField(t *testing.T) {
	t.Parallel()
	tests := map[string]func(*TaskForm){
		"workspace is not an identifier": func(f *TaskForm) { f.WorkspaceID = "not-a-uuid" },
		"unknown priority":               func(f *TaskForm) { f.Priority = "eventually" },
		"memory is not a number":         func(f *TaskForm) { f.MemoryLimit = "abc" },
		"memory is zero":                 func(f *TaskForm) { f.MemoryLimit = "0" },
		"memory is negative":             func(f *TaskForm) { f.MemoryLimit = "-3" },
		"cpu is fractional":              func(f *TaskForm) { f.CPULimit = "1.5" },
		"cpu overflows 32 bits":          func(f *TaskForm) { f.CPULimit = "99999999999" },
		"branch reads as a git option":   func(f *TaskForm) { f.BaseBranch = "--upload-pack=x" },
		"branch is too long":             func(f *TaskForm) { f.BaseBranch = strings.Repeat("a", MaxBaseBranchLength+1) },
		"link without a kind":            func(f *TaskForm) { f.LinkAnchor = uuid.NewString() },
		"link kind is unknown":           func(f *TaskForm) { f.LinkKind = "sideways"; f.LinkAnchor = uuid.NewString() },
		"link anchor is not an id":       func(f *TaskForm) { f.LinkKind = "next"; f.LinkAnchor = "x" },
		"six images":                     func(f *TaskForm) { f.Images = make([]FormImage, models.MaxImagesPerMessage+1) },
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			form := validForm()
			change(&form)

			_, _, err := form.ToNewTask()

			assert.Error(t, err)
		})
	}
}

func TestToUpdateCountsTheStoredImagesItKeeps(t *testing.T) {
	t.Parallel()
	images := make([]FormImage, 3)
	for i := range images {
		images[i] = FormImage{Name: "a.png", MimeType: "image/png", Data: pngBytes}
	}
	form := TaskForm{Title: "t", Prompt: "p", Images: images}

	_, err := form.ToUpdate(3)

	require.Error(t, err, "3 kept and 3 new is more than the 5 a message allows")
	update, err := form.ToUpdate(2)
	require.NoError(t, err, "2 kept and 3 new is exactly the limit")
	assert.Len(t, update.Images, 3)
}

func TestToUpdateRejectsARemovedIdThatIsNotAnIdentifier(t *testing.T) {
	t.Parallel()
	form := TaskForm{Title: "t", Prompt: "p", RemoveIDs: []string{"not-a-uuid"}}

	_, err := form.ToUpdate(0)

	assert.Error(t, err)
}

func TestToUpdateKeepsTheRemovedIdsAndTheBranchChoice(t *testing.T) {
	t.Parallel()
	removed := uuid.NewString()
	form := TaskForm{Title: "t", Prompt: "p", RemoveIDs: []string{removed}, BaseBranch: "main"}

	update, err := form.ToUpdate(0)

	require.NoError(t, err)
	assert.Equal(t, []string{removed}, update.RemoveIDs)
	require.NotNil(t, update.BaseBranch)
	assert.Equal(t, "main", *update.BaseBranch)
}

// multipartRequest builds a multipart form request with the given fields and images.
func multipartRequest(t *testing.T, fields map[string]string, images map[string][]byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, value := range fields {
		require.NoError(t, writer.WriteField(name, value))
	}
	for name, data := range images {
		part, err := writer.CreateFormFile("images", name)
		require.NoError(t, err)
		_, err = part.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	request := httptest.NewRequest(http.MethodPost, "/board/tasks", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func TestParseTaskFormReadsTheFieldsAndSniffsTheImageType(t *testing.T) {
	t.Parallel()
	request := multipartRequest(t,
		map[string]string{"workspace_id": uuid.NewString(), "prompt": "p", "start_now": "on"},
		map[string][]byte{"shot.jpg": pngBytes},
	)

	form, err := ParseTaskForm(httptest.NewRecorder(), request)

	require.NoError(t, err)
	assert.True(t, form.StartNow)
	require.Len(t, form.Images, 1)
	assert.Equal(t, "shot.jpg", form.Images[0].Name)
	assert.Equal(t, "image/png", form.Images[0].MimeType, "the bytes decide the type, not the name")
}

func TestParseTaskFormRefusesAnImageOverTheLimit(t *testing.T) {
	t.Parallel()
	big := make([]byte, models.MaxImageBytes+1)
	request := multipartRequest(t, map[string]string{"prompt": "p"}, map[string][]byte{"huge.png": big})

	_, err := ParseTaskForm(httptest.NewRecorder(), request)

	assert.ErrorContains(t, err, "huge.png")
	assert.ErrorContains(t, err, "4 MiB")
}

func TestParseTaskFormIgnoresAnEmptyFilePart(t *testing.T) {
	t.Parallel()
	request := multipartRequest(t, map[string]string{"prompt": "p"}, map[string][]byte{"": {}})

	form, err := ParseTaskForm(httptest.NewRecorder(), request)

	require.NoError(t, err)
	assert.Empty(t, form.Images)
}

func TestParseTaskFormRejectsANonMultipartBody(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequest(http.MethodPost, "/board/tasks", strings.NewReader("plain"))
	request.Header.Set("Content-Type", "text/plain")

	_, err := ParseTaskForm(httptest.NewRecorder(), request)

	assert.Error(t, err)
}

func TestToNewTaskParsesAgentOptionsWhenAdvancedIsOpen(t *testing.T) {
	t.Parallel()
	form := validForm()
	form.ShowAdvanced = true
	form.AgentRuntime = "claude"
	form.AgentModel = "anthropic/sonnet"
	form.AgentThinking = "high"
	form.AgentProfile = "Claude" // Should be ignored for claude runtime

	input, _, err := form.ToNewTask()
	require.NoError(t, err)
	require.NotNil(t, input.Agent.Runtime)
	assert.Equal(t, "claude", input.Agent.Runtime.String())
	require.NotNil(t, input.Agent.ModelID)
	assert.Equal(t, "sonnet", *input.Agent.ModelID)
	require.NotNil(t, input.Agent.ModelProvider)
	assert.Equal(t, "anthropic", *input.Agent.ModelProvider)
	require.NotNil(t, input.Agent.Thinking)
	assert.Equal(t, "high", *input.Agent.Thinking)
	assert.Nil(t, input.Agent.Profile, "profile must be ignored for claude")
}

func TestToNewTaskIgnoresThinkingAndProfileForCursor(t *testing.T) {
	t.Parallel()
	form := validForm()
	form.ShowAdvanced = true
	form.AgentRuntime = "cursor"
	form.AgentModel = "cursor/agent"
	form.AgentThinking = "high"
	form.AgentProfile = "SomeProfile"

	input, _, err := form.ToNewTask()
	require.NoError(t, err)
	require.NotNil(t, input.Agent.Runtime)
	assert.Equal(t, "cursor", input.Agent.Runtime.String())
	assert.Nil(t, input.Agent.Thinking, "thinking must be ignored for cursor")
	assert.Nil(t, input.Agent.Profile, "profile must be ignored for cursor")
}

func TestToNewTaskPreservesAllOptionsForPi(t *testing.T) {
	t.Parallel()
	form := validForm()
	form.ShowAdvanced = true
	form.AgentRuntime = "pi"
	form.AgentModel = "openai/gpt-4o"
	form.AgentThinking = "low"
	form.AgentProfile = "Gentleman"

	input, _, err := form.ToNewTask()
	require.NoError(t, err)
	require.NotNil(t, input.Agent.Runtime)
	assert.Equal(t, "pi", input.Agent.Runtime.String())
	require.NotNil(t, input.Agent.ModelID)
	assert.Equal(t, "gpt-4o", *input.Agent.ModelID)
	require.NotNil(t, input.Agent.ModelProvider)
	assert.Equal(t, "openai", *input.Agent.ModelProvider)
	require.NotNil(t, input.Agent.Thinking)
	assert.Equal(t, "low", *input.Agent.Thinking)
	require.NotNil(t, input.Agent.Profile)
	assert.Equal(t, "Gentleman", *input.Agent.Profile)
}

func TestToNewTaskIgnoresAgentOptionsWhileAdvancedIsHidden(t *testing.T) {
	t.Parallel()
	form := validForm()
	form.ShowAdvanced = false
	form.AgentRuntime = "claude"
	form.AgentModel = "anthropic/sonnet"
	form.AgentThinking = "high"
	form.AgentProfile = "Claude"

	input, _, err := form.ToNewTask()
	require.NoError(t, err)
	assert.True(t, input.Agent.IsZero(), "agent options must be empty when advanced options are hidden")
}
