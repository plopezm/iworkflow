package models_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

func TestTaskPriorityRoundTripsThroughString(t *testing.T) {
	t.Parallel()
	for _, priority := range models.AllTaskPriorities {
		parsed, err := models.ParseTaskPriority(string(priority))
		require.NoError(t, err)
		assert.Equal(t, priority, parsed)
	}
}

func TestTaskPriorityRejectsUnknownValue(t *testing.T) {
	t.Parallel()
	_, err := models.ParseTaskPriority("critical")
	assert.ErrorIs(t, err, werrors.InvalidPriority("critical"))
}

func TestTaskPriorityDefaultsToNormal(t *testing.T) {
	t.Parallel()
	assert.Equal(t, models.PriorityNormal, models.DefaultTaskPriority)
}

func TestTaskPriorityRankIncreasesWithUrgency(t *testing.T) {
	t.Parallel()
	for index := 1; index < len(models.AllTaskPriorities); index++ {
		assert.Less(t, models.AllTaskPriorities[index-1].Rank(), models.AllTaskPriorities[index].Rank())
	}
}

func TestPendingDialogWithoutTimeoutNeverExpires(t *testing.T) {
	t.Parallel()
	dialog := models.NewPendingDialog("d1", nil)
	assert.False(t, dialog.IsExpiredAt(time.Now().Add(24*time.Hour)))
}

func TestPendingDialogExpiresAtOrAfterItsDeadline(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	dialog := models.NewPendingDialog("d1", &now)
	assert.False(t, dialog.IsExpiredAt(now.Add(-time.Millisecond)))
	assert.True(t, dialog.IsExpiredAt(now))
	assert.True(t, dialog.IsExpiredAt(now.Add(time.Millisecond)))
}

func TestSubagentActivityDefaultIsNotWorking(t *testing.T) {
	t.Parallel()
	assert.False(t, models.SubagentActivity{}.IsWorking())
}

func TestActiveSubagentsMeanWorking(t *testing.T) {
	t.Parallel()
	assert.True(t, models.NewSubagentActivity(1, 0).IsWorking())
	assert.False(t, models.NewSubagentActivity(0, 2).IsWorking())
}

func TestSubagentStatusOnlyQueuedRunningAndWaitingAreActive(t *testing.T) {
	t.Parallel()
	assert.True(t, models.SubagentQueued.IsActive())
	assert.True(t, models.SubagentRunning.IsActive())
	assert.True(t, models.SubagentWaiting.IsActive())
	for _, status := range []models.SubagentStatus{
		models.SubagentCompleted, models.SubagentFailed, models.SubagentCancelled,
		models.SubagentTimedOut, models.SubagentUnknown,
	} {
		assert.False(t, status.IsActive(), string(status))
	}
}

func TestResourceLimitsDefaultIsUnlimited(t *testing.T) {
	t.Parallel()
	assert.True(t, models.ResourceLimits{}.IsUnlimited())
}

func TestSystemdPropertiesCoverDefinedResourcesOnly(t *testing.T) {
	t.Parallel()
	memory := uint32(2048)
	assert.Equal(t, []string{"MemoryMax=2048M"}, models.ResourceLimits{MemoryMB: &memory}.SystemdProperties())

	cpuMemory := uint32(512)
	cpu := uint32(150)
	assert.Equal(t, []string{"MemoryMax=512M", "CPUQuota=150%"},
		models.ResourceLimits{MemoryMB: &cpuMemory, CPUPercent: &cpu}.SystemdProperties())
}

func TestZeroLimitsAreInvalid(t *testing.T) {
	t.Parallel()
	zero := uint32(0)
	_, err := models.ResourceLimits{MemoryMB: &zero}.Validated()
	assert.ErrorIs(t, err, werrors.InvalidLimits())
	_, err = models.ResourceLimits{CPUPercent: &zero}.Validated()
	assert.ErrorIs(t, err, werrors.InvalidLimits())
	_, err = models.ResourceLimits{}.Validated()
	assert.NoError(t, err)
}

func TestOrPrefersOwnValuesPerResource(t *testing.T) {
	t.Parallel()
	memory := uint32(1024)
	global := uint32(4096)
	cpu := uint32(200)
	task := models.ResourceLimits{MemoryMB: &memory}
	fallback := models.ResourceLimits{MemoryMB: &global, CPUPercent: &cpu}
	merged := task.Or(fallback)
	require.NotNil(t, merged.MemoryMB)
	require.NotNil(t, merged.CPUPercent)
	assert.Equal(t, uint32(1024), *merged.MemoryMB)
	assert.Equal(t, uint32(200), *merged.CPUPercent)
}

