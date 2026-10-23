package models_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	agentserrors "iworkflow/internal/modules/agents_files/domain/errors"
	"iworkflow/internal/modules/agents_files/domain/limits"
	"iworkflow/internal/modules/agents_files/domain/models"
)

func input(description string, content []byte) models.NewAgentsFile {
	return models.NewAgentsFile{Description: description, Content: content}
}

func TestValidatedTrimsTheDescription(t *testing.T) {
	result, err := input("  reglas  ", []byte("# hola")).Validated()

	require.NoError(t, err)
	require.Equal(t, "reglas", result.Description)
}

func TestValidatedRejectsBlankOrLongDescription(t *testing.T) {
	tests := []struct {
		name        string
		description string
		valid       bool
	}{
		{"blank", "   ", false},
		{"one rune over the limit", strings.Repeat("a", limits.MaxDescriptionChars+1), false},
		{"exactly the limit, counted in runes", strings.Repeat("ñ", limits.MaxDescriptionChars), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := input(tc.description, []byte("x")).Validated()

			if tc.valid {
				require.NoError(t, err)
				return
			}
			require.Equal(t, agentserrors.InvalidDescription(), err)
		})
	}
}

func TestValidatedRejectsEmptyContent(t *testing.T) {
	_, err := input("d", []byte{}).Validated()

	require.Equal(t, agentserrors.EmptyContent(), err)
}

func TestValidatedRejectsOversizedContent(t *testing.T) {
	_, err := input("d", bytes.Repeat([]byte("a"), limits.MaxContentBytes+1)).Validated()
	require.Equal(t, agentserrors.TooLarge(), err)

	_, err = input("d", bytes.Repeat([]byte("a"), limits.MaxContentBytes)).Validated()
	require.NoError(t, err)
}

func TestValidatedRejectsNonUTF8Content(t *testing.T) {
	_, err := input("d", []byte{0xff, 0xfe, 0x00}).Validated()

	require.Equal(t, agentserrors.NotUTF8(), err)
}

func TestValidatedReportsTheFirstFailingCheckInOrder(t *testing.T) {
	// A blank description wins over empty content, and empty content wins over bad UTF-8.
	_, err := input(" ", []byte{}).Validated()
	require.Equal(t, agentserrors.InvalidDescription(), err)

	_, err = input("d", []byte{}).Validated()
	require.Equal(t, agentserrors.EmptyContent(), err)
}

func TestParseAgentsFileIDRejectsGarbage(t *testing.T) {
	_, err := models.ParseAgentsFileID("nope")

	require.Equal(t, agentserrors.InvalidID("nope"), err)
}

func TestParseAgentsFileIDRoundTripsTheCanonicalForm(t *testing.T) {
	id, err := models.NewAgentsFileID()
	require.NoError(t, err)

	parsed, err := models.ParseAgentsFileID(id.String())

	require.NoError(t, err)
	require.Equal(t, id, parsed)
}

// These texts must stay byte-identical to the Rust Display implementation (parity).
func TestErrorMessagesMatchTheRustTexts(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"invalid description", agentserrors.InvalidDescription(), "la descripción es obligatoria (máximo 200 caracteres)"},
		{"empty content", agentserrors.EmptyContent(), "el fichero está vacío"},
		{"not utf8", agentserrors.NotUTF8(), "el fichero no es texto UTF-8 válido"},
		{"too large", agentserrors.TooLarge(), "el fichero supera el tamaño máximo de 512 KiB"},
		{"invalid id", agentserrors.InvalidID("x"), "identificador de fichero AGENTS.md inválido: x"},
		{"not found", agentserrors.NotFound("x"), "fichero AGENTS.md no encontrado: x"},
		{"project not found", agentserrors.ProjectNotFound("p"), "proyecto no encontrado: p"},
		{"target exists", agentserrors.TargetExists("/a/AGENTS.md"), "ya existe un AGENTS.md en /a/AGENTS.md"},
		{"storage", agentserrors.Storage("d"), "error de almacenamiento: d"},
		{"repository", agentserrors.Repository("d"), "error de persistencia: d"},
		{"install", agentserrors.Install("d"), "error al instalar el fichero: d"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.EqualError(t, tc.err, tc.want)
		})
	}
}

func TestErrorsOfTheSameKindMatchRegardlessOfDetail(t *testing.T) {
	require.ErrorIs(t, agentserrors.NotFound("a"), agentserrors.NotFound("b"))
	require.NotErrorIs(t, agentserrors.NotFound("a"), agentserrors.Storage("a"))
}
