package tasks_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/persistence"
)

// lockHoldDuration is longer than the connection busy timeout (5 s), so the first attempts of a write
// really fail with a locked database and the retry decorator has to carry the call.
const lockHoldDuration = 5500 * time.Millisecond

// TestRetryingRepositoryRecoversWhenTheDatabaseIsLocked holds a write lock on a second connection, makes
// a write through the retrying decorator, and releases the lock after the busy timeout has expired. The
// write must succeed once the lock is free, without the caller seeing a transient error.
func TestRetryingRepositoryRecoversWhenTheDatabaseIsLocked(t *testing.T) {
	if testing.Short() {
		t.Skip("holds a database lock for longer than the busy timeout")
	}
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/busy")
	task := newTask(t, workspaceID, "locked", models.StatusRunning)
	insertTask(t, repo, task)

	locked := make(chan struct{})
	release := make(chan struct{})
	var holder sync.WaitGroup
	var holdErr error
	holder.Add(1)
	go func() {
		defer holder.Done()
		holdErr = holdWriteLock(t, repo, locked, release)
	}()
	<-locked

	retrying := persistence.NewRetryingTaskRepository(repo.Tasks, persistence.BoundedRetryPolicy(), repo.clock.Now, persistence.ContextSleep)
	writeErr := make(chan error, 1)
	go func() {
		writeErr <- retrying.AddPendingDialog(bg(), task.ID, models.NewPendingDialog("after-lock", nil))
	}()
	time.Sleep(lockHoldDuration)
	close(release)

	require.NoError(t, <-writeErr, "the retrying write must succeed once the lock is released")
	holder.Wait()
	require.NoError(t, holdErr)
	assert.Equal(t, []string{"after-lock"}, dialogIDs(found(t, repo, task.ID).PendingDialogs))
}

// holdWriteLock takes the write lock on its own connection and keeps it until release is closed.
func holdWriteLock(t *testing.T, repo *Repo, locked chan<- struct{}, release <-chan struct{}) error {
	t.Helper()
	conn, err := repo.DB.Conn(bg())
	if err != nil {
		close(locked)
		return err
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.ExecContext(bg(), "BEGIN IMMEDIATE"); err != nil {
		close(locked)
		return err
	}
	close(locked)
	<-release
	_, err = conn.ExecContext(bg(), "ROLLBACK")
	return err
}