func TestUsageLimitDetectsProviderMessages(t *testing.T) {
	t.Parallel()
	for _, text := range []string{
		"You've hit your session limit · resets 3:45pm",
		"You've hit your weekly limit · resets Mon 12:00am",
		"claude -p: You're out of extra usage · resets 6:40pm",
		"Claude AI usage limit reached|1784283600",
		`429 {"error":{"type":"rate_limit_error"}}`,
		"You exceeded your current quota",
		"Error: quota exceeded for this account",
		"your available balance is too low",
	} {
		assert.True(t, models.LooksLikeUsageLimit(text), text)
	}
}

func TestUsageLimitIgnoresUnrelatedErrors(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"error_max_turns", "boom", "command not found", "limit"} {
		assert.False(t, models.LooksLikeUsageLimit(text), text)
	}
}

var usageNow = time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

func TestKnownFutureResetIsUsedWithMargin(t *testing.T) {
	t.Parallel()
	reset := usageNow.Add(3 * time.Hour)
	state := models.UsageLimitState{}.AfterHit(&reset, usageNow)
	require.NotNil(t, state.ResumeAt)
	assert.Equal(t, reset.Add(time.Minute), *state.ResumeAt)
	assert.Equal(t, uint32(1), state.Attempts)
}

func TestUnknownOrPastResetBacksOffExponentiallyUpToTheCap(t *testing.T) {
	t.Parallel()
	state := models.UsageLimitState{}
	var waits []int64
	for range 6 {
		state = state.AfterHit(nil, usageNow)
		waits = append(waits, int64(state.ResumeAt.Sub(usageNow).Minutes()))
	}
	assert.Equal(t, []int64{5, 10, 20, 40, 60, 60}, waits)

	past := usageNow.Add(-time.Minute)
	pastState := models.UsageLimitState{}.AfterHit(&past, usageNow)
	require.NotNil(t, pastState.ResumeAt)
	assert.Equal(t, usageNow.Add(5*time.Minute), *pastState.ResumeAt)
}

func TestIsDueOnlyOnceTheResumeTimeHasPassed(t *testing.T) {
	t.Parallel()
	state := models.UsageLimitState{}.AfterHit(nil, usageNow)
	assert.False(t, state.IsDue(usageNow))
	assert.True(t, state.IsDue(usageNow.Add(5*time.Minute)))
	assert.False(t, models.UsageLimitState{}.IsDue(usageNow))
}

func TestEpochSecondsConvertToDates(t *testing.T) {
	t.Parallel()
	value, ok := models.FromEpochSecs(0)
	assert.True(t, ok)
	assert.Equal(t, time.Unix(0, 0).UTC(), value)
}

func TestRuntimeIDParsesSlugs(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"pi", "gentle-shell", "agent2"} {
		id, err := models.ParseRuntimeID(raw)
		require.NoError(t, err)
		assert.Equal(t, raw, id.String())
	}
}

func TestRuntimeIDRejectsInvalidValues(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"", "Pi", "a b", "-pi", "pi-", "pi/x"} {
		_, err := models.ParseRuntimeID(raw)
		assert.ErrorIs(t, err, werrors.InvalidRuntime(raw), raw)
	}
}

func TestAgentModeParseAndStringRoundTrip(t *testing.T) {
	t.Parallel()
	for _, mode := range []models.AgentMode{models.AgentModeAgent, models.AgentModePlan} {
		parsed, ok := models.ParseAgentMode(string(mode))
		assert.True(t, ok)
		assert.Equal(t, mode, parsed)
	}
	_, ok := models.ParseAgentMode("bypass")
	assert.False(t, ok)
}

func TestImageAttachmentLimits(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 4*1024*1024, models.MaxImageBytes)
	for _, mime := range []string{"image/png", "image/jpeg", "image/gif", "image/webp"} {
		_, err := models.ImageAttachment{Name: "a", MimeType: mime, Data: "aGk="}.ValidatedImage()
		assert.NoError(t, err, mime)
	}
	_, err := models.ImageAttachment{Name: "a", MimeType: "image/bmp", Data: "aGk="}.ValidatedImage()
	assert.ErrorIs(t, err, werrors.InvalidAttachment(""))
	_, err = models.ImageAttachment{Name: "a", MimeType: "image/png", Data: ""}.ValidatedImage()
	assert.ErrorIs(t, err, werrors.InvalidAttachment(""))
}

