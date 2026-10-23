//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package inbound

import (
	"errors"
	"io"
	"net/http"
	"unicode/utf8"

	agentserrors "iworkflow/internal/modules/agents_files/domain/errors"
	"iworkflow/internal/modules/agents_files/domain/limits"
	"iworkflow/internal/modules/agents_files/domain/models"
)

const (
	// maxFields is the number of multipart parts accepted: the two fields plus a margin.
	maxFields = 8
	// maxDescriptionBytes allows four bytes per character, the worst case of UTF-8.
	maxDescriptionBytes = limits.MaxDescriptionChars * 4
	descriptionField    = "description"
	fileField           = "file"
)

// UploadBody is an upload decoded from the multipart form. The business checks (blank or long
// description, empty content) run in the use case, not here.
type UploadBody struct {
	Description string
	Content     []byte
}

// NewAgentsFile is the input of the upload use case.
func (b UploadBody) NewAgentsFile() models.NewAgentsFile {
	return models.NewAgentsFile{Description: b.Description, Content: b.Content}
}

// malformedUploadError means the multipart body itself is broken, not that a business rule failed.
type malformedUploadError struct {
	detail string
}

func (e malformedUploadError) Error() string {
	return "formulario de subida inválido: " + e.detail
}

// ReadUpload decodes the multipart form. It reads the "description" and "file" parts, ignores any
// other field, and stops reading a part as soon as it exceeds its limit. The errors and their order
// follow the Rust parse_upload: description limits first, then the file, then the missing fields.
func ReadUpload(r *http.Request) (UploadBody, error) {
	reader, err := r.MultipartReader()
	if err != nil {
		return UploadBody{}, malformedUploadError{detail: err.Error()}
	}
	var body UploadBody
	var hasDescription, hasFile bool
	seen := 0
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return body, malformedUploadError{detail: err.Error()}
		}
		seen++
		if seen > maxFields {
			return body, malformedUploadError{detail: "demasiados campos"}
		}
		switch part.FormName() {
		case descriptionField:
			raw, withinLimit, err := readCapped(part, maxDescriptionBytes)
			if err != nil {
				return body, err
			}
			if !withinLimit || !utf8.Valid(raw) {
				return body, agentserrors.InvalidDescription()
			}
			body.Description = string(raw)
			hasDescription = true
		case fileField:
			content, withinLimit, err := readCapped(part, limits.MaxContentBytes)
			if err != nil {
				return body, err
			}
			if !withinLimit {
				return body, agentserrors.TooLarge()
			}
			body.Content = content
			hasFile = true
		}
	}
	if !hasDescription {
		return body, agentserrors.InvalidDescription()
	}
	if !hasFile {
		return body, agentserrors.EmptyContent()
	}
	return body, nil
}

// readCapped reads at most limit bytes of part. withinLimit is false when the part is longer.
func readCapped(part io.Reader, limit int) ([]byte, bool, error) {
	data, err := io.ReadAll(io.LimitReader(part, int64(limit)+1))
	if err != nil {
		return nil, false, malformedUploadError{detail: err.Error()}
	}
	if len(data) > limit {
		return nil, false, nil
	}
	return data, true, nil
}
