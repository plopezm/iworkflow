package state_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/data_home/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/data_home/presentation/state"
)

func contextIn(locale i18n.Locale) context.Context {
	return i18n.WithState(context.Background(), i18n.State{Locale: locale, Chosen: true})
}

func TestHomeCardShowsTheCurrentFolderAndItsSource(t *testing.T) {
	props := state.HomeCard(contextIn(i18n.En), state.HomeCardData{
		Status: dto.HomeStatusDTO{Current: "/data/iwf", Source: "pointer"},
	})

	require.True(t, props.Loaded)
	require.Equal(t, "/data/iwf", props.Current)
	require.Equal(t, "saved in the per-user settings", props.SourceText)
	require.False(t, props.EnvLocked)
	require.Empty(t, props.Error)
}

func TestHomeCardIsReadOnlyWhenIWFHomeFixesTheFolder(t *testing.T) {
	props := state.HomeCard(contextIn(i18n.En), state.HomeCardData{
		Status: dto.HomeStatusDTO{Current: "/env/iwf", Source: "env", EnvLocked: true},
	})

	require.True(t, props.EnvLocked)
	require.Contains(t, props.EnvNote, "IWF_HOME")
}

func TestHomeCardOnlyShowsTheReadErrorWhenTheFolderCannotBeRead(t *testing.T) {
	props := state.HomeCard(contextIn(i18n.En), state.HomeCardData{
		ReadErr: state.Notice{Kind: state.NoticeFailed, Detail: "pointer is corrupt"},
	})

	require.False(t, props.Loaded)
	require.Contains(t, props.Error, "pointer is corrupt")
}

func TestHomeCardTranslatesEveryTextToTheRequestLanguage(t *testing.T) {
	props := state.HomeCard(contextIn(i18n.Es), state.HomeCardData{
		Status: dto.HomeStatusDTO{Current: "/data/iwf", Source: "default"},
	})

	require.Equal(t, "Carpeta de datos", props.Title)
	require.Equal(t, "carpeta predeterminada", props.SourceText)
}

func TestHomeCardNoticeIsTheTranslatedReasonOfTheRefusal(t *testing.T) {
	props := state.HomeCard(contextIn(i18n.En), state.HomeCardData{
		Status: dto.HomeStatusDTO{Current: "/data/iwf", Source: "default"},
		Notice: state.Notice{Kind: state.NoticeDestination, Reason: "not_empty"},
	})

	require.Equal(t, "The destination must be a new or empty folder.", props.Notice)
}

func TestHomeCardNoticeForAnUnknownReasonKeepsTheDetail(t *testing.T) {
	props := state.HomeCard(contextIn(i18n.En), state.HomeCardData{
		Status: dto.HomeStatusDTO{Current: "/data/iwf", Source: "default"},
		Notice: state.Notice{Kind: state.NoticeFailed, Detail: "disk gone"},
	})

	require.Contains(t, props.Notice, "disk gone")
}

func TestHomeCardDescribesAPendingMoveAndOffersToCancelIt(t *testing.T) {
	props := state.HomeCard(contextIn(i18n.En), state.HomeCardData{
		Status: dto.HomeStatusDTO{
			Current: "/data/iwf",
			Source:  "default",
			Pending: &dto.PendingDTO{From: "/data/iwf", To: "/mnt/iwf"},
		},
	})

	require.Contains(t, props.PendingText, "/mnt/iwf")
	require.Equal(t, "Cancel the pending move", props.CancelLabel)
}

func TestHomeCardReportsTheLastMoveWithItsWarnings(t *testing.T) {
	props := state.HomeCard(contextIn(i18n.En), state.HomeCardData{
		Status: dto.HomeStatusDTO{
			Current: "/mnt/iwf",
			Source:  "pointer",
			Last:    &dto.OutcomeDTO{OK: true, From: "/data/iwf", To: "/mnt/iwf", Warnings: []string{"old trash kept"}},
		},
	})

	require.Contains(t, props.LastText, "/mnt/iwf")
	require.False(t, props.LastFailed)
	require.Equal(t, []string{"old trash kept"}, props.Warnings)
}

func TestHomeCardReportsAFailedLastMoveAsFailed(t *testing.T) {
	props := state.HomeCard(contextIn(i18n.En), state.HomeCardData{
		Status: dto.HomeStatusDTO{
			Current: "/data/iwf",
			Source:  "default",
			Last:    &dto.OutcomeDTO{OK: false, From: "/data/iwf", To: "/mnt/iwf", Error: "no space"},
		},
	})

	require.True(t, props.LastFailed)
	require.Contains(t, props.LastText, "no space")
	require.Contains(t, props.LastText, "The old folder stays in use")
}

func TestHomeCardRestartingHidesTheFormAndSaysSo(t *testing.T) {
	props := state.HomeCard(contextIn(i18n.En), state.HomeCardData{
		Status:     dto.HomeStatusDTO{Current: "/data/iwf", Source: "default"},
		Restarting: true,
	})

	require.True(t, props.Restarting)
	require.NotEmpty(t, props.RestartingText)
}
