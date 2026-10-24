//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package api

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
	"iworkflow/internal/modules/jira/domain/models"
)

// issueFields are the fields requested from Jira; they keep the responses small.
const issueFields = "summary,description,status,issuetype,priority,assignee,reporter,labels,updated"

// jiraDateLayout is the Jira timestamp form: an offset without a colon (2024-05-01T10:00:00.000+0200).
const jiraDateLayout = "2006-01-02T15:04:05.000-0700"

// decodeDocument parses an answer. A body that is not JSON is an invalid answer, as in the Rust client.
func decodeDocument(body []byte) (any, error) {
	var doc any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, jiraerrors.Remote("respuesta no válida: " + err.Error())
	}
	return doc, nil
}

// textAt returns the string at a JSON Pointer (RFC 6901), or nil when the path is missing or not text.
func textAt(root any, pointer string) *string {
	value, ok := valueAt(root, pointer)
	if !ok {
		return nil
	}
	text, ok := value.(string)
	if !ok {
		return nil
	}
	return &text
}

// valueAt walks the objects of a document along a JSON Pointer.
func valueAt(root any, pointer string) (any, bool) {
	current := root
	for _, segment := range strings.Split(strings.TrimPrefix(pointer, "/"), "/") {
		if pointer == "" {
			return root, true
		}
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		next, found := object[segment]
		if !found {
			return nil, false
		}
		current = next
	}
	return current, true
}

// parseJiraDate reads the Jira timestamp form, or the RFC 3339 form. It returns nil when neither fits.
func parseJiraDate(value string) *time.Time {
	for _, layout := range []string{jiraDateLayout, time.RFC3339Nano} {
		if parsed, err := time.Parse(layout, value); err == nil {
			utc := parsed.UTC()
			return &utc
		}
	}
	return nil
}

// dateAt reads a timestamp at a pointer, falling back to now when it is missing or unreadable.
func dateAt(root any, pointer string, now func() time.Time) time.Time {
	if text := textAt(root, pointer); text != nil {
		if parsed := parseJiraDate(*text); parsed != nil {
			return *parsed
		}
	}
	return now()
}

// optionalTextAt returns the text at a pointer, or nil when it is absent.
func optionalTextAt(root any, pointer string) *string {
	return textAt(root, pointer)
}

// personAt returns the display name of a user, or its login when there is no display name.
func personAt(root any, pointer string) *string {
	if name := textAt(root, pointer+"/displayName"); name != nil {
		return name
	}
	return textAt(root, pointer+"/name")
}

// issueFromJSON maps one issue document of REST v2 to the remote issue.
func issueFromJSON(connection models.Connection, now func() time.Time, doc any) (models.RemoteIssue, error) {
	key := textAt(doc, "/key")
	if key == nil {
		return models.RemoteIssue{}, jiraerrors.Remote("respuesta de Jira sin clave de tarea")
	}
	labels := []string{}
	if values, ok := valueAt(doc, "/fields/labels"); ok {
		if list, isList := values.([]any); isList {
			for _, entry := range list {
				if label, isText := entry.(string); isText {
					labels = append(labels, label)
				}
			}
		}
	}
	return models.RemoteIssue{
		Key:           *key,
		Summary:       valueOrEmpty(textAt(doc, "/fields/summary")),
		Description:   valueOrEmpty(textAt(doc, "/fields/description")),
		Status:        valueOrEmpty(textAt(doc, "/fields/status/name")),
		IssueType:     valueOrEmpty(textAt(doc, "/fields/issuetype/name")),
		Priority:      optionalTextAt(doc, "/fields/priority/name"),
		Assignee:      personAt(doc, "/fields/assignee"),
		Reporter:      personAt(doc, "/fields/reporter"),
		Labels:        labels,
		URL:           connection.BrowseURL(*key),
		JiraUpdatedAt: dateAt(doc, "/fields/updated", now),
	}, nil
}

// searchPageFromJSON reads one page of /search: the issues plus the offset of the next page and the
// total, which decide whether another page is needed.
func searchPageFromJSON(connection models.Connection, now func() time.Time, doc any) ([]models.RemoteIssue, int, int, error) {
	raw, ok := valueAt(doc, "/issues")
	list, isList := raw.([]any)
	if !ok || !isList {
		return nil, 0, 0, jiraerrors.Remote("respuesta de búsqueda inesperada")
	}
	issues := make([]models.RemoteIssue, 0, len(list))
	for _, entry := range list {
		issue, err := issueFromJSON(connection, now, entry)
		if err != nil {
			return nil, 0, 0, err
		}
		issues = append(issues, issue)
	}
	startAt := numberAt(doc, "/startAt")
	total := numberAt(doc, "/total")
	return issues, startAt + len(issues), total, nil
}

// numberAt reads a non-negative integer, zero when it is missing.
func numberAt(root any, pointer string) int {
	value, ok := valueAt(root, pointer)
	if !ok {
		return 0
	}
	number, ok := value.(float64)
	if !ok || number < 0 {
		return 0
	}
	return int(number)
}

// commentsFromJSON maps the comments of an issue; a missing list is an empty one.
func commentsFromJSON(now func() time.Time, doc any) []models.Comment {
	list, ok := valueAt(doc, "/comments")
	entries, isList := list.([]any)
	if !ok || !isList {
		return []models.Comment{}
	}
	comments := make([]models.Comment, 0, len(entries))
	for _, entry := range entries {
		author := personAt(entry, "/author")
		comments = append(comments, models.Comment{
			ID:        valueOrEmpty(textAt(entry, "/id")),
			Author:    valueOrEmpty(author),
			Body:      valueOrEmpty(textAt(entry, "/body")),
			CreatedAt: dateAt(entry, "/created", now),
		})
	}
	return comments
}

// transitionsFromJSON maps the allowed transitions; an entry without id or name is skipped.
func transitionsFromJSON(doc any) []models.Transition {
	list, ok := valueAt(doc, "/transitions")
	entries, isList := list.([]any)
	if !ok || !isList {
		return []models.Transition{}
	}
	transitions := make([]models.Transition, 0, len(entries))
	for _, entry := range entries {
		id := textAt(entry, "/id")
		name := textAt(entry, "/name")
		if id == nil || name == nil {
			continue
		}
		toStatus := *name
		if target := textAt(entry, "/to/name"); target != nil {
			toStatus = *target
		}
		transitions = append(transitions, models.Transition{ID: *id, Name: *name, ToStatus: toStatus})
	}
	return transitions
}

// errorMessage extracts the first message of a Jira error body: `errorMessages`, else the first value
// of `errors` (keys in sorted order, so the choice is stable). It returns "" when there is none.
func errorMessage(body []byte) string {
	var doc any
	if err := json.Unmarshal(body, &doc); err != nil {
		return ""
	}
	if messages, ok := valueAt(doc, "/errorMessages"); ok {
		if list, isList := messages.([]any); isList && len(list) > 0 {
			if first, isText := list[0].(string); isText {
				return first
			}
		}
	}
	if errs, ok := valueAt(doc, "/errors"); ok {
		if object, isObject := errs.(map[string]any); isObject {
			keys := make([]string, 0, len(object))
			for key := range object {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				if text, isText := object[key].(string); isText {
					return text
				}
			}
		}
	}
	return ""
}

// encodePayload renders a JSON request body.
func encodePayload(value any) ([]byte, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, jiraerrors.Remote(fmt.Sprintf("petición no válida: %v", err))
	}
	return payload, nil
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
