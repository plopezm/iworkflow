package application_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/workspaces/application"
	werrors "iworkflow/internal/modules/workspaces/domain/errors"
	"iworkflow/internal/modules/workspaces/domain/models"
)

// fakeMemoryPort answers one batch per call. Each workspace gets its summary by name, and names without one are
// unavailable. err, when set, fails the whole batch. It counts the batches and the workspaces it was asked about.
type fakeMemoryPort struct {
	summaries map[string]models.MemorySummary
	err       error
	// short drops the last summary of the answer, so it does not line up with the workspaces.
	short   bool
	batches int
	asked   []string
}

func (p *fakeMemoryPort) Summaries(_ context.Context, workspaces []models.Workspace) ([]models.MemorySummary, error) {
	p.batches++
	if p.err != nil {
		return nil, p.err
	}
	out := make([]models.MemorySummary, 0, len(workspaces))
	for _, workspace := range workspaces {
		p.asked = append(p.asked, workspace.Name)
		summary, found := p.summaries[workspace.Name]
		if !found {
			summary = models.MemorySummary{State: models.MemoryUnavailable}
		}
		out = append(out, summary)
	}
	if p.short && len(out) > 0 {
		out = out[:len(out)-1]
	}
	return out, nil
}

func TestListWorkspacesWithMemoryPairsEachWorkspaceWithItsSummary(t *testing.T) {
	cases := map[string]struct {
		summary models.MemorySummary
		err     error
		want    models.MemorySummary
	}{
		"available with memories": {
			summary: models.MemorySummary{State: models.MemoryAvailable, Count: 3},
			want:    models.MemorySummary{State: models.MemoryAvailable, Count: 3},
		},
		"available without memories": {
			summary: models.MemorySummary{State: models.MemoryEmpty},
			want:    models.MemorySummary{State: models.MemoryEmpty},
		},
		"unavailable": {
			summary: models.MemorySummary{State: models.MemoryUnavailable},
			want:    models.MemorySummary{State: models.MemoryUnavailable},
		},
		"port error is unavailable, not a failed page": {
			err:  errors.New("unexpected provider failure"),
			want: models.MemorySummary{State: models.MemoryUnavailable},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			repository := newFakeRepository(workspaceAt(t, "alpha", "/repos/alpha", "main"))
			port := &fakeMemoryPort{summaries: map[string]models.MemorySummary{"alpha": tc.summary}, err: tc.err}
			useCase := application.NewListWorkspacesWithMemory(application.NewListWorkspaces(repository), port, discardLogger())

			entries, err := useCase.Execute(context.Background())

			require.NoError(t, err)
			require.Len(t, entries, 1)
			assert.Equal(t, "alpha", entries[0].Workspace.Name)
			assert.Equal(t, tc.want, entries[0].Memory)
		})
	}
}

func TestListWorkspacesWithMemoryAsksOneBatchPerPage(t *testing.T) {
	repository := newFakeRepository(
		workspaceAt(t, "alpha", "/repos/alpha", "main"),
		workspaceAt(t, "beta", "/repos/beta", "main"),
		workspaceAt(t, "gamma", "/repos/gamma", "main"),
	)
	port := &fakeMemoryPort{summaries: map[string]models.MemorySummary{
		"alpha": {State: models.MemoryAvailable, Count: 1},
		"beta":  {State: models.MemoryAvailable, Count: 1},
		"gamma": {State: models.MemoryEmpty},
	}}
	useCase := application.NewListWorkspacesWithMemory(application.NewListWorkspaces(repository), port, discardLogger())

	entries, err := useCase.Execute(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, port.batches, "every row is read by a single batch")
	assert.Equal(t, []string{"alpha", "beta", "gamma"}, port.asked)
	assert.Equal(t, models.MemoryEmpty, entries[2].Memory.State)
}

func TestListWorkspacesWithMemoryFailedBatchMarksEveryRowUnavailable(t *testing.T) {
	repository := newFakeRepository(
		workspaceAt(t, "alpha", "/repos/alpha", "main"),
		workspaceAt(t, "beta", "/repos/beta", "main"),
	)
	port := &fakeMemoryPort{
		summaries: map[string]models.MemorySummary{"alpha": {State: models.MemoryAvailable, Count: 2}},
		err:       errors.New("provider down"),
	}
	useCase := application.NewListWorkspacesWithMemory(application.NewListWorkspaces(repository), port, discardLogger())

	entries, err := useCase.Execute(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, port.batches)
	require.Len(t, entries, 2)
	for _, entry := range entries {
		assert.Equal(t, models.MemoryUnavailable, entry.Memory.State, entry.Workspace.Name)
	}
}

