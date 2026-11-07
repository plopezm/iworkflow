package components

import (
	"context"
	"fmt"
	"strconv"
	"time"

	ui "iworkflow/internal/core/components"
	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/presentation/state"
)

// The keys of the editor options and of the labels. They are the values the form submits, so they are the domain
// names: one source for the option values, the label maps and the switches.
const (
	eventStart  = string(models.EventTaskStart)
	eventFinish = string(models.EventTaskFinish)
	phasePre    = string(models.PhasePre)
	phasePost   = string(models.PhasePost)
	kindScript  = string(models.KindScript)
	kindAgent   = string(models.KindAgent)
	failBlock   = string(models.FailureBlock)
	failWarn    = string(models.FailureWarn)
	failIgnore  = string(models.FailureIgnore)
)

// eventTitle is the heading of the group of an event.
func eventTitle(ctx context.Context, event string) string {
	if event == eventFinish {
		return msgEventFinish.T(ctx)
	}
	return msgEventStart.T(ctx)
}

// kindText names the kind of a hook.
func kindText(ctx context.Context, kind string) string {
	if kind == kindAgent {
		return msgKindAgent.T(ctx)
	}
	return msgKindScript.T(ctx)
}

// failureText says what a failure does, as the hook row shows it.
func failureText(ctx context.Context, policy string) string {
	switch policy {
	case failWarn:
		return msgFailWarn.T(ctx)
	case failIgnore:
		return msgFailIgnore.T(ctx)
	default:
		return msgFailBlock.T(ctx)
	}
}

// noticeText is the message of the last action.
func noticeText(ctx context.Context, notice state.Notice) string {
	switch notice {
	case state.NoticeUpdated:
		return msgNoticeUpd.T(ctx)
	case state.NoticeDeleted:
		return msgNoticeDel.T(ctx)
	case state.NoticeReordered:
		return msgNoticeOrder.T(ctx)
	default:
		return msgNoticeSaved.T(ctx)
	}
}

// resultText is the status of a test run.
func resultText(ctx context.Context, status string) string {
	switch status {
	case state.ResultOK:
		return msgResultOK.T(ctx)
	case state.ResultFailed:
		return msgResultFailed.T(ctx)
	case state.ResultTimeout:
		return msgResultTO.T(ctx)
	default:
		return msgResultError.T(ctx)
	}
}

// resultLine is the status of a test with its exit code, when there is one.
func resultLine(ctx context.Context, result *state.TestResult) string {
	if exit := exitText(ctx, result); exit != "" {
		return resultText(ctx, result.Status) + " · " + exit
	}
	return resultText(ctx, result.Status)
}

// exitText is the exit code of a script test; an agent has none to show.
func exitText(ctx context.Context, result *state.TestResult) string {
	if result.ExitCode < 0 {
		return ""
	}
	return msgExitCode.T(ctx) + " " + strconv.Itoa(result.ExitCode)
}

// outputText is the captured output, or a placeholder when there is none.
func outputText(ctx context.Context, output string) string {
	if output == "" {
		return msgNoOutput.T(ctx)
	}
	return output
}

// editorTitle names the editor: a new hook or the one being edited.
func editorTitle(ctx context.Context, id string) string {
	if id == "" {
		return msgFormNew.T(ctx)
	}
	return msgFormEdit.T(ctx)
}

// timeoutHint says the timeout limits of the editor: whole seconds, at most the domain maximum.
func timeoutHint(ctx context.Context) string {
	return fmt.Sprintf(msgTimeoutHint.T(ctx), int(models.MaxTimeout/time.Second))
}

// timeoutLabel is the timeout of a row in seconds.
func timeoutLabel(ctx context.Context, seconds string) string {
	return msgTimeout.T(ctx) + ": " + seconds + " s"
}

func eventOptions(ctx context.Context) []ui.FormOption {
	return []ui.FormOption{
		{Value: eventStart, Label: msgEventStart.T(ctx)},
		{Value: eventFinish, Label: msgEventFinish.T(ctx)},
	}
}

func phaseOptions(ctx context.Context) []ui.FormOption {
	return []ui.FormOption{
		{Value: phasePre, Label: msgBefore.T(ctx)},
		{Value: phasePost, Label: msgAfter.T(ctx)},
	}
}

func kindOptions(ctx context.Context) []ui.FormOption {
	return []ui.FormOption{
		{Value: kindScript, Label: msgKindScript.T(ctx)},
		{Value: kindAgent, Label: msgKindAgent.T(ctx)},
	}
}

func failureOptions(ctx context.Context) []ui.FormOption {
	return []ui.FormOption{
		{Value: failBlock, Label: msgFailBlock.T(ctx)},
		{Value: failWarn, Label: msgFailWarn.T(ctx)},
		{Value: failIgnore, Label: msgFailIgnore.T(ctx)},
	}
}

// kindClass is the look of a kind button: the chosen kind is the filled one.
func kindClass(chosen bool) string {
	if chosen {
		return "btn-primary"
	}
	return "btn-secondary"
}

// pressedText is the aria-pressed value of a kind button.
func pressedText(chosen bool) string {
	if chosen {
		return "true"
	}
	return "false"
}
