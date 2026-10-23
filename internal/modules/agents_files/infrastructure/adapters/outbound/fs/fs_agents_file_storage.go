// Package fs stores the AGENTS.md contents as files on disk.
//
//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package fs

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	agentserrors "iworkflow/internal/modules/agents_files/domain/errors"
	"iworkflow/internal/modules/agents_files/domain/models"
	"iworkflow/internal/modules/agents_files/domain/ports/outbound"
)

// AgentsFileStorage saves each file as <uuid>.md directly inside baseDir. User-supplied names are
// never used, and only paths that sit directly inside baseDir are read or removed.
type AgentsFileStorage struct {
	baseDir string
}

var _ outbound.AgentsFileStorage = (*AgentsFileStorage)(nil)

// NewAgentsFileStorage builds the adapter for the given storage folder.
func NewAgentsFileStorage(baseDir string) *AgentsFileStorage {
	return &AgentsFileStorage{baseDir: baseDir}
}

// Store writes the content through a temporary file and renames it into place, so a reader never
// sees a partial file. It returns the canonical absolute path of the stored file.
func (s *AgentsFileStorage) Store(_ context.Context, id models.AgentsFileID, content []byte) (string, error) {
	if err := os.MkdirAll(s.baseDir, 0o755); err != nil {
		return "", agentserrors.Storage(ioDetail("no se pudo crear", s.baseDir, err))
	}
	base, err := canonicalBase(s.baseDir)
	if err != nil {
		return "", agentserrors.Storage(ioDetail("no se pudo resolver", s.baseDir, err))
	}
	target := filepath.Join(base, id.String()+".md")
	temp := filepath.Join(base, "."+id.String()+".md.tmp")
	if err := os.WriteFile(temp, content, 0o644); err != nil {
		_ = os.Remove(temp) // best effort: the write error is the one worth reporting
		return "", agentserrors.Storage(ioDetail("no se pudo escribir", temp, err))
	}
	if err := os.Rename(temp, target); err != nil {
		_ = os.Remove(temp) // best effort: the rename error is the one worth reporting
		return "", agentserrors.Storage(ioDetail("no se pudo mover a", target, err))
	}
	return target, nil
}

// Read returns the content of a file that lives inside the storage folder.
func (s *AgentsFileStorage) Read(_ context.Context, path string) ([]byte, error) {
	if err := s.ensureInsideBase(path); err != nil {
		return nil, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, agentserrors.Storage(ioDetail("no se pudo leer", path, err))
	}
	return content, nil
}

// Remove deletes a file inside the storage folder. A file that is already gone is not an error.
func (s *AgentsFileStorage) Remove(_ context.Context, path string) error {
	if err := s.ensureInsideBase(path); err != nil {
		return err
	}
	err := os.Remove(path)
	if err == nil || errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return agentserrors.Storage(ioDetail("no se pudo borrar", path, err))
}

// ensureInsideBase accepts only clean paths whose parent is the storage folder, either as
// configured or after resolving symlinks. Anything else, including ".." segments, is rejected.
func (s *AgentsFileStorage) ensureInsideBase(path string) error {
	outside := agentserrors.Storage(fmt.Sprintf("la ruta %s está fuera de la carpeta de almacenamiento", path))
	if filepath.Clean(path) != path {
		return outside
	}
	parent := filepath.Dir(path)
	if parent == filepath.Clean(s.baseDir) {
		return nil
	}
	base, err := canonicalBase(s.baseDir)
	if err != nil || parent != base {
		return outside
	}
	return nil
}

// canonicalBase returns the absolute path of baseDir with every symlink resolved.
func canonicalBase(baseDir string) (string, error) {
	absolute, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path: %w", err)
	}
	return filepath.EvalSymlinks(absolute)
}

func ioDetail(action string, path string, err error) string {
	return fmt.Sprintf("%s %s: %v", action, path, err)
}
