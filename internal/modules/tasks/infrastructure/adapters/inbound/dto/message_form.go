package dto

import (
	"encoding/base64"
	"net/http"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/domain/models"
)

// Wire values of the message modes, as the composer sends them.
const (
	messageModeSteer    = "steer"
	messageModeFollowUp = "follow_up"
)

// msgModeInvalid is the refusal of a message mode the adapter does not know.
var msgModeInvalid = i18n.Msg{
	En: "The message mode is not valid.",
	Es: "El modo del mensaje no es válido.",
	Zh: "消息模式无效。",
	De: "Der Nachrichtenmodus ist ungültig.",
	Fr: "Le mode du message n’est pas valide.",
}

// msgAgentModeInvalid is the refusal of an agent mode the mode switch does not know.
var msgAgentModeInvalid = i18n.Msg{
	En: "The agent mode is not valid.",
	Es: "El modo del agente no es válido.",
	Zh: "智能体模式无效。",
	De: "Der Agent-Modus ist ungültig.",
	Fr: "Le mode de l’agent n’est pas valide.",
}

// MessageForm is the message of the composer as the browser sent it. Mode is raw: ToDomain validates it, so a
// message is never queued with a mode the domain does not define.
type MessageForm struct {
	Message string
	Mode    string
	Images  []FormImage
}

// SendMessageInput is a message the send use case can take.
type SendMessageInput struct {
	Message string
	Images  []models.ImageAttachment
	Mode    models.MessageMode
}

// ParseMessageForm reads the multipart message of the composer. The body is capped and every image is read up to
// its limit, the same way the task form is read.
func ParseMessageForm(w http.ResponseWriter, r *http.Request) (MessageForm, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	values, files, err := readFormValues(r)
	if err != nil {
		return MessageForm{}, err
	}
	images, err := readImages(files[imagesField])
	if err != nil {
		return MessageForm{}, err
	}
	return MessageForm{Message: first(values, "message"), Mode: first(values, "mode"), Images: images}, nil
}

// ToDomain validates the mode and encodes the images as the domain expects. Image limits and types are checked by
// the use case, which owns them.
func (f MessageForm) ToDomain() (SendMessageInput, error) {
	mode, err := ParseMessageMode(f.Mode)
	if err != nil {
		return SendMessageInput{}, err
	}
	images := make([]models.ImageAttachment, 0, len(f.Images))
	for _, image := range f.Images {
		images = append(images, models.ImageAttachment{
			Name:     image.Name,
			MimeType: image.MimeType,
			Data:     base64.StdEncoding.EncodeToString(image.Data),
		})
	}
	return SendMessageInput{Message: f.Message, Images: images, Mode: mode}, nil
}

// ParseAgentModeForm converts the wire value of an agent mode the mode switch posts. Only agent and plan exist.
func ParseAgentModeForm(raw string) (models.AgentMode, error) {
	mode, known := models.ParseAgentMode(raw)
	if !known {
		return "", formErr(msgAgentModeInvalid, nil)
	}
	return mode, nil
}

// ParseMessageMode converts the wire value of a message mode. Only the two queue modes exist: an idle agent always
// gets a prompt, a working one a steer or a follow-up. Anything else, including an empty value, is refused.
func ParseMessageMode(raw string) (models.MessageMode, error) {
	switch raw {
	case messageModeSteer:
		return models.MessageModeSteer, nil
	case messageModeFollowUp:
		return models.MessageModeFollowUp, nil
	default:
		return 0, formErr(msgModeInvalid, nil)
	}
}
