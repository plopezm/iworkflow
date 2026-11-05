package datahome

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// ErrVerify is returned when a copied tree does not match its manifest.
var ErrVerify = errors.New("the copy does not match the original")

const ownerAccess fs.FileMode = 0o700

// Entry is one folder, file or symbolic link of a data folder, by path relative to its root.
type Entry struct {
	Rel     string
	Mode    fs.FileMode
	Size    int64
	Link    string
	ModTime time.Time
	// Sum is the SHA-256 of a regular file, recorded by Copy from the bytes it read and checked by Verify
	// against the bytes it finds. It is only meaningful when Hashed is true.
	Sum    [sha256.Size]byte
	Hashed bool
}

func (e Entry) isDir() bool     { return e.Mode.IsDir() }
func (e Entry) isSymlink() bool { return e.Mode&fs.ModeSymlink != 0 }

// Manifest is the exact list of what a data folder holds at scan time. Copy, Verify and Prune all work
// from it, so nothing that appears later is ever copied or deleted by mistake.
type Manifest struct {
	entries []Entry
	// Skipped are the paths the scan found but cannot copy (sockets, pipes, devices); they stay behind.
	Skipped []string
}

// Entries returns the entries, parents before their children.
func (m Manifest) Entries() []Entry { return slices.Clone(m.entries) }

// Names returns the relative path of every entry.
func (m Manifest) Names() []string {
	names := make([]string, 0, len(m.entries))
	for _, entry := range m.entries {
		names = append(names, entry.Rel)
	}
	return names
}

// TotalSize is the number of bytes the regular files hold.
func (m Manifest) TotalSize() int64 {
	var total int64
	for _, entry := range m.entries {
		if entry.Mode.IsRegular() {
			total += entry.Size
		}
	}
	return total
}

// Unhashed lists the regular files Copy recorded no checksum of. Verify compares the content only of
// files with a checksum, so a copy is fully verified when this list is empty.
func (m Manifest) Unhashed() []string {
	var names []string
	for _, entry := range m.entries {
		if entry.Mode.IsRegular() && !entry.Hashed {
			names = append(names, entry.Rel)
		}
	}
	return names
}

// Only returns the manifest reduced to the entries keep accepts.
func (m Manifest) Only(keep func(Entry) bool) Manifest {
	reduced := Manifest{Skipped: slices.Clone(m.Skipped)}
	for _, entry := range m.entries {
		if keep(entry) {
			reduced.entries = append(reduced.entries, entry)
		}
	}
	return reduced
}

// Rebased returns the manifest as it must look under a new root: an absolute symbolic link that points
// into from now points into to. Relative links and links elsewhere are unchanged. Copy and Verify take
// the rebased manifest; Prune takes the original one.
func (m Manifest) Rebased(from string, to string) Manifest {
	rebased := Manifest{entries: slices.Clone(m.entries), Skipped: slices.Clone(m.Skipped)}
	for index, entry := range rebased.entries {
		if !entry.isSymlink() || !filepath.IsAbs(entry.Link) {
			continue
		}
		if rel, err := filepath.Rel(from, entry.Link); err == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))) {
			rebased.entries[index].Link = filepath.Join(to, rel)
		}
	}
	return rebased
}

// Scan lists root without following symbolic links. skip names the relative paths (and, for a folder,
// everything below it) to leave out.
func Scan(root string, skip func(rel string) bool) (Manifest, error) {
	var manifest Manifest
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved // a data folder reached through a link is scanned, not mistaken for an empty link
	}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("scan %s: %w", path, walkErr)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("relative path of %s: %w", path, err)
		}
		if rel == "." {
			return nil
		}
		if skip != nil && skip(rel) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat %s: %w", path, err)
		}
		item := Entry{Rel: rel, Mode: info.Mode(), Size: info.Size(), ModTime: info.ModTime()}
		switch {
		case info.IsDir():
			item.Size = 0
		case info.Mode()&fs.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("read link %s: %w", path, err)
			}
			item.Link, item.Size = target, 0
		case !info.Mode().IsRegular():
			manifest.Skipped = append(manifest.Skipped, rel)
			return nil
		}
		manifest.entries = append(manifest.entries, item)
		return nil
	})
	if err != nil {
		return Manifest{}, fmt.Errorf("list %s: %w", root, err)
	}
	return manifest, nil
}

// Copy reproduces the manifest under dst: file contents flushed to disk, permissions and times kept,
// symbolic links recreated as links. It never overwrites an existing file and never changes src.
func Copy(ctx context.Context, src string, dst string, manifest Manifest) error {
	return CopyProgress(ctx, src, dst, manifest, 0, nil)
}

