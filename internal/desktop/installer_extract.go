package desktop

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// extractArchive reads a gzip-compressed tar archive from r into dest, which must be an absolute folder.
//
// The archive must have one top folder, which is stripped: its contents land directly in dest. Entries that
// could leave dest are rejected before anything is written for them: absolute names, ".." components,
// backslashes and volume names, and symlinks or hard links whose target is outside dest. Only regular
// files, directories, symlinks and hard links to regular files are accepted. File modes are kept. The total
// size of the regular files is capped at maxBytes.
func extractArchive(r io.Reader, dest string, maxBytes int64) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("open gzip stream: %w", err)
	}
	defer func() { _ = gz.Close() }() // the reader holds no buffered writes, so its close error carries no information
	tr := tar.NewReader(gz)
	ex := &extractor{dest: dest, maxBytes: maxBytes}
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar entry: %w", err)
		}
		if err := ex.entry(tr, hdr); err != nil {
			return fmt.Errorf("extract entry %q: %w", hdr.Name, err)
		}
	}
	if ex.top == "" {
		return errors.New("archive has no entries")
	}
	return nil
}

// extractor keeps the state of one extraction.
type extractor struct {
	dest     string
	maxBytes int64
	top      string
	written  int64
}

func (e *extractor) entry(tr *tar.Reader, hdr *tar.Header) error {
	rel, err := e.relative(hdr.Name)
	if err != nil {
		return err
	}
	if len(rel) == 0 {
		// The top folder itself carries nothing to extract.
		return nil
	}
	target := filepath.Join(append([]string{e.dest}, rel...)...)
	if !isWithin(e.dest, target) {
		return errors.New("entry leaves the staging folder")
	}
	switch hdr.Typeflag {
	case tar.TypeDir:
		return e.makeDir(target, hdr)
	case tar.TypeReg:
		return e.writeFile(tr, target, hdr)
	case tar.TypeSymlink:
		return e.symlink(target, hdr.Linkname)
	case tar.TypeLink:
		return e.hardLink(target, hdr.Linkname)
	default:
		return fmt.Errorf("unsupported entry type %q", hdr.Typeflag)
	}
}

// relative validates an archive name and returns its components below the top folder. The first name fixes the
// top folder; every other entry must use the same one.
func (e *extractor) relative(name string) ([]string, error) {
	parts, err := splitArchivePath(name)
	if err != nil {
		return nil, err
	}
	if len(parts) == 0 {
		return nil, errors.New("entry has no name")
	}
	if e.top == "" {
		e.top = parts[0]
	}
	if parts[0] != e.top {
		return nil, fmt.Errorf("entry is outside the single top folder %q", e.top)
	}
	return parts[1:], nil
}

func (e *extractor) makeDir(target string, hdr *tar.Header) error {
	if err := os.MkdirAll(target, 0o755); err != nil {
		return fmt.Errorf("create folder: %w", err)
	}
	// Owner access is always granted so the folder can be filled and removed later.
	if err := os.Chmod(target, hdr.FileInfo().Mode().Perm()|0o700); err != nil {
		return fmt.Errorf("set folder mode: %w", err)
	}
	return nil
}

func (e *extractor) writeFile(tr *tar.Reader, target string, hdr *tar.Header) error {
	if hdr.Size < 0 {
		return errors.New("negative file size")
	}
	if hdr.Size > e.maxBytes-e.written {
		return fmt.Errorf("archive extracts more than %d bytes", e.maxBytes)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create parent folder: %w", err)
	}
	// O_EXCL refuses to follow a symlink or overwrite an entry created earlier in the archive.
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	written, copyErr := io.Copy(file, io.LimitReader(tr, hdr.Size))
	closeErr := file.Close()
	e.written += written
	switch {
	case copyErr != nil:
		return fmt.Errorf("write file: %w", copyErr)
	case written != hdr.Size:
		return fmt.Errorf("file is truncated: %d of %d bytes", written, hdr.Size)
	case closeErr != nil:
		return fmt.Errorf("close file: %w", closeErr)
	}
	if err := os.Chmod(target, hdr.FileInfo().Mode().Perm()); err != nil {
		return fmt.Errorf("set the mode of %s: %w", target, err)
	}
	return nil
}

// symlink creates a symlink whose target, resolved from its own folder, stays inside dest.
func (e *extractor) symlink(target string, linkname string) error {
	if linkname == "" {
		return errors.New("symlink has no target")
	}
	if strings.HasPrefix(linkname, "/") || filepath.IsAbs(linkname) || filepath.VolumeName(linkname) != "" || strings.ContainsRune(linkname, '\\') {
		return fmt.Errorf("symlink target %q is absolute or not portable", linkname)
	}
	resolved := filepath.Join(filepath.Dir(target), filepath.FromSlash(linkname))
	if !isWithin(e.dest, resolved) {
		return fmt.Errorf("symlink target %q leaves the staging folder", linkname)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create parent folder: %w", err)
	}
	if err := os.Symlink(filepath.FromSlash(linkname), target); err != nil {
		return fmt.Errorf("create symlink: %w", err)
	}
	return nil
}

// hardLink creates a hard link to a regular file that an earlier entry of this archive created inside dest.
func (e *extractor) hardLink(target string, linkname string) error {
	parts, err := splitArchivePath(linkname)
	if err != nil {
		return fmt.Errorf("hard link source: %w", err)
	}
	if len(parts) < 2 || parts[0] != e.top {
		return fmt.Errorf("hard link source %q is outside the top folder", linkname)
	}
	source := filepath.Join(append([]string{e.dest}, parts[1:]...)...)
	if !isWithin(e.dest, source) {
		return fmt.Errorf("hard link source %q leaves the staging folder", linkname)
	}
	info, err := os.Lstat(source)
	if err != nil {
		return fmt.Errorf("hard link source: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("hard link source %q is not a regular file", linkname)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create parent folder: %w", err)
	}
	if err := os.Link(source, target); err != nil {
		return fmt.Errorf("create hard link: %w", err)
	}
	return nil
}

// splitArchivePath validates an archive name and returns its non-empty components. Absolute names, volume
// names, backslashes and ".." components are rejected.
func splitArchivePath(name string) ([]string, error) {
	if name == "" {
		return nil, errors.New("empty entry name")
	}
	if strings.ContainsRune(name, '\\') {
		return nil, fmt.Errorf("entry name %q contains a backslash", name)
	}
	if strings.HasPrefix(name, "/") || filepath.IsAbs(name) || filepath.VolumeName(name) != "" {
		return nil, fmt.Errorf("entry name %q is absolute", name)
	}
	var parts []string
	for _, part := range strings.Split(name, "/") {
		switch part {
		case "", ".":
			continue
		case "..":
			return nil, fmt.Errorf("entry name %q has a parent reference", name)
		}
		parts = append(parts, part)
	}
	return parts, nil
}

// isWithin reports whether p is root or lies below it, comparing cleaned paths without touching the file system.
func isWithin(root string, p string) bool {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
