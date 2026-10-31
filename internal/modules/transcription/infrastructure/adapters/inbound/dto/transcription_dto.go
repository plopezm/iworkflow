// Package dto holds the JSON shapes the transcription endpoint answers.
package dto

// TranscriptionResponse is the answer of a successful transcription.
type TranscriptionResponse struct {
	Text string `json:"text"`
}

// ErrorResponse is the answer of a failed transcription; Error is already translated.
type ErrorResponse struct {
	Error string `json:"error"`
}
