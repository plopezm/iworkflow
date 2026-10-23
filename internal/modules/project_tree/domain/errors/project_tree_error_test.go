package errors_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	treeerrors "iworkflow/internal/modules/project_tree/domain/errors"
)

// The texts are user-visible and must match the Rust version byte-for-byte.
func TestErrorTextsKeepTheRustWording(t *testing.T) {
	require.Equal(t, "el workspace no existe", treeerrors.WorkspaceNotFound().Error())
	require.Equal(t, "ruta inválida: ../x", treeerrors.InvalidPath("../x").Error())
	require.Equal(t, "la ruta no existe: a", treeerrors.PathNotFound("a").Error())
	require.Equal(t, "la ruta no es un directorio: a", treeerrors.NotADirectory("a").Error())
	require.Equal(t, "la ruta no es un fichero: a", treeerrors.NotAFile("a").Error())
	require.Equal(t, "el fichero supera el tamaño máximo editable: a", treeerrors.FileTooLarge("a").Error())
	require.Equal(t, "el fichero no es texto UTF-8: a", treeerrors.NotText("a").Error())
	require.Equal(t, "la ruta queda fuera del proyecto: a", treeerrors.PathOutsideRoot("a").Error())
	require.Equal(t, "el mensaje del commit no puede estar vacío", treeerrors.EmptyCommitMessage().Error())
	require.Equal(t, "no se pudo hacer commit: nada", treeerrors.CommitFailed("nada").Error())
	require.Equal(t, "error de lectura del directorio: boom", treeerrors.IO("boom").Error())
	require.Equal(t, "error al buscar el workspace: boom", treeerrors.WorkspaceLookup("boom").Error())
}

func TestErrorsMatchByKindWhateverTheDetail(t *testing.T) {
	err := error(treeerrors.InvalidPath("../x"))

	require.True(t, errors.Is(err, treeerrors.InvalidPath("")))
	require.False(t, errors.Is(err, treeerrors.PathNotFound("")))
	require.Equal(t, treeerrors.InvalidPath("../x"), err)
	require.NotEqual(t, treeerrors.InvalidPath("other"), err)
}
