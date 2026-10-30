//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package persistence

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	werrors "iworkflow/internal/modules/workspaces/domain/errors"
	"iworkflow/internal/modules/workspaces/domain/models"
	"iworkflow/internal/modules/workspaces/infrastructure/adapters/outbound/persistence/gen"
)

// timestampLayout is the text form the Rust (Diesel) version wrote for TIMESTAMP columns:
// "YYYY-MM-DD HH:MM:SS" plus the fraction only when it is not zero (chrono's %.f). The
// ".999999999" element gives the same shape and keeps full precision. Both versions must read
// each other's rows.
const timestampLayout = "2006-01-02 15:04:05.999999999"

// timestampLayouts are accepted when reading. The last one is what the SQLite driver returns
// when it converts a stored value itself.
var timestampLayouts = [...]string{
	"2006-01-02 15:04:05.999999999",
	"2006-01-02T15:04:05.999999999Z07:00",
}

func formatTimestamp(value time.Time) string {
	return value.UTC().Format(timestampLayout)
}

func parseTimestamp(value string) (time.Time, error) {
	for _, layout := range timestampLayouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid timestamp %q", value)
}

// rowToDomain maps a stored row to the domain entity. Malformed stored data is a repository error.
func rowToDomain(row gen.Workspace) (models.Workspace, error) {
	id, err := uuid.Parse(row.ID)
	if err != nil {
		return models.Workspace{}, werrors.Repository(fmt.Sprintf("id inválido %s: %v", row.ID, err))
	}
	createdAt, err := parseTimestamp(row.CreatedAt)
	if err != nil {
		return models.Workspace{}, werrors.Repository(fmt.Sprintf("fecha de creación inválida %s: %v", row.CreatedAt, err))
	}
	return models.Workspace{
		ID:         models.WorkspaceIDFromUUID(id),
		Name:       row.Name,
		Path:       row.Path,
		BaseBranch: row.BaseBranch,
		CreatedAt:  createdAt,
	}, nil
}
