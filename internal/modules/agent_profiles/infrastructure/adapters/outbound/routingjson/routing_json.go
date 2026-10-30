// Package routingjson converts between JSON documents and routing maps. profiles.json, models.json
// and the exchange envelope share this layout: an object of agent -> entry, where an entry is an
// object with optional `model` and `thinking`, or a legacy `"provider/model"` string.
//
//nolint:wrapcheck // Validation errors are returned as domain errors: their text is user-visible and must stay identical to the Rust version.
package routingjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"

	"iworkflow/internal/modules/agent_profiles/domain/models"
)

// Decode reads one JSON document. Numbers stay json.Number so an integer field can be checked
// exactly, and trailing content after the document is an error.
func Decode(raw []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("contenido inesperado después del documento JSON")
	}
	return value, nil
}

// Encode writes value as pretty JSON with two-space indentation, sorted object keys, no HTML
// escaping and a trailing newline.
func Encode(value any) (string, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return "", fmt.Errorf("encode json: %w", err)
	}
	return buffer.String(), nil
}

// Compact writes value as one-line JSON without a trailing newline.
func Compact(value any) (string, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return "", fmt.Errorf("encode json: %w", err)
	}
	return string(bytes.TrimSuffix(buffer.Bytes(), []byte("\n"))), nil
}

// ParseRouting maps an object of agent -> entry. Agents are visited in sorted order, and every
// error names the agent it belongs to.
func ParseRouting(object map[string]any) (map[string]models.RoutingEntry, error) {
	routing := make(map[string]models.RoutingEntry, len(object))
	for _, agent := range slices.Sorted(maps.Keys(object)) {
		entry, err := parseEntry(object[agent])
		if err != nil {
			return nil, fmt.Errorf("agente `%s`: %w", agent, err)
		}
		routing[agent] = entry
	}
	if err := models.ValidateRouting(routing); err != nil {
		return nil, err
	}
	return routing, nil
}

// RoutingToObject builds the JSON object for a routing map. Only the fields that are set appear.
func RoutingToObject(routing map[string]models.RoutingEntry) map[string]any {
	object := make(map[string]any, len(routing))
	for agent, entry := range routing {
		fields := map[string]any{}
		if entry.Model != nil {
			fields["model"] = *entry.Model
		}
		if entry.Thinking != nil {
			fields["thinking"] = *entry.Thinking
		}
		object[agent] = fields
	}
	return object
}

func parseEntry(value any) (models.RoutingEntry, error) {
	var entry models.RoutingEntry
	switch typed := value.(type) {
	case string:
		entry = models.RoutingEntry{Model: &typed}
	case map[string]any:
		model, err := optionalString(typed, "model")
		if err != nil {
			return models.RoutingEntry{}, err
		}
		thinking, err := optionalString(typed, "thinking")
		if err != nil {
			return models.RoutingEntry{}, err
		}
		entry = models.RoutingEntry{Model: model, Thinking: thinking}
	default:
		return models.RoutingEntry{}, errors.New("se esperaba un objeto o una cadena")
	}
	if err := entry.Validate(); err != nil {
		return models.RoutingEntry{}, err
	}
	return entry, nil
}

// optionalString returns nil for an absent or null key and an error for a non-string value.
func optionalString(object map[string]any, key string) (*string, error) {
	value, present := object[key]
	if !present || value == nil {
		return nil, nil
	}
	text, isString := value.(string)
	if !isString {
		return nil, fmt.Errorf("`%s` debe ser texto", key)
	}
	return &text, nil
}
