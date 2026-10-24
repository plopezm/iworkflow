package persistence

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

func TestDatabaseIsLockedMapsToBusy(t *testing.T) {
	t.Parallel()
	err := dbError(errors.New("database is locked (5) (SQLITE_BUSY)"))
	assert.True(t, werrors.IsTransient(err))
}

func TestTableLockedMapsToBusyCaseInsensitive(t *testing.T) {
	t.Parallel()
	err := dbError(errors.New("Database TABLE is LOCKED"))
	assert.True(t, werrors.IsTransient(err))
}

func TestOtherDatabaseErrorMapsToRepository(t *testing.T) {
	t.Parallel()
	err := dbError(errors.New("UNIQUE constraint failed"))
	assert.False(t, werrors.IsTransient(err))
	assert.ErrorIs(t, err, werrors.Repository(""))
}

func TestActiveAndClosedStatusNamesPartitionEveryStatus(t *testing.T) {
	t.Parallel()
	active := activeNames()
	closed := closedStatusNames()
	for _, name := range active {
		assert.NotContains(t, closed, name)
	}
	assert.Len(t, active, len(models.AllTaskStatuses)-len(closed))
	assert.NotContains(t, active, "merged")
	assert.NotContains(t, active, "discarded")
	assert.Contains(t, active, "backlog")
	assert.Contains(t, active, "failed")
}

// activeNames computes the complement of closedStatusNames independently, so the partition is checked
// against the domain predicate rather than against the code under test.
func activeNames() []string {
	var names []string
	for _, status := range models.AllTaskStatuses {
		if !status.IsClosed() {
			names = append(names, status.String())
		}
	}
	return names
}
