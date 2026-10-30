package dto

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestParseWorkspaceIDAcceptsAUUIDAndRejectsGarbage(t *testing.T) {
	id := uuid.New()

	got, err := ParseWorkspaceID(id.String())

	require.NoError(t, err)
	require.Equal(t, id, got)
	_, err = ParseWorkspaceID("nope")
	require.ErrorIs(t, err, ErrInvalidWorkspaceID)
	require.EqualError(t, err, "id de workspace inválido")
}

func TestParseTaskIDRejectsGarbage(t *testing.T) {
	_, err := ParseTaskID("nope")

	require.ErrorIs(t, err, ErrInvalidTaskID)
	require.EqualError(t, err, "id de tarea inválido")
}

func TestParseSessionIDAcceptsAUUIDAndRejectsGarbage(t *testing.T) {
	id := uuid.New()

	got, err := ParseSessionID(id.String())

	require.NoError(t, err)
	require.Equal(t, id, got.UUID())
	_, err = ParseSessionID("nope")
	require.ErrorIs(t, err, ErrInvalidSessionID)
	require.EqualError(t, err, "id de sesión inválido")
}
