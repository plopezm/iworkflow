package dto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecodeResizeMessageParsesAResizeFrame(t *testing.T) {
	message, err := DecodeResizeMessage([]byte(`{"type":"resize","cols":100,"rows":30}`))

	require.NoError(t, err)
	require.Equal(t, ResizeMessage{Cols: 100, Rows: 30}, message)
}

func TestDecodeResizeMessageRejectsUnknownTypesGarbageAndOverflow(t *testing.T) {
	cases := map[string]string{
		"unknown type":  `{"type":"nope"}`,
		"plain text":    "hola",
		"overflow":      `{"type":"resize","cols":99999,"rows":1}`,
		"missing field": `{"type":"resize","cols":1}`,
		"null payload":  `null`,
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := DecodeResizeMessage([]byte(payload))

			require.Error(t, err)
		})
	}
}

func TestDecodeResizeMessageIgnoresExtraFields(t *testing.T) {
	message, err := DecodeResizeMessage([]byte(`{"type":"resize","cols":80,"rows":24,"extra":true}`))

	require.NoError(t, err)
	require.Equal(t, ResizeMessage{Cols: 80, Rows: 24}, message)
}
