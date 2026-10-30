//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package models

import (
	"fmt"
	"strings"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
)

// MaxImagesPerMessage is the maximum number of images attached to one message.
const MaxImagesPerMessage = 5

// MaxImageBytes is the maximum decoded size of each image: 4 MiB.
const MaxImageBytes = 4 * 1024 * 1024

// AllowedImageMimeTypes are the accepted image MIME types.
var AllowedImageMimeTypes = [...]string{"image/png", "image/jpeg", "image/gif", "image/webp"}

// ImageAttachment is an image attached to a user message. Data is base64 encoded.
type ImageAttachment struct {
	Name     string
	MimeType string
	Data     string
}

// ValidatedImage checks one image and returns it normalized.
func (i ImageAttachment) ValidatedImage() (ImageAttachment, error) {
	if !IsAllowedImageMimeType(i.MimeType) {
		return ImageAttachment{}, werrors.InvalidAttachment(fmt.Sprintf("%s: tipo de imagen no admitido (%s)", i.Name, i.MimeType))
	}
	if i.Data == "" {
		return ImageAttachment{}, werrors.InvalidAttachment(i.Name + ": la imagen está vacía")
	}
	if i.DecodedLen() > MaxImageBytes {
		return ImageAttachment{}, werrors.InvalidAttachment(i.Name + ": la imagen supera los 4 MiB")
	}
	return i, nil
}

// ValidateAllImages checks the images of one message.
func ValidateAllImages(images []ImageAttachment) ([]ImageAttachment, error) {
	if len(images) > MaxImagesPerMessage {
		return nil, werrors.InvalidAttachment(fmt.Sprintf("máximo %d imágenes por mensaje", MaxImagesPerMessage))
	}
	validated := make([]ImageAttachment, 0, len(images))
	for _, image := range images {
		checked, err := image.ValidatedImage()
		if err != nil {
			return nil, err
		}
		validated = append(validated, checked)
	}
	return validated, nil
}

// DecodedLen estimates the decoded size from the base64 length.
func (i ImageAttachment) DecodedLen() int {
	padding := len(i.Data) - len(strings.TrimRight(i.Data, "="))
	length := len(i.Data)
	decoded := length/4*3 + (length%4)*3/4
	if decoded < padding {
		return 0
	}
	return decoded - padding
}

// IsAllowedImageMimeType reports whether the MIME type is one of the accepted image types.
func IsAllowedImageMimeType(mimeType string) bool {
	for _, allowed := range AllowedImageMimeTypes {
		if allowed == mimeType {
			return true
		}
	}
	return false
}
