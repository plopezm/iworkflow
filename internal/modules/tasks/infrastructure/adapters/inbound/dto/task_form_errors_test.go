package dto

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/domain/models"
)

// textIn is the text of a form error in one language.
func textIn(locale i18n.Locale, err error) string {
	var formErr *FormError
	if !errors.As(err, &formErr) {
		return ""
	}
	return formErr.Text(i18n.WithState(context.Background(), i18n.State{Locale: locale}))
}

func TestBoundaryErrorsAreTranslatedIntoEveryLanguage(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		produce func() error
		english string
		spanish string
	}{
		"workspace is not an identifier": {
			produce: func() error { f := validForm(); f.WorkspaceID = "x"; _, _, err := f.ToNewTask(); return err },
			english: "The workspace is not valid", spanish: "el workspace no es válido",
		},
		"memory is not a number": {
			produce: func() error { f := validForm(); f.MemoryLimit = "abc"; _, _, err := f.ToNewTask(); return err },
			english: "The maximum RAM must be a whole number greater than zero", spanish: "la RAM máxima debe ser un número entero mayor que cero",
		},
		"cpu is zero": {
			produce: func() error { f := validForm(); f.CPULimit = "0"; _, _, err := f.ToNewTask(); return err },
			english: "The maximum CPU must be a whole number greater than zero", spanish: "la CPU máxima debe ser un número entero mayor que cero",
		},
		"branch reads as a git option": {
			produce: func() error { f := validForm(); f.BaseBranch = "--x"; _, _, err := f.ToNewTask(); return err },
			english: "The base branch is not valid", spanish: "la rama base no es válida",
		},
		"branch is too long": {
			produce: func() error {
				f := validForm()
				f.BaseBranch = strings.Repeat("a", MaxBaseBranchLength+1)
				_, _, err := f.ToNewTask()
				return err
			},
			english: "The base branch exceeds 255 characters", spanish: "la rama base supera los 255 caracteres",
		},
		"link anchor is not an identifier": {
			produce: func() error {
				f := validForm()
				f.LinkKind = "next"
				f.LinkAnchor = "x"
				_, _, err := f.ToNewTask()
				return err
			},
			english: "The linked task is not valid", spanish: "la tarea enlazada no es válida",
		},
		"link kind is unknown": {
			produce: func() error {
				f := validForm()
				f.LinkKind = "sideways"
				f.LinkAnchor = uuid.NewString()
				_, _, err := f.ToNewTask()
				return err
			},
			english: "The link type is not valid", spanish: "el tipo de vínculo no es válido",
		},
		"too many images": {
			produce: func() error {
				f := validForm()
				f.Images = make([]FormImage, models.MaxImagesPerMessage+1)
				_, _, err := f.ToNewTask()
				return err
			},
			english: "A message admits at most 5 images", spanish: "un mensaje admite como máximo 5 imágenes",
		},
		"removed image is not an identifier": {
			produce: func() error { f := validForm(); f.RemoveIDs = []string{"x"}; _, err := f.ToUpdate(0); return err },
			english: "An image to remove is not valid", spanish: "una imagen a quitar no es válida",
		},
		"image over the limit": {
			produce: func() error {
				request := multipartRequest(t, map[string]string{"prompt": "p"}, map[string][]byte{"huge.png": make([]byte, models.MaxImageBytes+1)})
				_, err := ParseTaskForm(httptest.NewRecorder(), request)
				return err
			},
			english: "huge.png: the image exceeds 4 MiB", spanish: "huge.png: la imagen supera los 4 MiB",
		},
		"body is not a form": {
			produce: func() error {
				request := httptest.NewRequest(http.MethodPost, "/board/tasks", strings.NewReader("plain"))
				request.Header.Set("Content-Type", "text/plain")
				_, err := ParseTaskForm(httptest.NewRecorder(), request)
				return err
			},
			english: "The form is not valid", spanish: "el formulario no es válido",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			err := test.produce()
			require.Error(t, err)

			assert.Equal(t, test.english, textIn(i18n.En, err))
			assert.Equal(t, test.spanish, textIn(i18n.Es, err))
			assert.Equal(t, test.spanish, err.Error(), "the plain error text keeps the Spanish Rust wording")
			for _, locale := range i18n.All {
				assert.NotEmpty(t, textIn(locale, err), "missing %s text", locale)
			}
			for _, locale := range []i18n.Locale{i18n.Zh, i18n.De, i18n.Fr} {
				assert.NotEqual(t, test.english, textIn(locale, err), "%s is still the English text", locale)
				assert.NotEqual(t, test.spanish, textIn(locale, err), "%s is still the Spanish text", locale)
			}
		})
	}
}

func TestOversizedFormIsAFormErrorThatCallersCanMatch(t *testing.T) {
	t.Parallel()
	assert.ErrorIs(t, ErrFormTooLarge, ErrFormTooLarge)
	assert.Equal(t, "The images exceed the maximum size of the form", textIn(i18n.En, ErrFormTooLarge))
	assert.Equal(t, "las imágenes superan el tamaño máximo del formulario", ErrFormTooLarge.Error())
}
