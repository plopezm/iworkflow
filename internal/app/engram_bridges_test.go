package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	engramapp "iworkflow/internal/modules/engram/application"
	engrammodels "iworkflow/internal/modules/engram/domain/models"
	tasksmodels "iworkflow/internal/modules/tasks/domain/models"
	workspacesmodels "iworkflow/internal/modules/workspaces/domain/models"
)

// fakeWorkspacesMemory stands in for the engram ListWorkspacesMemory use case. It answers memories and err, and
// records every batch it was asked about.
type fakeWorkspacesMemory struct {
	memories []engrammodels.WorkspaceMemory
	err      error
	batches  [][]engrammodels.Workspace
}

func (f *fakeWorkspacesMemory) Execute(_ context.Context, workspaces []engrammodels.Workspace) ([]engrammodels.WorkspaceMemory, error) {
	f.batches = append(f.batches, workspaces)
	return f.memories, f.err
}

// countingMemoryStore is an engram MemoryStore that counts its reads, so a test can prove how many requests a page
// makes. It answers the projects and the recent observations it holds.
type countingMemoryStore struct {
	projects      []engrammodels.MemoryProject
	recent        map[string][]engrammodels.Memory
	projectsCalls int
	recentCalls   int
}

func (s *countingMemoryStore) Projects(context.Context) ([]engrammodels.MemoryProject, error) {
	s.projectsCalls++
	return s.projects, nil
}

func (s *countingMemoryStore) Recent(_ context.Context, project string, _ int) ([]engrammodels.Memory, error) {
	s.recentCalls++
	return s.recent[project], nil
}

// bridgeWorkspace is a workspace of the bridge tests, with its own ID.
func bridgeWorkspace(t *testing.T, name string, path string) workspacesmodels.Workspace {
	t.Helper()
	id, err := workspacesmodels.NewWorkspaceID()
	require.NoError(t, err)
	return workspacesmodels.Workspace{ID: id, Name: name, Path: path, BaseBranch: "main"}
}

func TestEngramMemoryBridgeMapsTheEngramStates(t *testing.T) {
	cases := map[string]struct {
		engram engrammodels.WorkspaceMemory
		want   workspacesmodels.MemorySummary
	}{
		"available with memories": {
			engram: engrammodels.WorkspaceMemory{State: engrammodels.StateAvailable, Count: 4},
			want:   workspacesmodels.MemorySummary{State: workspacesmodels.MemoryAvailable, Count: 4},
		},
		"matching project without memories": {
			engram: engrammodels.WorkspaceMemory{State: engrammodels.StateAvailable},
			want:   workspacesmodels.MemorySummary{State: workspacesmodels.MemoryEmpty},
		},
		"no project matches the workspace": {
			engram: engrammodels.WorkspaceMemory{State: engrammodels.StateNoMatch},
			want:   workspacesmodels.MemorySummary{State: workspacesmodels.MemoryEmpty},
		},
		"engram unavailable": {
			engram: engrammodels.WorkspaceMemory{State: engrammodels.StateUnavailable},
			want:   workspacesmodels.MemorySummary{State: workspacesmodels.MemoryUnavailable},
		},
		"unknown engram state is unavailable": {
			engram: engrammodels.WorkspaceMemory{State: engrammodels.MemoryState("unknown")},
			want:   workspacesmodels.MemorySummary{State: workspacesmodels.MemoryUnavailable},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			bridge := newEngramMemoryBridge(&fakeWorkspacesMemory{memories: []engrammodels.WorkspaceMemory{tc.engram}})

			summaries, err := bridge.Summaries(context.Background(), []workspacesmodels.Workspace{bridgeWorkspace(t, "Demo", "/repos/demo")})

			require.NoError(t, err)
			require.Len(t, summaries, 1)
			assert.Equal(t, tc.want, summaries[0])
		})
	}
}

func TestEngramMemoryBridgeAsksEngramOnceForTheWorkspacesNamesAndPaths(t *testing.T) {
	fake := &fakeWorkspacesMemory{memories: []engrammodels.WorkspaceMemory{
		{State: engrammodels.StateNoMatch},
		{State: engrammodels.StateNoMatch},
	}}
	bridge := newEngramMemoryBridge(fake)

	_, err := bridge.Summaries(context.Background(), []workspacesmodels.Workspace{
		bridgeWorkspace(t, "Demo", "/repos/demo"),
		bridgeWorkspace(t, "Other", "/repos/other"),
	})

	require.NoError(t, err)
	require.Len(t, fake.batches, 1)
	assert.Equal(t, []engrammodels.Workspace{
		engrammodels.NewWorkspace("Demo", "/repos/demo"),
		engrammodels.NewWorkspace("Other", "/repos/other"),
	}, fake.batches[0])
}

