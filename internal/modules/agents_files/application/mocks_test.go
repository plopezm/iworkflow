package application_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sort"
	"time"

	agentserrors "iworkflow/internal/modules/agents_files/domain/errors"
	"iworkflow/internal/modules/agents_files/domain/models"
)

// fixedNow is the instant every test uses as "now".
var fixedNow = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fakeRepository is an in-memory AgentsFileRepository. Set the *Err fields to inject failures.
type fakeRepository struct {
	files     map[string]models.AgentsFile
	listErr   error
	findErr   error
	saveErr   error
	deleteErr error
	saves     int
	deleted   []models.AgentsFileID
}

func newFakeRepository(files ...models.AgentsFile) *fakeRepository {
	repository := &fakeRepository{files: map[string]models.AgentsFile{}}
	for _, file := range files {
		repository.files[file.ID.String()] = file
	}
	return repository
}

func (r *fakeRepository) Save(_ context.Context, file models.AgentsFile) error {
	r.saves++
	if r.saveErr != nil {
		return r.saveErr
	}
	r.files[file.ID.String()] = file
	return nil
}

func (r *fakeRepository) List(_ context.Context) ([]models.AgentsFile, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	listed := make([]models.AgentsFile, 0, len(r.files))
	for _, file := range r.files {
		listed = append(listed, file)
	}
	sort.Slice(listed, func(i, j int) bool { return listed[i].ID.String() < listed[j].ID.String() })
	return listed, nil
}

func (r *fakeRepository) FindByID(_ context.Context, id models.AgentsFileID) (*models.AgentsFile, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	file, ok := r.files[id.String()]
	if !ok {
		return nil, nil
	}
	return &file, nil
}

func (r *fakeRepository) Delete(_ context.Context, id models.AgentsFileID) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	if _, ok := r.files[id.String()]; !ok {
		return agentserrors.NotFound(id.String())
	}
	delete(r.files, id.String())
	r.deleted = append(r.deleted, id)
	return nil
}

// fakeStorage keeps file contents in memory, keyed by path.
type fakeStorage struct {
	contents  map[string][]byte
	storeErr  error
	readErr   error
	removeErr error
	removed   []string
	stores    int
}

func newFakeStorage() *fakeStorage {
	return &fakeStorage{contents: map[string][]byte{}}
}

func (s *fakeStorage) Store(_ context.Context, id models.AgentsFileID, content []byte) (string, error) {
	s.stores++
	if s.storeErr != nil {
		return "", s.storeErr
	}
	path := "/data/" + id.String() + ".md"
	s.contents[path] = content
	return path, nil
}

func (s *fakeStorage) Read(_ context.Context, path string) ([]byte, error) {
	if s.readErr != nil {
		return nil, s.readErr
	}
	content, ok := s.contents[path]
	if !ok {
		return nil, errors.New("missing " + path)
	}
	return content, nil
}

func (s *fakeStorage) Remove(_ context.Context, path string) error {
	s.removed = append(s.removed, path)
	return s.removeErr
}

// installCall records one call to fakeInstaller.Install.
type installCall struct {
	content   []byte
	dir       string
	overwrite bool
}

// fakeInstaller records the installs it receives and answers with outcome or err.
type fakeInstaller struct {
	outcome models.InstallOutcome
	err     error
	calls   []installCall
}

func (i *fakeInstaller) Install(_ context.Context, content []byte, dir string, overwrite bool) (models.InstallOutcome, error) {
	i.calls = append(i.calls, installCall{content: content, dir: dir, overwrite: overwrite})
	if i.err != nil {
		return models.InstallOutcome{}, i.err
	}
	return i.outcome, nil
}

// fakeProjects is a ProjectLookup over a fixed list.
type fakeProjects struct {
	projects []models.ProjectRef
	listErr  error
	findErr  error
}

func (p *fakeProjects) List(_ context.Context) ([]models.ProjectRef, error) {
	if p.listErr != nil {
		return nil, p.listErr
	}
	return p.projects, nil
}

func (p *fakeProjects) Find(_ context.Context, id string) (*models.ProjectRef, error) {
	if p.findErr != nil {
		return nil, p.findErr
	}
	for index := range p.projects {
		if p.projects[index].ID == id {
			return &p.projects[index], nil
		}
	}
	return nil, nil
}
