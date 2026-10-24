package dto

import (
	"encoding/json"
	"errors"
)

// resizeMessageType is the "type" value of the resize control frame.
const resizeMessageType string = "resize"

// ErrUnknownControlMessage reports a text frame that is not a valid resize control message.
var ErrUnknownControlMessage = errors.New("mensaje de control desconocido")

// ResizeMessage is a client request to change the window size, sent as a JSON text frame.
type ResizeMessage struct {
	Cols uint16
	Rows uint16
}

// controlWire is the JSON shape of a control frame. Pointers tell a missing field from a zero one.
type controlWire struct {
	Type string  `json:"type"`
	Cols *uint16 `json:"cols"`
	Rows *uint16 `json:"rows"`
}

// DecodeResizeMessage parses a text frame such as {"type":"resize","cols":100,"rows":30}. Unknown
// types, missing dimensions, non-object input and values that do not fit in uint16 are rejected.
func DecodeResizeMessage(payload []byte) (ResizeMessage, error) {
	var wire controlWire
	if err := json.Unmarshal(payload, &wire); err != nil {
		return ResizeMessage{}, err //nolint:wrapcheck // the decoder message is the diagnostic
	}
	if wire.Type != resizeMessageType || wire.Cols == nil || wire.Rows == nil {
		return ResizeMessage{}, ErrUnknownControlMessage
	}
	return ResizeMessage{Cols: *wire.Cols, Rows: *wire.Rows}, nil
}
