package di_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
	"iworkflow/internal/modules/tasks/infrastructure/di"
)

// unusedStart is the start use case of the wiring tests; none of them starts a task.
type unusedStart struct{}

func (unusedStart) Start(context.Context, models.TaskID) (models.Task, error) {
	return models.Task{}, errors.New("start is not used by the wiring tests")
}

// noWorkspaces knows no workspace; the wiring tests never look one up.
type noWorkspaces struct{}

func (noWorkspaces) Find(context.Context, string) (*models.WorkspaceRef, error) { return nil, nil }

func (noWorkspaces) List(context.Context) ([]models.WorkspaceRef, error) { return nil, nil }

// stubRuntime and stubSettings only satisfy the ports; the wiring never calls them.
type stubRuntime struct {
	outbound.AgentRuntime
}

type stubSettings struct {
	outbound.LaunchSettings
}

// baseDeps is the composition every existing caller provides: no agent runtime.
func baseDeps(t *testing.T) di.Deps {
	t.Helper()
	return di.Deps{
		DB:             dbtest.New(t),
		Workspaces:     noWorkspaces{},
		AttachmentsDir: t.TempDir(),
		WorktreesDir:   t.TempDir(),
		Start:          unusedStart{},
	}
}

// Rule from source: the resume use cases exist only where a runtime is provided. Callers that do not provide one
// (the composition before slice F) keep the module exactly as it was: no resume use case, no error.
func TestNewModuleLeavesTheResumeUseCasesUnwiredWithoutARuntime(t *testing.T) {
	t.Parallel()

	module, err := di.NewModule(baseDeps(t))

	require.NoError(t, err)
	assert.Nil(t, module.ResumeTask)
	assert.Nil(t, module.ResumeRateLimited)
}

// Rule from source: a resume needs both the runtime that restarts the agent and the launch settings that prepare it;
// a partial composition is rejected before it can start.
func TestNewModuleRejectsARuntimeWithoutItsLaunchSettingsAndTheReverse(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		runtime  outbound.AgentRuntime
		settings outbound.LaunchSettings
	}{
		{name: "runtime without launch settings", runtime: stubRuntime{}},
		{name: "launch settings without runtime", settings: stubSettings{}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			deps := baseDeps(t)
			deps.Runtime = testCase.runtime
			deps.LaunchSettings = testCase.settings

			module, err := di.NewModule(deps)

			require.Error(t, err)
			assert.Nil(t, module)
		})
	}
}

// Rule from source: tasks_context.rs builds ResumeTask and ResumeRateLimitedTasks over the same lifecycle; the
// composition exposes both when the runtime and the launch settings are provided.
func TestNewModuleWiresTheResumeUseCasesWhenARuntimeIsProvided(t *testing.T) {
	t.Parallel()
	deps := baseDeps(t)
	deps.Runtime = stubRuntime{}
	deps.LaunchSettings = stubSettings{}

	module, err := di.NewModule(deps)

	require.NoError(t, err)
	assert.NotNil(t, module.ResumeTask)
	assert.NotNil(t, module.ResumeRateLimited)
}
