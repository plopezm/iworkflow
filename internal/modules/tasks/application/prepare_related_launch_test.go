package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
	"iworkflow/internal/modules/tasks/domain/models"
)

// fakeRelatedProbe answers the probe questions from fixed values and records the projects asked about.
type fakeRelatedProbe struct {
	codegraph map[string]bool
	engram    bool
	asked     []string
}

func (p *fakeRelatedProbe) HasCodegraph(_ context.Context, projectPath string) bool {
	p.asked = append(p.asked, projectPath)
	return p.codegraph[projectPath]
}

func (p *fakeRelatedProbe) EngramAvailable(_ context.Context) bool {
	return p.engram
}

// relatedWorkspaces knows two related projects, "ws-api" and "ws-web", with their paths.
func relatedWorkspaces() knownWorkspaces {
	return knownWorkspaces{
		ownWorkspace: {ID: ownWorkspace, Name: "demo", Path: "/repos/demo", BaseBranch: "main"},
		"ws-api":     {ID: "ws-api", Name: "api", Path: "/repos/api", BaseBranch: "main"},
		"ws-web":     {ID: "ws-web", Name: "web", Path: "/repos/web", BaseBranch: "main"},
	}
}

// prepareWith wires PrepareRelatedLaunch over the given related store, workspaces and probe.
func prepareWith(related *fakeRelatedProjects, known knownWorkspaces, probe *fakeRelatedProbe) *application.PrepareRelatedLaunch {
	return application.NewPrepareRelatedLaunch(application.NewListTaskRelatedProjects(related, known), probe)
}

func TestPrepareRelatedLaunchWithoutRelatedProjectsChangesNothing(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		stored []string
	}{
		{name: "no related projects stored", stored: nil},
		{name: "only a related project that no longer exists", stored: []string{"ws-gone"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			task := newTask(models.StatusBacklog)
			related := newFakeRelatedProjects()
			related.stored[task.ID.String()] = tt.stored
			probe := &fakeRelatedProbe{engram: true}

			launch := prepareWith(related, relatedWorkspaces(), probe).Execute(context.Background(), task.ID)

			assert.Empty(t, launch.PromptSection)
			assert.Empty(t, launch.AddDirs)
			assert.Empty(t, probe.asked, "no project means nothing is probed")
		})
	}
}

func TestPrepareRelatedLaunchWithProjectsDescribesThemAndAllowsTheirFolders(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		codegraph   map[string]bool
		engram      bool
		wantInText  []string
		wantMissing []string
	}{
		{
			name:        "plain projects",
			wantInText:  []string{"`@api` -> `/repos/api` (api)", "`@web` -> `/repos/web` (web)"},
			wantMissing: []string{"codegraph index available", "mem_current_project"},
		},
		{
			name:        "codegraph only on the project that has it",
			codegraph:   map[string]bool{"/repos/web": true},
			wantInText:  []string{"`@web` -> `/repos/web` (web) - codegraph index available", "`projectPath`"},
			wantMissing: []string{"`@api` -> `/repos/api` (api) - codegraph", "mem_current_project"},
		},
		{
			name:        "engram available",
			engram:      true,
			wantInText:  []string{"`mem_current_project`", "`mem_search`"},
			wantMissing: []string{"codegraph"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			task := newTask(models.StatusBacklog)
			related := newFakeRelatedProjects()
			related.stored[task.ID.String()] = []string{"ws-web", "ws-api"}
			probe := &fakeRelatedProbe{codegraph: tt.codegraph, engram: tt.engram}

			launch := prepareWith(related, relatedWorkspaces(), probe).Execute(context.Background(), task.ID)

			assert.Equal(t, []string{"/repos/web", "/repos/api"}, launch.AddDirs, "the stored order is kept")
			assert.Equal(t, []string{"/repos/web", "/repos/api"}, probe.asked, "every project is probed by its path")
			require.NotEmpty(t, launch.PromptSection)
			assert.Contains(t, launch.PromptSection, "## Related projects")
			for _, want := range tt.wantInText {
				assert.Contains(t, launch.PromptSection, want)
			}
			for _, unwanted := range tt.wantMissing {
				assert.NotContains(t, launch.PromptSection, unwanted)
			}
		})
	}
}

func TestPrepareRelatedLaunchSkipsMissingProjectsInAddDirs(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	related := newFakeRelatedProjects()
	related.stored[task.ID.String()] = []string{"ws-gone", "ws-api"}

	launch := prepareWith(related, relatedWorkspaces(), &fakeRelatedProbe{}).Execute(context.Background(), task.ID)

	assert.Equal(t, []string{"/repos/api"}, launch.AddDirs)
	assert.Contains(t, launch.PromptSection, "`@api`")
	assert.NotContains(t, launch.PromptSection, "gone")
}

// A related project whose path is blank has no folder to give the agent: it is in neither the prompt nor the
// folders, and the codegraph probe never asks about it.
func TestPrepareRelatedLaunchLeavesOutAProjectWithoutPath(t *testing.T) {
	t.Parallel()
	known := relatedWorkspaces()
	known["ws-blank"] = models.WorkspaceRef{ID: "ws-blank", Name: "blank", Path: " \t ", BaseBranch: "main"}
	task := newTask(models.StatusBacklog)
	related := newFakeRelatedProjects()
	related.stored[task.ID.String()] = []string{"ws-blank", "ws-api"}
	probe := &fakeRelatedProbe{}

	launch := prepareWith(related, known, probe).Execute(context.Background(), task.ID)

	assert.Equal(t, []string{"/repos/api"}, launch.AddDirs, "an empty --add-dir value would be wrong")
	assert.Contains(t, launch.PromptSection, "`@api`")
	assert.NotContains(t, launch.PromptSection, "blank")
	assert.Equal(t, []string{"/repos/api"}, probe.asked)
}

func TestPrepareRelatedLaunchWithOnlyBlankPathsChangesNothing(t *testing.T) {
	t.Parallel()
	known := knownWorkspaces{ownWorkspace: {ID: ownWorkspace, Name: "demo", Path: "/repos/demo", BaseBranch: "main"}, "ws-blank": {ID: "ws-blank", Name: "blank", Path: ""}}
	task := newTask(models.StatusBacklog)
	related := newFakeRelatedProjects()
	related.stored[task.ID.String()] = []string{"ws-blank"}

	launch := prepareWith(related, known, &fakeRelatedProbe{engram: true}).Execute(context.Background(), task.ID)

	assert.Empty(t, launch.PromptSection)
	assert.Empty(t, launch.AddDirs)
}

func TestPrepareRelatedLaunchListFailureStartsWithoutProjects(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	related := newFakeRelatedProjects()
	related.stored[task.ID.String()] = []string{"ws-api"}
	related.failList = true

	launch := prepareWith(related, relatedWorkspaces(), &fakeRelatedProbe{engram: true}).Execute(context.Background(), task.ID)

	assert.Empty(t, launch.PromptSection)
	assert.Empty(t, launch.AddDirs)
}
