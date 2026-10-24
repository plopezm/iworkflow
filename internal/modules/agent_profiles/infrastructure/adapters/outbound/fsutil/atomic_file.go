// Package fsutil holds the file helpers that the agent_profiles file adapters share.
//
//nolint:wrapcheck // Failures are returned as domain errors: their text is user-visible and must stay identical to the Rust version.
package fsutil

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"unicode/utf8"

	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
)

const (
	maxTempAttempts = 100
	createMode      = 0o666
	dirMode         = 0o755
)

// AtomicWrite writes content to a hidden temporary sibling of target and renames it over target.
// The parent folder is created when missing. A failed write removes its temporary file.
func AtomicWrite(target string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(target), dirMode); err != nil {
		return agenterrors.IO(err.Error())
	}
	temp, file, err := createTemp(target)
	if err != nil {
		return err
	}
	if written := writeAndClose(file, content); written != nil {
		_ = os.Remove(temp) // best effort: the write error is the one worth reporting
		return agenterrors.IO(written.Error())
	}
	if err := os.Rename(temp, target); err != nil {
		_ = os.Remove(temp) // best effort: the rename error is the one worth reporting
		return agenterrors.IO(err.Error())
	}
	return nil
}

// ReadOptional reads a file as UTF-8 text. found is false when the file does not exist.
func ReadOptional(path string) (content []byte, found bool, err error) {
	content, err = os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, agenterrors.IO(err.Error())
	}
	if !utf8.Valid(content) {
		return nil, false, agenterrors.IO("stream did not contain valid UTF-8")
	}
	return content, true, nil
}

// createTemp creates a new hidden file next to target. The name carries the process id and an
// attempt index, and O_EXCL makes a clash with another writer a retry instead of a corruption.
func createTemp(target string) (string, *os.File, error) {
	dir := filepath.Dir(target)
	base := filepath.Base(target)
	for attempt := range maxTempAttempts {
		temp := filepath.Join(dir, fmt.Sprintf(".%s.%d.%d.tmp", base, os.Getpid(), attempt))
		file, err := os.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, createMode)
		if err == nil {
			return temp, file, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return "", nil, agenterrors.IO(err.Error())
		}
	}
	return "", nil, agenterrors.IO("no free temporary name next to " + target)
}

// writeAndClose writes content and closes the file, returning the first failure.
func writeAndClose(file *os.File, content []byte) error {
	_, writeErr := file.Write(content)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}
