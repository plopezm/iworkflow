package state

import (
	"context"
	"fmt"
	"strconv"

	dto "iworkflow/internal/modules/code_hosting/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/code_hosting/presentation/components"
)

// Notice is a confirmation shown above the panel after an action.
type Notice uint8

const (
	// NoNotice shows nothing.
	NoNotice Notice = iota
	// NoticePushed confirms that the branch reached origin.
	NoticePushed
)

// PanelData is what the merge request panel of a task shows after a read or an action.
type PanelData struct {
	TaskID string
	// Title keeps the title typed in the creation form when that action fails.
	Title string
	// Status is the request of the task; nil when it has none or the read failed.
	Status *dto.MergeRequestStatusDTO
	// NoRequest is set when the read succeeded and the task has no request: the form that opens one
	// is shown. A failed read leaves it false, so the panel shows only the error.
	NoRequest bool
	// Err is the message of a failed read or action.
	Err    string
	Notice Notice
	// Confirming shows the merge confirmation instead of the merge button.
	Confirming bool
}

// MergeRequestPanel builds the body of the merge request panel of a task.
func MergeRequestPanel(ctx context.Context, data PanelData) components.TaskMergeRequestBodyProps {
	props := components.TaskMergeRequestBodyProps{
		TaskID:       data.TaskID,
		Title:        msgPanelTitle.T(ctx),
		RefreshLabel: msgRefresh.T(ctx),
		Error:        data.Err,
		Notice:       noticeText(ctx, data.Notice),
	}
	switch {
	case data.Status != nil:
		props.Status = statusProps(ctx, data.TaskID, *data.Status, data.Confirming)
	case data.NoRequest:
		props.Create = &components.CreateRequestProps{
			Intro:      msgNoMRYet.T(ctx),
			TitleLabel: msgTitleOptional.T(ctx),
			Title:      data.Title,
			Note:       msgNoPushNote.T(ctx),
			Action:     msgCreateMR.T(ctx),
		}
	}
	return props
}

func noticeText(ctx context.Context, notice Notice) string {
	if notice == NoticePushed {
		return msgPushDone.T(ctx)
	}
	return ""
}

// statusProps builds the view of an existing request. Actions are offered only while it is open.
func statusProps(ctx context.Context, taskID string, status dto.MergeRequestStatusDTO, confirming bool) *components.MergeRequestStatusProps {
	provider := status.Provider
	request := status.MergeRequest
	abbrev := requestAbbrev(provider)
	open := request.IsOpen()
	return &components.MergeRequestStatusProps{
		TaskID:          taskID,
		Heading:         requestPrefix(provider) + strconv.FormatUint(request.IID, 10) + " " + request.Title,
		Branches:        fmt.Sprintf("%s → %s · %s", request.SourceBranch, request.TargetBranch, status.ProjectPath),
		State:           badge(statusLabel(ctx, request.State), statusVariant(request.State)),
		MergeStatus:     badge(statusLabel(ctx, request.MergeStatus), statusVariant(request.MergeStatus)),
		ShowMergeStatus: open,
		HasConflicts:    request.HasConflicts,
		ConflictLabel:   msgHasConflicts.T(ctx),
		WebURL:          request.WebURL,
		OpenLabel:       msgOpenRequestIn.Fmt(ctx, map[string]string{"abbrev": abbrev, "provider": providerLabel(provider)}),
		Pipeline:        pipelineProps(ctx, provider, status.Pipeline),
		NoPipeline:      msgNoPipeline.T(ctx),
		Open:            open,
		Confirm:         open && confirming,
		ConfirmMessage:  msgMergeConfirm.Fmt(ctx, map[string]string{"abbrev": abbrev, "provider": providerLabel(provider)}),
		MergeLabel:      msgMerge.T(ctx),
		MergeButton:     msgMergeRequestButton.Fmt(ctx, map[string]string{"abbrev": abbrev}),
		PushLabel:       msgPushBranch.T(ctx),
	}
}

// pipelineProps builds the CI run of a request; nil when it has none.
func pipelineProps(ctx context.Context, provider string, pipeline *dto.PipelineDTO) *components.PipelineProps {
	if pipeline == nil {
		return nil
	}
	jobs := make([]components.PipelineJobProps, 0, len(pipeline.Jobs))
	for _, job := range pipeline.Jobs {
		jobs = append(jobs, components.PipelineJobProps{
			Status:     badge(statusLabel(ctx, job.Status), statusVariant(job.Status)),
			Name:       job.Name,
			StageLabel: "(" + job.Stage + ")",
			WebURL:     job.WebURL,
			LogLabel:   "log",
		})
	}
	return &components.PipelineProps{
		Title:     pipelineNoun(provider) + " #" + strconv.FormatUint(pipeline.ID, 10),
		Status:    badge(statusLabel(ctx, pipeline.Status), statusVariant(pipeline.Status)),
		WebURL:    pipeline.WebURL,
		OpenLabel: msgOpenIn.Fmt(ctx, map[string]string{"provider": providerLabel(provider)}),
		Jobs:      jobs,
	}
}

func badge(text string, variant string) components.BadgeProps {
	return components.BadgeProps{Text: text, Variant: variant}
}