// CopyProgress is Copy that reports the bytes copied so far (and the total) to progress, at most once per
// every, and once more at the end. A nil progress reports nothing.
func CopyProgress(ctx context.Context, src string, dst string, manifest Manifest, every time.Duration, progress func(copied int64, total int64)) error {
	total := manifest.TotalSize()
	var copied int64
	lastReport := time.Now()
	if err := os.MkdirAll(dst, ownerAccess); err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	for index := range manifest.entries {
		// The manifest shares its entries with the caller: Copy records the checksum of each file there.
		entry := &manifest.entries[index]
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("copy stopped: %w", err)
		}
		from, to := filepath.Join(src, entry.Rel), filepath.Join(dst, entry.Rel)
		var err error
		switch {
		case entry.isDir():
			// Owner access while filling it; the recorded permissions are applied at the end.
			err = os.MkdirAll(to, ownerAccess)
		case entry.isSymlink():
			err = os.Symlink(entry.Link, to)
		default:
			err = copyFile(from, to, entry)
			copied += entry.Size
		}
		if err != nil {
			return fmt.Errorf("copy %s: %w", entry.Rel, err)
		}
		if progress != nil && time.Since(lastReport) >= every {
			progress(copied, total)
			lastReport = time.Now()
		}
	}
	if progress != nil {
		progress(copied, total)
	}
	// Children first, so a read-only folder is sealed only after everything below it is in place.
	for _, entry := range slices.Backward(manifest.entries) {
		if !entry.isDir() {
			continue
		}
		if err := os.Chmod(filepath.Join(dst, entry.Rel), entry.Mode.Perm()); err != nil {
			return fmt.Errorf("set permissions of %s: %w", entry.Rel, err)
		}
	}
	return nil
}

func copyFile(from string, to string, entry *Entry) (err error) {
	in, err := os.Open(from)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer func() { _ = in.Close() }() // read-only handle: nothing to recover from a close error
	out, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, ownerAccess)
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}
	defer func() {
		if closeErr := out.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("close: %w", closeErr)
		}
	}()
	hasher := sha256.New()
	if _, err = io.Copy(io.MultiWriter(out, hasher), in); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	copy(entry.Sum[:], hasher.Sum(nil))
	entry.Hashed = true
	if err = out.Sync(); err != nil {
		return fmt.Errorf("flush: %w", err)
	}
	if err = out.Chmod(entry.Mode.Perm()); err != nil {
		return fmt.Errorf("permissions: %w", err)
	}
	if err = os.Chtimes(to, entry.ModTime, entry.ModTime); err != nil {
		return fmt.Errorf("times: %w", err)
	}
	return nil
}

// Verify checks that root holds every entry of the manifest with the same kind, size, permissions and
// link target, and the same content for the files Copy recorded a checksum of.
func Verify(root string, manifest Manifest) error {
	for _, entry := range manifest.entries {
		info, err := os.Lstat(filepath.Join(root, entry.Rel))
		if err != nil {
			return fmt.Errorf("%w: %s: %w", ErrVerify, entry.Rel, err)
		}
		if info.Mode().Type() != entry.Mode.Type() {
			return fmt.Errorf("%w: %s changed kind", ErrVerify, entry.Rel)
		}
		switch {
		case entry.isSymlink():
			target, err := os.Readlink(filepath.Join(root, entry.Rel))
			if err != nil || target != entry.Link {
				return fmt.Errorf("%w: %s points elsewhere", ErrVerify, entry.Rel)
			}
		case entry.Mode.IsRegular():
			if info.Size() != entry.Size || info.Mode().Perm() != entry.Mode.Perm() {
				return fmt.Errorf("%w: %s has a different size or permissions", ErrVerify, entry.Rel)
			}
			if entry.Hashed {
				sum, err := fileSum(filepath.Join(root, entry.Rel))
				if err != nil || sum != entry.Sum {
					return fmt.Errorf("%w: %s has different content", ErrVerify, entry.Rel)
				}
			}
		}
	}
	return nil
}

// Prune deletes exactly the manifest entries under root, never following links, then every folder that
// became empty, root included. Anything the manifest does not list stays.
func Prune(root string, manifest Manifest) error {
	var first error
	remember := func(err error) {
		if err != nil && !errors.Is(err, fs.ErrNotExist) && first == nil {
			first = err
		}
	}
	for _, entry := range manifest.entries {
		if entry.isDir() {
			// A read-only folder could not lose its children.
			_ = os.Chmod(filepath.Join(root, entry.Rel), ownerAccess) // best effort: a failure shows up as the removal error
		}
	}
	for _, entry := range manifest.entries {
		if !entry.isDir() {
			remember(os.Remove(filepath.Join(root, entry.Rel)))
		}
	}
	for _, entry := range slices.Backward(manifest.entries) {
		if entry.isDir() {
			remember(removeIfEmpty(filepath.Join(root, entry.Rel)))
		}
	}
	remember(removeIfEmpty(root))
	if first != nil {
		return fmt.Errorf("remove the old folder: %w", first)
	}
	return nil
}

// removeIfEmpty removes a folder only when nothing is left in it.
func removeIfEmpty(path string) error {
	err := os.Remove(path)
	// fs.ErrExist matches "directory not empty" on every platform: ENOTEMPTY and EEXIST on Unix,
	// ERROR_DIR_NOT_EMPTY on Windows (where syscall.ENOTEMPTY is not what the system returns).
	if errors.Is(err, fs.ErrExist) {
		return nil
	}
	return err //nolint:wrapcheck // The caller wraps it with the remove context.
}

func fileSum(path string) ([sha256.Size]byte, error) {
	var sum [sha256.Size]byte
	file, err := os.Open(path)
	if err != nil {
		return sum, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = file.Close() }() // read-only handle: nothing to recover from a close error
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return sum, fmt.Errorf("read %s: %w", path, err)
	}
	copy(sum[:], hasher.Sum(nil))
	return sum, nil
}