func TestImageAttachmentEnforcesDecodedSizeLimit(t *testing.T) {
	t.Parallel()
	// Four base64 characters decode to three bytes: this payload decodes to exactly the limit.
	limitData := repeatString("AAAA", models.MaxImageBytes/3)
	_, err := models.ImageAttachment{Name: "a", MimeType: "image/png", Data: limitData}.ValidatedImage()
	assert.NoError(t, err)
	_, err = models.ImageAttachment{Name: "a", MimeType: "image/png", Data: limitData + "AAAA"}.ValidatedImage()
	assert.ErrorIs(t, err, werrors.InvalidAttachment(""))
}

func TestImageAttachmentEnforcesMaxImagesPerMessage(t *testing.T) {
	t.Parallel()
	image := models.ImageAttachment{Name: "a", MimeType: "image/png", Data: "aGk="}
	six := []models.ImageAttachment{image, image, image, image, image, image}
	_, err := models.ValidateAllImages(six)
	assert.ErrorIs(t, err, werrors.InvalidAttachment(""))
	five, err := models.ValidateAllImages(six[:5])
	assert.NoError(t, err)
	assert.Len(t, five, 5)
}

func TestLaunchOptionsTaskLimitsOverrideGlobalLimitsPerResource(t *testing.T) {
	t.Parallel()
	globalMemory := uint32(4096)
	globalCPU := uint32(200)
	taskMemory := uint32(1024)
	global := models.LaunchOptions{Limits: models.ResourceLimits{MemoryMB: &globalMemory, CPUPercent: &globalCPU}}
	merged := global.WithTaskLimits(models.ResourceLimits{MemoryMB: &taskMemory}).Limits
	require.NotNil(t, merged.MemoryMB)
	require.NotNil(t, merged.CPUPercent)
	assert.Equal(t, uint32(1024), *merged.MemoryMB)
	assert.Equal(t, uint32(200), *merged.CPUPercent)
}

func TestLaunchOptionsDisabledSwitchDropsGlobalAndTaskLimits(t *testing.T) {
	t.Parallel()
	memory := uint32(4096)
	taskMemory := uint32(1024)
	taskCPU := uint32(50)
	options := models.LaunchOptions{Limits: models.ResourceLimits{MemoryMB: &memory}, LimitsDisabled: true}
	assert.True(t, options.WithTaskLimits(models.ResourceLimits{MemoryMB: &taskMemory, CPUPercent: &taskCPU}).Limits.IsUnlimited())
}

func TestLaunchOptionsFullCapabilitiesKeepEverything(t *testing.T) {
	t.Parallel()
	options := fullLaunchOptions(t)
	assert.Equal(t, options, options.RestrictedTo(models.FullAgentCapabilities()))
}

func TestLaunchOptionsMinimalCapabilitiesDropModelAndThinkingButKeepRuntime(t *testing.T) {
	t.Parallel()
	restricted := fullLaunchOptions(t).RestrictedTo(models.MinimalAgentCapabilities())
	require.NotNil(t, restricted.Runtime)
	assert.Equal(t, "pi", restricted.Runtime.String())
	assert.Nil(t, restricted.Model)
	assert.Nil(t, restricted.Thinking)
}

func TestLaunchOptionsMinimalCapabilitiesKeepTheRelatedProjectDirectories(t *testing.T) {
	t.Parallel()
	options := models.LaunchOptions{AddDirs: []string{"/repos/api", "/repos/web"}}
	assert.Equal(t, []string{"/repos/api", "/repos/web"}, options.RestrictedTo(models.MinimalAgentCapabilities()).AddDirs)
}

func fullLaunchOptions(t *testing.T) models.LaunchOptions {
	t.Helper()
	runtime, err := models.ParseRuntimeID("pi")
	require.NoError(t, err)
	thinking := "high"
	return models.LaunchOptions{
		Runtime:  &runtime,
		Model:    &models.LaunchModel{Provider: "anthropic", Model: "claude-5"},
		Thinking: &thinking,
	}
}

func repeatString(text string, times int) string {
	result := make([]byte, 0, len(text)*times)
	for range times {
		result = append(result, text...)
	}
	return string(result)
}

func TestNewTaskWithoutPriorityUsesTheDefault(t *testing.T) {
	t.Parallel()
	task, err := models.NewTask{WorkspaceID: "ws", Title: "t", Prompt: "p"}.Validated()
	require.NoError(t, err)
	assert.Equal(t, models.DefaultTaskPriority, task.Priority)
}