func TestEngramMemoryBridgeReturnsUnexpectedEngramErrors(t *testing.T) {
	boom := errors.New("boom")
	bridge := newEngramMemoryBridge(&fakeWorkspacesMemory{err: boom})

	_, err := bridge.Summaries(context.Background(), []workspacesmodels.Workspace{bridgeWorkspace(t, "Demo", "/repos/demo")})

	require.ErrorIs(t, err, boom)
}

// A page with N workspaces costs one GET /projects and no recent-observation read at all, even when several
// workspaces match the same project.
func TestEngramMemoryBridgeReadsEveryRowWithOneProjectsRequest(t *testing.T) {
	store := &countingMemoryStore{projects: []engrammodels.MemoryProject{
		engrammodels.NewMemoryProject("alpha", 3, []string{"/repos/alpha"}),
		engrammodels.NewMemoryProject("beta", 0, []string{"/repos/beta"}),
	}}
	bridge := newEngramMemoryBridge(engramapp.NewListWorkspacesMemory(store))
	workspaces := []workspacesmodels.Workspace{
		bridgeWorkspace(t, "alpha", "/repos/alpha"),
		bridgeWorkspace(t, "alpha-clone", "/repos/alpha"),
		bridgeWorkspace(t, "beta", "/repos/beta"),
		bridgeWorkspace(t, "gamma", "/repos/gamma"),
	}

	summaries, err := bridge.Summaries(context.Background(), workspaces)

	require.NoError(t, err)
	assert.Equal(t, 1, store.projectsCalls, "one GET /projects for the whole page")
	assert.Zero(t, store.recentCalls, "the chips never read recent observations")
	assert.Equal(t, []workspacesmodels.MemorySummary{
		{State: workspacesmodels.MemoryAvailable, Count: 3},
		{State: workspacesmodels.MemoryAvailable, Count: 3},
		{State: workspacesmodels.MemoryEmpty},
		{State: workspacesmodels.MemoryEmpty},
	}, summaries)
}

// fakeRecentMemories stands in for the engram RecentMemories use case. It answers feed and err, and records the
// workspaces it was asked about.
type fakeRecentMemories struct {
	feed engrammodels.MemoryFeed
	err  error
	seen []engrammodels.Workspace
}

func (f *fakeRecentMemories) Execute(_ context.Context, workspace engrammodels.Workspace) (engrammodels.MemoryFeed, error) {
	f.seen = append(f.seen, workspace)
	return f.feed, f.err
}

func TestEngramTaskMemoryBridgeMapsTheRecentMemories(t *testing.T) {
	t.Parallel()
	lastSeen := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	cases := map[string]struct {
		engram engrammodels.MemoryFeed
		want   tasksmodels.TaskMemoryFeed
	}{
		"available with memories": {
			engram: engrammodels.MemoryFeed{State: engrammodels.StateAvailable, Memories: []engrammodels.Memory{
				engrammodels.NewMemory(1, "decision", "Use sqlc", lastSeen),
			}},
			want: tasksmodels.TaskMemoryFeed{Available: true, Memories: []tasksmodels.TaskMemory{
				{Type: "decision", Title: "Use sqlc", LastSeenAt: lastSeen},
			}},
		},
		"matching project without memories": {
			engram: engrammodels.MemoryFeed{State: engrammodels.StateAvailable},
			want:   tasksmodels.TaskMemoryFeed{Available: true},
		},
		"no project matches the workspace": {
			engram: engrammodels.MemoryFeed{State: engrammodels.StateNoMatch},
			want:   tasksmodels.TaskMemoryFeed{},
		},
		"engram unavailable": {
			engram: engrammodels.MemoryFeed{State: engrammodels.StateUnavailable},
			want:   tasksmodels.TaskMemoryFeed{},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			bridge := newEngramTaskMemoryBridge(&fakeRecentMemories{feed: tc.engram})

			got, err := bridge.Recent(context.Background(), "Demo", "/repos/demo")

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestEngramTaskMemoryBridgeAsksEngramAboutTheWorkspaceNameAndPath(t *testing.T) {
	t.Parallel()
	fake := &fakeRecentMemories{feed: engrammodels.MemoryFeed{State: engrammodels.StateNoMatch}}
	bridge := newEngramTaskMemoryBridge(fake)

	_, err := bridge.Recent(context.Background(), "Demo", "/repos/demo")

	require.NoError(t, err)
	require.Len(t, fake.seen, 1)
	assert.Equal(t, engrammodels.NewWorkspace("Demo", "/repos/demo"), fake.seen[0])
}

func TestEngramTaskMemoryBridgeReturnsUnexpectedEngramErrors(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	bridge := newEngramTaskMemoryBridge(&fakeRecentMemories{err: boom})

	_, err := bridge.Recent(context.Background(), "Demo", "/repos/demo")

	require.ErrorIs(t, err, boom)
}
