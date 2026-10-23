// Package models holds the agents_files entities and value objects. It is pure Go: no I/O.
//
//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package models

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"iworkflow/internal/modules/agents_files/domain/errors"
	"iworkflow/internal/modules/agents_files/domain/limits"
)

// AgentsFileID identifies a stored AGENTS.md. It is a UUID in canonical (lowercase, hyphenated) form.
type AgentsFileID struct {
	value uuid.UUID
}

// NewAgentsFileID generates a random identifier.
func NewAgentsFileID() (AgentsFileID, error) {
	value, err := uuid.NewRandom()
	if err != nil {
		return AgentsFileID{}, err
	}
	return AgentsFileID{value: value}, nil
}

// AgentsFileIDFromUUID wraps an existing UUID, for example one read from the database.
func AgentsFileIDFromUUID(value uuid.UUID) AgentsFileID {
	return AgentsFileID{value: value}
}

// ParseAgentsFileID parses the textual form; any malformed value yields errors.InvalidID.
func ParseAgentsFileID(value string) (AgentsFileID, error) {
	parsed, err := uuid.Parse(value)
	if err != nil {
		return AgentsFileID{}, errors.InvalidID(value)
	}
	return AgentsFileID{value: parsed}, nil
}

// String returns the canonical textual form.
func (id AgentsFileID) String() string { return id.value.String() }

// UUID returns the underlying UUID.
func (id AgentsFileID) UUID() uuid.UUID { return id.value }

// AgentsFile is a reusable AGENTS.md. Its content lives only on disk, at Path.
type AgentsFile struct {
	ID          AgentsFileID
	Description string
	Path        string
	CreatedAt   time.Time
}

// NewAgentsFile is the input for uploading an AGENTS.md.
type NewAgentsFile struct {
	Description string
	Content     []byte
}

// Validated normalizes the input and checks the invariants that do not depend on infrastructure.
// The checks run in a fixed order (description, emptiness, size, UTF-8) and stop at the first failure.
func (input NewAgentsFile) Validated() (NewAgentsFile, error) {
	description := strings.TrimSpace(input.Description)
	if description == "" || utf8.RuneCountInString(description) > limits.MaxDescriptionChars {
		return NewAgentsFile{}, errors.InvalidDescription()
	}
	if len(input.Content) == 0 {
		return NewAgentsFile{}, errors.EmptyContent()
	}
	if len(input.Content) > limits.MaxContentBytes {
		return NewAgentsFile{}, errors.TooLarge()
	}
	if !utf8.Valid(input.Content) {
		return NewAgentsFile{}, errors.NotUTF8()
	}
	return NewAgentsFile{Description: description, Content: input.Content}, nil
}
