package agents_files_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	agentserrors "iworkflow/internal/modules/agents_files/domain/errors"
	"iworkflow/internal/modules/agents_files/domain/models"
	"iworkflow/internal/modules/agents_files/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/agents_files/infrastructure/di"
)

// staticProjects is a ProjectLookup over a fixed list, standing in for the workspaces module.
type staticProjects struct {
	projects []models.ProjectRef
}

func (p staticProjects) List(_ context.Context) ([]models.ProjectRef, error) {
	return p.projects, nil
}

func (p staticProjects) Find(_ context.Context, id string) (*models.ProjectRef, error) {
	for index := range p.projects {
		if p.projects[index].ID == id {
			return &p.projects[index], nil
		}
	}
	return nil, nil
}

// TestModuleUploadListApplyDeleteFlow runs the whole vertical slice against a migrated database,
// a real storage folder and a real git repository.
func TestModuleUploadListApplyDeleteFlow(t *testing.T) {
	ctx := context.Background()
	root := gitRepo(t)
	storageDir := filepath.Join(t.TempDir(), "agents-files")
	module := di.NewModule(di.Deps{
		DB:         dbtest.New(t),
		StorageDir: storageDir,
		Projects:   staticProjects{projects: []models.ProjectRef{{ID: "p1", Name: "demo", Path: root}}},
		Logger:     quietLogger(),
	})

	uploaded, err := module.Upload.Execute(ctx, models.NewAgentsFile{Description: "  Reglas ", Content: []byte("# reglas\n")})
	require.NoError(t, err)
	require.Equal(t, "Reglas", uploaded.Description)

	listed, err := module.List.Execute(ctx)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Equal(t, dto.FromAgentsFile(uploaded), dto.FromAgentsFile(listed[0]))

	outcome, err := module.Apply.Execute(ctx, uploaded.ID, "p1", false)
	require.NoError(t, err)
	require.True(t, outcome.Committed)
	require.Equal(t, "docs: add AGENTS.md", runGitCmd(t, root, "log", "-1", "--format=%s"))

	_, err = module.Apply.Execute(ctx, uploaded.ID, "p1", false)
	require.Equal(t, agentserrors.TargetExists(filepath.Join(root, "AGENTS.md")), err)

	require.NoError(t, module.Delete.Execute(ctx, uploaded.ID))
	listed, err = module.List.Execute(ctx)
	require.NoError(t, err)
	require.Empty(t, listed)
	require.NoFileExists(t, uploaded.Path)
	require.FileExists(t, filepath.Join(root, "AGENTS.md"), "deleting the reference keeps installed copies")
}

func TestModuleListProjectsReturnsTheLookupProjects(t *testing.T) {
	module := di.NewModule(di.Deps{
		DB:         dbtest.New(t),
		StorageDir: t.TempDir(),
		Projects:   staticProjects{projects: []models.ProjectRef{{ID: "p1", Name: "demo", Path: "/repos/demo"}}},
		Logger:     quietLogger(),
	})

	projects, err := module.ListProjects.Execute(context.Background())

	require.NoError(t, err)
	require.Equal(t, []models.ProjectRef{{ID: "p1", Name: "demo", Path: "/repos/demo"}}, projects)
}

func TestModuleUploadRejectsInvalidContentWithoutLeavingAFile(t *testing.T) {
	storageDir := filepath.Join(t.TempDir(), "agents-files")
	module := di.NewModule(di.Deps{
		DB:         dbtest.New(t),
		StorageDir: storageDir,
		Projects:   staticProjects{},
		Logger:     quietLogger(),
	})

	_, err := module.Upload.Execute(context.Background(), models.NewAgentsFile{Description: "d", Content: []byte{0xff}})

	require.Equal(t, agentserrors.NotUTF8(), err)
	_, statErr := os.Stat(storageDir)
	require.True(t, os.IsNotExist(statErr), "invalid input must not create the storage folder")
}
