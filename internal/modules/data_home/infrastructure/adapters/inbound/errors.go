package inbound

import (
	"errors"

	domainerrors "iworkflow/internal/modules/data_home/domain/errors"
	"iworkflow/internal/modules/data_home/presentation/state"
)

// NoticeOf maps an error of the use cases to the notice the card shows. The kind selects the
// translated text in the presentation; the detail is kept only for unexpected failures.
func NoticeOf(err error) state.Notice {
	var destination *domainerrors.DestinationError
	switch {
	case errors.Is(err, domainerrors.ErrEnvLocked):
		return state.Notice{Kind: state.NoticeEnvLocked}
	case errors.Is(err, domainerrors.ErrActiveTasks):
		return state.Notice{Kind: state.NoticeActiveTasks}
	case errors.Is(err, domainerrors.ErrMovePending):
		return state.Notice{Kind: state.NoticePending}
	case errors.As(err, &destination):
		return state.Notice{Kind: state.NoticeDestination, Reason: string(destination.Reason)}
	case errors.Is(err, domainerrors.ErrUnavailable):
		return state.Notice{Kind: state.NoticeUnavailable}
	default:
		return state.Notice{Kind: state.NoticeFailed, Detail: err.Error()}
	}
}
