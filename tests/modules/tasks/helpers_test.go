package tasks_test

import (
	"context"
	"database/sql"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	"iworkflow/internal/modules/tasks/application"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/persistence"
)

// fixedClock is the instant the repository writes when a test does not move it.
var fixedClock = time.Date(2026, 10, 24, 10, 0, 0, 0, time.UTC)

// testClock returns the instant of the test; advance moves it forward.
type testClock struct {
	now atomic.Pointer[time.Time]
}

func newTestClock() *testClock {
	clock := &testClock{}
	start := fixedClock
	clock.now.Store(&start)
	return clock
}

func (c *testClock) Now() time.Time {
	return *c.now.Load()
}

// advance moves the clock forward, so that a write which changes updated_at is visible in tests.
func (c *testClock) advance(d time.Duration) {
	next := c.Now().Add(d)
	c.now.Store(&next)
}

// Repo bundles a migrated database with the repository built on it.
type Repo struct {
	DB          *sql.DB
	Tasks       *persistence.TaskRepository
	Attachments *persistence.TaskAttachmentRepository
	Graph       *persistence.TaskGraphRepository
	clock       *testClock
}

// advance moves the repository clock forward.
func (r *Repo) advance(d time.Duration) {
	r.clock.advance(d)
}

func newRepo(t *testing.T) *Repo {
	t.Helper()
	db := dbtest.New(t)
	clock := newTestClock()
	return &Repo{
		DB:          db,
		Tasks:       persistence.NewTaskRepository(db, clock.Now),
		Attachments: persistence.NewTaskAttachmentRepository(db, clock.Now),
		Graph:       persistence.NewTaskGraphRepository(db, clock.Now),
		clock:       clock,
	}
}

// insertWorkspace stores a workspace row. The tasks table references workspaces, so the tests need one.
// It is a fixture: parameters are bound and the workspaces module is not imported.
func insertWorkspace(t *testing.T, db *sql.DB, path string) string {
	t.Helper()
	id := uuid.NewString()
	_, err := db.ExecContext(context.Background(),
		"INSERT INTO workspaces (id, name, path, base_branch, created_at) VALUES (?, ?, ?, ?, ?)",
		id, "demo", path, "main", "2026-01-01 00:00:00")
	require.NoError(t, err)
	return id
}

// newTask builds a task in the given status with a fresh id and a microsecond-truncated timestamp,
// which is the precision the repository stores.
func newTask(t *testing.T, workspaceID string, title string, status models.TaskStatus) models.Task {
	t.Helper()
	id, err := models.NewTaskID()
	require.NoError(t, err)
	now := fixedClock.Truncate(time.Microsecond)
	return models.Task{
		ID:           id,
		WorkspaceID:  workspaceID,
		Title:        title,
		Prompt:       "haz algo",
		Branch:       "pi/" + title,
		BaseBranch:   "main",
		WorktreePath: "/wt/" + title,
		Status:       status,
		Priority:     models.DefaultTaskPriority,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// insertTask stores the task and fails the test on error.
func insertTask(t *testing.T, repo *Repo, task models.Task) {
	t.Helper()
	require.NoError(t, repo.Tasks.Insert(context.Background(), task))
}

// found loads the task and fails the test when it is missing.
func found(t *testing.T, repo *Repo, id models.TaskID) models.Task {
	t.Helper()
	task, err := repo.Tasks.Find(context.Background(), id)
	require.NoError(t, err)
	require.NotNil(t, task, "task %s should exist", id)
	return *task
}

// ptr returns a pointer to a copy of value.
func ptr[T any](value T) *T {
	return &value
}

// bg is the context of the tests that do not cancel anything.
func bg() context.Context {
	return context.Background()
}

// editInput is an update form that changes only the title and prompt.
func editInput(title string, prompt string) application.UpdateInput {
	return application.UpdateInput{Title: title, Prompt: prompt, Priority: models.DefaultTaskPriority}
}