func TestListWorkspacesWithMemoryWithoutAProviderHidesEveryChip(t *testing.T) {
	repository := newFakeRepository(workspaceAt(t, "alpha", "/repos/alpha", "main"))
	useCase := application.NewListWorkspacesWithMemory(application.NewListWorkspaces(repository), nil, discardLogger())

	entries, err := useCase.Execute(context.Background())

	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, models.MemoryDisabled, entries[0].Memory.State, "no provider is wired: no chip, not an unavailable one")
}

func TestListWorkspacesWithMemoryKeepsTheWorkspaceOrder(t *testing.T) {
	repository := newFakeRepository(
		workspaceAt(t, "beta", "/repos/beta", "main"),
		workspaceAt(t, "Alpha", "/repos/alpha", "main"),
	)
	port := &fakeMemoryPort{summaries: map[string]models.MemorySummary{}}
	useCase := application.NewListWorkspacesWithMemory(application.NewListWorkspaces(repository), port, discardLogger())

	entries, err := useCase.Execute(context.Background())

	require.NoError(t, err)
	names := []string{entries[0].Workspace.Name, entries[1].Workspace.Name}
	assert.Equal(t, []string{"Alpha", "beta"}, names)
}

func TestListWorkspacesWithMemoryPropagatesRepositoryErrors(t *testing.T) {
	repository := newFakeRepository()
	repository.listErr = werrors.Repository("boom")
	port := &fakeMemoryPort{}
	useCase := application.NewListWorkspacesWithMemory(application.NewListWorkspaces(repository), port, discardLogger())

	_, err := useCase.Execute(context.Background())

	require.ErrorIs(t, err, werrors.Repository(""))
	assert.EqualError(t, err, "error de persistencia: boom")
	assert.Zero(t, port.batches, "no memory is read when the workspaces cannot be listed")
}

// discardLogger is the project logger of the tests that do not read the log.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// memoryLogLines returns the log lines of a JSON handler, one per record.
func memoryLogLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var records []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		record := map[string]any{}
		require.NoError(t, json.Unmarshal([]byte(line), &record))
		records = append(records, record)
	}
	return records
}

// A failed batch is logged once through the project logger, and every row still degrades to unavailable.
func TestListWorkspacesWithMemoryLogsAFailedBatchOnceAndDegradesEveryRow(t *testing.T) {
	cases := map[string]*fakeMemoryPort{
		"port error": {err: errors.New("provider exploded")},
		"answer that does not line up with the workspaces": {
			summaries: map[string]models.MemorySummary{"alpha": {State: models.MemoryAvailable, Count: 2}},
			short:     true,
		},
	}
	for name, port := range cases {
		t.Run(name, func(t *testing.T) {
			repository := newFakeRepository(
				workspaceAt(t, "alpha", "/repos/alpha", "main"),
				workspaceAt(t, "beta", "/repos/beta", "main"),
			)
			var logs bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&logs, nil))
			useCase := application.NewListWorkspacesWithMemory(application.NewListWorkspaces(repository), port, logger)

			entries, err := useCase.Execute(context.Background())

			require.NoError(t, err)
			require.Len(t, entries, 2)
			for _, entry := range entries {
				assert.Equal(t, models.MemoryUnavailable, entry.Memory.State, entry.Workspace.Name)
			}
			records := memoryLogLines(t, &logs)
			require.Len(t, records, 1, "one log line per failed batch")
			assert.Equal(t, "WARN", records[0]["level"])
			assert.Contains(t, records[0]["msg"], "could not read the memory of the workspaces")
		})
	}
}

// A healthy batch and an unavailable provider are not failures: nothing is logged.
func TestListWorkspacesWithMemoryDoesNotLogHealthyOrUnavailableBatches(t *testing.T) {
	repository := newFakeRepository(workspaceAt(t, "alpha", "/repos/alpha", "main"))
	port := &fakeMemoryPort{summaries: map[string]models.MemorySummary{"alpha": {State: models.MemoryUnavailable}}}
	var logs bytes.Buffer
	useCase := application.NewListWorkspacesWithMemory(application.NewListWorkspaces(repository), port, slog.New(slog.NewJSONHandler(&logs, nil)))

	_, err := useCase.Execute(context.Background())

	require.NoError(t, err)
	assert.Empty(t, memoryLogLines(t, &logs))
}
