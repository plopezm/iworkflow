// Package dto holds the input and output shapes of the data_home HTTP adapter. Handlers never expose
// the domain models or the core types directly.
package dto

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"iworkflow/internal/modules/data_home/domain/models"
)

// HomeStatusDTO is the data folder as the card reads it.
type HomeStatusDTO struct {
	Current   string
	Source    string
	EnvLocked bool
	Pending   *PendingDTO
	Last      *OutcomeDTO
}

// PendingDTO is a move waiting for the next start.
type PendingDTO struct {
	From string
	To   string
}

// OutcomeDTO is the result of the last move.
type OutcomeDTO struct {
	OK         bool
	From       string
	To         string
	Error      string
	Warnings   []string
	FinishedAt time.Time
}

// FromHomeStatus maps the status of the domain to its output shape.
func FromHomeStatus(status models.HomeStatus) HomeStatusDTO {
	mapped := HomeStatusDTO{Current: status.Current, Source: string(status.Source), EnvLocked: status.EnvLocked}
	if status.Pending != nil {
		mapped.Pending = &PendingDTO{From: status.Pending.From, To: status.Pending.To}
	}
	if status.Last != nil {
		mapped.Last = &OutcomeDTO{
			OK:         status.Last.OK,
			From:       status.Last.From,
			To:         status.Last.To,
			Error:      status.Last.Error,
			Warnings:   status.Last.Warnings,
			FinishedAt: status.Last.FinishedAt,
		}
	}
	return mapped
}

// MoveForm is the form of the move request: the destination the user typed.
type MoveForm struct {
	Destination string
}

// DecodeMoveForm reads the move form. The destination is trimmed; its validity is the use case's concern.
func DecodeMoveForm(r *http.Request) (MoveForm, error) {
	if err := r.ParseForm(); err != nil {
		return MoveForm{}, fmt.Errorf("read the move form: %w", err)
	}
	return MoveForm{Destination: strings.TrimSpace(r.PostForm.Get("destination"))}, nil
}
