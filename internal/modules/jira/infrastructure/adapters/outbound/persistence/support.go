//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package persistence

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
)

// timestampLayout is the text form the Rust (Diesel) version wrote for TIMESTAMP columns:
// "YYYY-MM-DD HH:MM:SS" plus the fraction only when it is not zero (chrono's %.f). The
// ".999999999" element gives the same shape and keeps full precision. Both versions must read
// each other's rows.
const timestampLayout = "2006-01-02 15:04:05.999999999"

// timestampLayouts are accepted when reading. The second one is what database/sql hands over when
// the driver converts a stored value itself (RFC 3339, the form time.Time takes as a string).
var timestampLayouts = [...]string{
	"2006-01-02 15:04:05.999999999",
	"2006-01-02T15:04:05.999999999Z07:00",
}

// formatTimestamp renders an instant the way the Rust version stored it: UTC, no offset.
func formatTimestamp(value time.Time) string {
	return value.UTC().Format(timestampLayout)
}

// parseTimestamp reads a stored instant. Malformed stored data is a repository error.
func parseTimestamp(value string) (time.Time, error) {
	for _, layout := range timestampLayouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, jiraerrors.Repository(fmt.Sprintf("fecha inválida %s", value))
}

// parseOptionalTimestamp reads a nullable instant; a NULL is nil.
func parseOptionalTimestamp(value *string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	parsed, err := parseTimestamp(*value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

// dbError turns a driver failure into the repository error of the module.
func dbError(err error) error {
	return jiraerrors.Repository(err.Error())
}

// isNoRows reports whether a single-row query found nothing.
func isNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

// nullString converts an optional text to the SQL form.
func nullString(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *value, Valid: true}
}

// optionalString converts a nullable SQL text back to the optional form.
func optionalString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	copied := value.String
	return &copied
}
