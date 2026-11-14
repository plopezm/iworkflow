package components

import (
	"context"

	shared "iworkflow/internal/core/components"
)

// phaseStart marks the steps that only run when the task is started by the create (the "Start now" box is checked).
const phaseStart = "start"

// CreateProgress is the panel of the create form. The start steps are hidden by the script when "Start now" is off.
func CreateProgress(ctx context.Context) shared.SubmitProgressProps {
	return shared.SubmitProgressProps{
		Title: msgSavingTask.T(ctx),
		Steps: []shared.ProgressStep{
			{Label: msgStepSave.T(ctx)},
			{Label: msgStepWorktree.T(ctx), Phase: phaseStart},
			{Label: msgStepHooks.T(ctx), Phase: phaseStart},
			{Label: msgStepAgent.T(ctx), Phase: phaseStart},
		},
		SlowHint: msgSubmitSlow.T(ctx),
	}
}

// EditProgress is the panel of the edit form.
func EditProgress(ctx context.Context) shared.SubmitProgressProps {
	return shared.SubmitProgressProps{
		Title: msgSavingChanges.T(ctx),
		Steps: []shared.ProgressStep{
			{Label: msgStepSave.T(ctx)},
			{Label: msgStepImages.T(ctx)},
			{Label: msgStepBoard.T(ctx)},
		},
		SlowHint: msgSubmitSlow.T(ctx),
	}
}
