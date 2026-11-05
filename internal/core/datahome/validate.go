package datahome

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	// ErrSameFolder is returned when the destination is the current data folder.
	ErrSameFolder = errors.New("the destination is the current data folder")
	// ErrNested is returned when one of the two folders is inside the other.
	ErrNested = errors.New("the destination and the current data folder cannot contain each other")
	// ErrNotEmpty is returned when the destination exists and is not an empty folder.
	ErrNotEmpty = errors.New("the destination must be a new or empty folder")
	// ErrNotWritable is returned when the destination cannot be created or written.
	ErrNotWritable = errors.New("the destination is not writable")
	// ErrNotDataFolder is returned when the current folder holds no database.
	ErrNotDataFolder = errors.New("the current folder is not an iworkflow data folder")
	// ErrUnsafeSource is returned for a source that must never be moved or deleted (a root, the user home or a parent of it).
	ErrUnsafeSource = errors.New("the current folder is too broad to move safely")
	// ErrNoSpace is returned when the destination disk cannot hold the data.
	ErrNoSpace = errors.New("not enough free space at the destination")
)

// DatabaseFile is the file that identifies a data folder.
const DatabaseFile string = "workspaces.db"

// Validate checks that source can be moved to dest. It reads nothing but metadata.
func Validate(source string, dest string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return ValidateWithHome(source, dest, home)
}

// ValidateWithHome is Validate with the user home given, so tests do not depend on the machine.
func ValidateWithHome(source string, dest string, home string) error {
	if !filepath.IsAbs(source) || !filepath.IsAbs(dest) {
		return fmt.Errorf("%w: %q", ErrNotAbsolute, dest)
	}
	realSource, realDest := resolveExisting(filepath.Clean(source)), resolveExisting(filepath.Clean(dest))
	if isUnsafeSource(realSource, resolveExisting(home)) {
		return fmt.Errorf("%w: %s", ErrUnsafeSource, source)
	}
	if _, err := os.Stat(filepath.Join(source, DatabaseFile)); err != nil {
		return fmt.Errorf("%w: %s", ErrNotDataFolder, source)
	}
	if realSource == realDest {
		return ErrSameFolder
	}
	if within(realSource, realDest) || within(realDest, realSource) {
		return ErrNested
	}
	if err := checkEmptyOrMissing(dest); err != nil {
		return err
	}
	return checkWritable(dest)
}

// isUnsafeSource reports whether moving, and above all deleting, source could reach beyond iworkflow's
// data: a filesystem root, the user home, or any folder that holds the home.
func isUnsafeSource(source string, home string) bool {
	if filepath.Dir(source) == source {
		return true
	}
	return home != "" && (source == home || within(source, home))
}

func checkEmptyOrMissing(dest string) error {
	info, err := os.Stat(dest)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: %w", ErrNotWritable, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: %s is a file", ErrNotEmpty, dest)
	}
	entries, err := os.ReadDir(dest)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrNotWritable, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("%w: %s", ErrNotEmpty, dest)
	}
	return nil
}

// checkWritable proves a file can be created where the destination will be: in the folder itself when it
// exists, else in the nearest existing ancestor.
func checkWritable(dest string) error {
	probeDir := nearestExisting(dest)
	probe, err := os.CreateTemp(probeDir, ".iwf-write-test-*")
	if err != nil {
		return fmt.Errorf("%w: %w", ErrNotWritable, err)
	}
	name := probe.Name()
	_ = probe.Close() // the probe is empty: a close error cannot lose data
	if err := os.Remove(name); err != nil {
		return fmt.Errorf("%w: %w", ErrNotWritable, err)
	}
	return nil
}

// CheckSpace verifies the disk of dest can take needed bytes plus a safety margin of 5%.
func CheckSpace(dest string, needed int64) error {
	free, err := FreeSpace(nearestExisting(dest))
	if err != nil {
		return fmt.Errorf("free space of %s: %w", dest, err)
	}
	required := uint64(needed) + uint64(needed)/20
	if free < required {
		return fmt.Errorf("%w: %d bytes free, %d needed", ErrNoSpace, free, required)
	}
	return nil
}

// nearestExisting returns path when it exists, else its closest existing ancestor.
func nearestExisting(path string) string {
	current := filepath.Clean(path)
	for {
		if _, err := os.Stat(current); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return current
		}
		current = parent
	}
}

// resolveExisting resolves symbolic links in the part of path that exists and keeps the rest as is, so a
// destination that does not exist yet is still compared by where it will really be.
func resolveExisting(path string) string {
	if path == "" {
		return ""
	}
	existing := nearestExisting(path)
	resolved, err := filepath.EvalSymlinks(existing)
	if err != nil {
		resolved = existing
	}
	rest, err := filepath.Rel(existing, filepath.Clean(path))
	if err != nil || rest == "." {
		return resolved
	}
	return filepath.Join(resolved, rest)
}

// within reports whether child is inside parent (not equal to it).
func within(parent string, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil || rel == "." {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
