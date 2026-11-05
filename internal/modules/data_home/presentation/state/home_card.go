package state

import (
	"context"

	"iworkflow/internal/modules/data_home/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/data_home/presentation/components"
)

// Kinds of notice. The inbound adapter maps the use case errors to them.
const (
	NoticeNone             = ""
	NoticeEnvLocked        = "env_locked"
	NoticeActiveTasks      = "active_tasks"
	NoticePending          = "pending"
	NoticeDestination      = "destination"
	NoticeUnavailable      = "unavailable"
	NoticeFailed           = "failed"
	sourceEnv              = "env"
	sourcePointer          = "pointer"
	destinationPlaceholder = "/path/to/new/folder"
)

// Notice is a refusal or failure to show above the card. Reason names the refused destination reason
// for NoticeDestination; Detail is the text of an unexpected failure.
type Notice struct {
	Kind   string
	Reason string
	Detail string
}

// HomeCardData is what the card handler read: the status, a read failure that replaces the card, the
// notice of the last action and whether the server is restarting for a move.
type HomeCardData struct {
	Status     dto.HomeStatusDTO
	ReadErr    Notice
	Notice     Notice
	Restarting bool
}

// HomeCard builds the props of the data folder card. Every text is translated to the language of ctx.
func HomeCard(ctx context.Context, data HomeCardData) components.DataHomeCardProps {
	props := components.DataHomeCardProps{
		Title:                  msgTitle.T(ctx),
		Help:                   msgHelp.T(ctx),
		CurrentLabel:           msgCurrentFolder.T(ctx),
		DestinationLabel:       msgDestinationLabel.T(ctx),
		DestinationPlaceholder: destinationPlaceholder,
		MoveLabel:              msgMove.T(ctx),
		CancelLabel:            msgCancel.T(ctx),
		RestartingText:         msgRestarting.T(ctx),
		Restarting:             data.Restarting,
		Notice:                 noticeText(ctx, data.Notice),
	}
	if data.ReadErr.Kind != NoticeNone {
		props.Error = readErrorText(ctx, data.ReadErr)
		return props
	}
	props.Loaded = true
	props.Current = data.Status.Current
	props.SourceText = sourceText(ctx, data.Status.Source)
	props.EnvLocked = data.Status.EnvLocked
	if props.EnvLocked {
		props.EnvNote = msgEnvNote.T(ctx)
	}
	if pending := data.Status.Pending; pending != nil {
		props.PendingText = msgPending.Fmt(ctx, map[string]string{"to": pending.To})
	}
	if last := data.Status.Last; last != nil {
		props.LastFailed = !last.OK
		if last.OK {
			props.LastText = msgLastOK.Fmt(ctx, map[string]string{"from": last.From, "to": last.To})
		} else {
			props.LastText = msgLastFailed.Fmt(ctx, map[string]string{"error": last.Error})
		}
		props.Warnings = last.Warnings
		if len(last.Warnings) > 0 {
			props.WarningsLabel = msgWarnings.T(ctx)
		}
	}
	return props
}

// sourceText says which rule picked the folder.
func sourceText(ctx context.Context, source string) string {
	switch source {
	case sourceEnv:
		return msgSourceEnv.T(ctx)
	case sourcePointer:
		return msgSourcePointer.T(ctx)
	default:
		return msgSourceDefault.T(ctx)
	}
}

// noticeText is the translated text of a notice, or "" when there is none.
func noticeText(ctx context.Context, notice Notice) string {
	switch notice.Kind {
	case NoticeNone:
		return ""
	case NoticeEnvLocked:
		return msgNoticeEnvLocked.T(ctx)
	case NoticeActiveTasks:
		return msgNoticeActiveTasks.T(ctx)
	case NoticePending:
		return msgNoticePending.T(ctx)
	case NoticeUnavailable:
		return msgUnavailable.T(ctx)
	case NoticeDestination:
		if reason, ok := reasonTexts[notice.Reason]; ok {
			return reason.T(ctx)
		}
		return msgNoticeFailed.Fmt(ctx, map[string]string{"error": notice.Reason})
	default:
		return msgNoticeFailed.Fmt(ctx, map[string]string{"error": notice.Detail})
	}
}

// readErrorText is the text that replaces the card when the folder cannot be read.
func readErrorText(ctx context.Context, notice Notice) string {
	if notice.Kind == NoticeUnavailable {
		return msgUnavailable.T(ctx)
	}
	return msgReadFailed.Fmt(ctx, map[string]string{"error": notice.Detail})
}
