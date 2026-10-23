package application_test

import (
	"context"
	"path/filepath"

	"iworkflow/internal/modules/agent_settings/domain/models"
)

// fakeRepository is an in-memory AgentSettingsRepositoryPort that records what was saved.
type fakeRepository struct {
	settings models.AgentSettings
	getErr   error
	saveErr  error
	saved    []models.AgentSettings
}

func (f *fakeRepository) Get(_ context.Context) (models.AgentSettings, error) {
	return f.settings, f.getErr
}

func (f *fakeRepository) Save(_ context.Context, settings models.AgentSettings) error {
	f.saved = append(f.saved, settings)
	return f.saveErr
}

// fakeModelCatalog returns a fixed list and records the runtimes it was asked for.
type fakeModelCatalog struct {
	models   []models.ModelRef
	err      error
	requests []models.AgentRuntime
}

func (f *fakeModelCatalog) List(_ context.Context, runtime models.AgentRuntime) ([]models.ModelRef, error) {
	f.requests = append(f.requests, runtime)
	return f.models, f.err
}

// fakeProfileCatalog returns a fixed catalog and counts the lookups.
type fakeProfileCatalog struct {
	catalog models.ProfileCatalog
	err     error
	calls   int
}

func (f *fakeProfileCatalog) List(_ context.Context) (models.ProfileCatalog, error) {
	f.calls++
	return f.catalog, f.err
}

// pinCall is one invocation of the profile pin port.
type pinCall struct {
	repoDir string
	profile *models.ProfileName
}

// fakePin records every pin applied.
type fakePin struct {
	calls []pinCall
	err   error
}

func (f *fakePin) Apply(_ context.Context, repoDir string, profile *models.ProfileName) error {
	f.calls = append(f.calls, pinCall{repoDir: filepath.Clean(repoDir), profile: profile})
	return f.err
}
