package inbound_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/agents_files/domain/errors"
	"iworkflow/internal/modules/agents_files/domain/limits"
	"iworkflow/internal/modules/agents_files/infrastructure/adapters/inbound"
)

type formPart struct {
	name string
	data []byte
}

// multipartRequest builds an upload request with the given parts, in order.
func multipartRequest(t *testing.T, parts ...formPart) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, part := range parts {
		field, err := writer.CreateFormField(part.name)
		require.NoError(t, err)
		_, err = field.Write(part.data)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	req := httptest.NewRequest(http.MethodPost, "/agents-files/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestReadUploadKeepsDescriptionAndFileAndIgnoresOtherFields(t *testing.T) {
	req := multipartRequest(t,
		formPart{name: "other", data: []byte("zzz")},
		formPart{name: "description", data: []byte("Reglas")},
		formPart{name: "file", data: []byte("# hola")},
	)

	body, err := inbound.ReadUpload(req)

	require.NoError(t, err)
	require.Equal(t, "Reglas", body.Description)
	require.Equal(t, []byte("# hola"), body.Content)
}

func TestReadUploadRejectsMissingFields(t *testing.T) {
	tests := []struct {
		name    string
		parts   []formPart
		wantErr error
	}{
		{"missing file", []formPart{{name: "description", data: []byte("d")}}, errors.EmptyContent()},
		{"missing description", []formPart{{name: "file", data: []byte("x")}}, errors.InvalidDescription()},
		{"missing both", nil, errors.InvalidDescription()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := inbound.ReadUpload(multipartRequest(t, tc.parts...))

			require.Equal(t, tc.wantErr, err)
		})
	}
}

func TestReadUploadStopsAtTheDescriptionCap(t *testing.T) {
	// Four bytes per character is the widest UTF-8; past that the part is refused before validation.
	tooLong := []byte(strings.Repeat("a", 4*limits.MaxDescriptionChars+1))

	_, err := inbound.ReadUpload(multipartRequest(t,
		formPart{name: "description", data: tooLong},
		formPart{name: "file", data: []byte("x")},
	))

	require.Equal(t, errors.InvalidDescription(), err)
}

func TestReadUploadLetsTheUseCaseCheckTheCharacterLimit(t *testing.T) {
	// 201 characters fit the byte cap, so the parser accepts them; the use case rejects them later.
	over := strings.Repeat("a", limits.MaxDescriptionChars+1)

	body, err := inbound.ReadUpload(multipartRequest(t,
		formPart{name: "description", data: []byte(over)},
		formPart{name: "file", data: []byte("x")},
	))

	require.NoError(t, err)
	require.Equal(t, over, body.Description)
}

func TestReadUploadRejectsNonUTF8Description(t *testing.T) {
	_, err := inbound.ReadUpload(multipartRequest(t,
		formPart{name: "description", data: []byte{0xff, 0xfe}},
		formPart{name: "file", data: []byte("x")},
	))

	require.Equal(t, errors.InvalidDescription(), err)
}

func TestReadUploadRejectsFilesOverTheLimit(t *testing.T) {
	_, err := inbound.ReadUpload(multipartRequest(t,
		formPart{name: "description", data: []byte("d")},
		formPart{name: "file", data: bytes.Repeat([]byte("a"), limits.MaxContentBytes+1)},
	))

	require.Equal(t, errors.TooLarge(), err)
}

func TestReadUploadAcceptsAFileExactlyAtTheLimit(t *testing.T) {
	_, err := inbound.ReadUpload(multipartRequest(t,
		formPart{name: "description", data: []byte("d")},
		formPart{name: "file", data: bytes.Repeat([]byte("a"), limits.MaxContentBytes)},
	))

	require.NoError(t, err)
}

func TestReadUploadCountsEveryPartAgainstTheFieldLimit(t *testing.T) {
	nine := make([]formPart, 0, 9)
	for i := 0; i < 9; i++ {
		nine = append(nine, formPart{name: "other", data: []byte("x")})
	}

	_, err := inbound.ReadUpload(multipartRequest(t, nine...))

	require.EqualError(t, err, "formulario de subida inválido: demasiados campos")
}

func TestReadUploadAcceptsEightParts(t *testing.T) {
	parts := []formPart{
		{name: "description", data: []byte("d")},
		{name: "file", data: []byte("x")},
	}
	for i := 0; i < 6; i++ {
		parts = append(parts, formPart{name: "other", data: []byte("x")})
	}

	_, err := inbound.ReadUpload(multipartRequest(t, parts...))

	require.NoError(t, err)
}

func TestReadUploadRejectsANonMultipartRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/agents-files/upload", strings.NewReader("x=1"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	_, err := inbound.ReadUpload(req)

	require.ErrorContains(t, err, "formulario de subida inválido: ")
}
