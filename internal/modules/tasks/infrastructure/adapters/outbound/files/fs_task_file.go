// Package files reads and overwrites the text files of a task worktree.
//
//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package files

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// gitFolderName is the metadata folder of a worktree. It is never readable or writable here.
const gitFolderName = ".git"

// FsTaskFile reads and overwrites existing text files inside a worktree. It never creates files or folders.
type FsTaskFile struct{}

var _ outbound.TaskFile = FsTaskFile{}

// NewFsTaskFile returns the adapter. It has no state.
func NewFsTaskFile() FsTaskFile {
	return FsTaskFile{}
}

// Read returns the text of the file rel inside root. It rejects paths that escape root, the .git folder,
// files larger than the limit, and files that are not UTF-8.
func (FsTaskFile) Read(_ context.Context, root string, rel string) (models.TaskFileContent, error) {
	target, size, err := resolveExisting(root, rel)
	if err != nil {
		return models.TaskFileContent{}, err
	}
	if size > models.MaxTaskFileBytes {
		return models.TaskFileContent{}, notEditable(rel, "supera el tamaño máximo de 1 MiB")
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return models.TaskFileContent{}, werrors.Storage(rel + ": " + err.Error())
	}
	if !utf8.Valid(data) {
		return models.TaskFileContent{}, notEditable(rel, "es binario o no es UTF-8")
	}
	return models.TaskFileContent{Path: rel, Content: string(data)}, nil
}

// Write overwrites the existing file rel inside root. The file is opened without O_CREATE, so a missing
// file is an error and no directory is ever created.
func (FsTaskFile) Write(_ context.Context, root string, rel string, content string) error {
	if int64(len(content)) > models.MaxTaskFileBytes {
		return notEditable(rel, "supera el tamaño máximo de 1 MiB")
	}
	target, _, err := resolveExisting(root, rel)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return werrors.Storage(rel + ": " + err.Error())
	}
	_, writeErr := file.WriteString(content)
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return werrors.Storage(rel + ": " + err.Error())
	}
	return nil
}

// ValidateRelativePath checks a user-supplied relative path lexically: no absolute path, no "..", no
// .git folder, no NUL byte, and at least one name. It returns the cleaned path.
func ValidateRelativePath(rel string) (string, error) {
	invalid := func(reason string) error {
		return werrors.InvalidFilePath(fmt.Sprintf("%q: %s", rel, reason))
	}
	if rel == "" {
		return "", invalid("vacía")
	}
	if strings.ContainsRune(rel, 0) {
		return "", invalid("contiene un byte nulo")
	}
	if strings.HasPrefix(rel, "/") {
		return "", invalid("es absoluta")
	}
	var parts []string
	for _, part := range strings.Split(rel, "/") {
		switch part {
		case "", ".":
			continue
		case "..":
			return "", invalid("contiene '..'")
		case gitFolderName:
			return "", invalid("apunta a .git")
		default:
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		return "", invalid("vacía")
	}
	return filepath.Join(parts...), nil
}

// resolveExisting resolves rel to an existing regular file whose real path (symbolic links followed) stays
// inside the real path of root.
func resolveExisting(root string, rel string) (string, int64, error) {
	relative, err := ValidateRelativePath(rel)
	if err != nil {
		return "", 0, err
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", 0, werrors.Storage("raíz del worktree: " + err.Error())
	}
	target, err := filepath.EvalSymlinks(filepath.Join(realRoot, relative))
	if errors.Is(err, fs.ErrNotExist) {
		return "", 0, notEditable(rel, "el fichero no existe")
	}
	if err != nil {
		return "", 0, werrors.Storage(rel + ": " + err.Error())
	}
	inside, err := filepath.Rel(realRoot, target)
	if err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
		return "", 0, werrors.InvalidFilePath(fmt.Sprintf("%q: sale del worktree", rel))
	}
	for _, part := range strings.Split(inside, string(filepath.Separator)) {
		if part == gitFolderName {
			return "", 0, werrors.InvalidFilePath(fmt.Sprintf("%q: apunta a .git", rel))
		}
	}
	info, err := os.Stat(target)
	if err != nil {
		return "", 0, werrors.Storage(rel + ": " + err.Error())
	}
	if !info.Mode().IsRegular() {
		return "", 0, notEditable(rel, "no es un fichero regular")
	}
	return target, info.Size(), nil
}

func notEditable(rel string, reason string) error {
	return werrors.FileNotEditable(rel + ": " + reason)
}
