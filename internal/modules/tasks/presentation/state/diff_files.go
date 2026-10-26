package state

import (
	"sort"
	"strconv"
	"strings"
)

// Pure model of a unified patch: per-file diffs with split/unified rows and a tree of the changed files. Port of
// src/modules/tasks/presentation/state/diff_files.rs; it has no framework dependencies.

const (
	diffHeader = "diff --git "
	devNull    = "/dev/null"
)

// FileStatus is how a file changed.
type FileStatus string

// The statuses of a changed file.
const (
	FileStatusAdded    FileStatus = "added"
	FileStatusModified FileStatus = "modified"
	FileStatusDeleted  FileStatus = "deleted"
	FileStatusRenamed  FileStatus = "renamed"
)

// CellKind is the role of a line in the diff.
type CellKind string

// The roles of a line.
const (
	CellContext CellKind = "context"
	CellAdded   CellKind = "added"
	CellRemoved CellKind = "removed"
)

// Cell is one side of a split row. Number is the line number on its side.
type Cell struct {
	Number int
	Text   string
	Kind   CellKind
}

// SplitRow is a row of the split view. A nil side is the blank half of an addition or a removal.
type SplitRow struct {
	Left  *Cell
	Right *Cell
}

// UnifiedLine is a line of the unified view. A zero line number means the line does not exist on that side.
type UnifiedLine struct {
	Kind      CellKind
	OldNumber int
	NewNumber int
	Text      string
}

// Hunk is one `@@` section of a file diff.
type Hunk struct {
	Header string
	Rows   []SplitRow
	Lines  []UnifiedLine
}

// FileDiff is the diff of one file. OldPath is only set for renames.
type FileDiff struct {
	Path      string
	OldPath   string
	Status    FileStatus
	Additions int
	Deletions int
	Binary    bool
	Hunks     []Hunk
}

// TreeNode is a folder or a file of the changed-files tree. FileIndex points into the files the tree was built from
// and is only meaningful for a file (Dir is false).
type TreeNode struct {
	Name      string
	Path      string
	Dir       bool
	Children  []TreeNode
	FileIndex int
	Additions int
	Deletions int
}

// StatusLetter is the one-letter badge of a status (A, M, D, R).
func StatusLetter(status FileStatus) string {
	switch status {
	case FileStatusAdded:
		return "A"
	case FileStatusModified:
		return "M"
	case FileStatusDeleted:
		return "D"
	case FileStatusRenamed:
		return "R"
	}
	return ""
}

// hunkBuilder collects the lines of one hunk while they are read, pairing removals with additions into split rows.
type hunkBuilder struct {
	header  string
	oldNext int
	newNext int
	removed []Cell
	added   []Cell
	rows    []SplitRow
	lines   []UnifiedLine
}

// flush turns the pending removals and additions into rows, pairing them in order.
func (h *hunkBuilder) flush() {
	count := max(len(h.removed), len(h.added))
	for index := range count {
		var row SplitRow
		if index < len(h.removed) {
			cell := h.removed[index]
			row.Left = &cell
		}
		if index < len(h.added) {
			cell := h.added[index]
			row.Right = &cell
		}
		h.rows = append(h.rows, row)
	}
	h.removed = nil
	h.added = nil
}

// pushLine consumes one line of the hunk. prefix is its first character: '-', '+' or ' ' (any other is context).
func (h *hunkBuilder) pushLine(prefix byte, text string) {
	switch prefix {
	case '-':
		if len(h.added) > 0 {
			h.flush()
		}
		number := h.oldNext
		h.oldNext++
		h.lines = append(h.lines, UnifiedLine{Kind: CellRemoved, OldNumber: number, Text: text})
		h.removed = append(h.removed, Cell{Number: number, Text: text, Kind: CellRemoved})
	case '+':
		number := h.newNext
		h.newNext++
		h.lines = append(h.lines, UnifiedLine{Kind: CellAdded, NewNumber: number, Text: text})
		h.added = append(h.added, Cell{Number: number, Text: text, Kind: CellAdded})
	default:
		h.flush()
		oldNumber := h.oldNext
		newNumber := h.newNext
		h.oldNext++
		h.newNext++
		h.lines = append(h.lines, UnifiedLine{Kind: CellContext, OldNumber: oldNumber, NewNumber: newNumber, Text: text})
		h.rows = append(h.rows, SplitRow{
			Left:  &Cell{Number: oldNumber, Text: text, Kind: CellContext},
			Right: &Cell{Number: newNumber, Text: text, Kind: CellContext},
		})
	}
}

func (h *hunkBuilder) finish() Hunk {
	h.flush()
	return Hunk{Header: h.header, Rows: h.rows, Lines: h.lines}
}

// fileBuilder collects one file section while its lines are read.
type fileBuilder struct {
	path    string
	oldPath string
	status  FileStatus
	binary  bool
	hunks   []Hunk
	hunk    *hunkBuilder
	sawOld  *string
	sawNew  *string
}

// newFileBuilder reads the names of a `diff --git a/<old> b/<new>` header.
func newFileBuilder(line string) *fileBuilder {
	rest := strings.TrimPrefix(line, diffHeader)
	oldName := rest
	newName := rest
	if index := strings.Index(rest, " b/"); index >= 0 {
		oldName = trimRepeatedPrefix(rest[:index], "a/")
		newName = rest[index+3:]
	}
	return &fileBuilder{path: newName, oldPath: oldName, status: FileStatusModified}
}

func (f *fileBuilder) closeHunk() {
	if f.hunk == nil {
		return
	}
	f.hunks = append(f.hunks, f.hunk.finish())
	f.hunk = nil
}

// consume reads one line of the file section.
func (f *fileBuilder) consume(line string) {
	if f.hunk != nil {
		f.consumeHunkLine(line)
		return
	}
	switch {
	case strings.HasPrefix(line, "@@"):
		f.startHunk(line)
	case strings.HasPrefix(line, "new file mode"):
		f.status = FileStatusAdded
	case strings.HasPrefix(line, "deleted file mode"):
		f.status = FileStatusDeleted
	case strings.HasPrefix(line, "rename from "):
		f.status = FileStatusRenamed
		f.oldPath = strings.TrimPrefix(line, "rename from ")
	case strings.HasPrefix(line, "rename to "):
		f.status = FileStatusRenamed
		f.path = strings.TrimPrefix(line, "rename to ")
	case strings.HasPrefix(line, "Binary files"), strings.HasPrefix(line, "GIT binary patch"):
		f.binary = true
	case strings.HasPrefix(line, "--- "):
		f.sawOld = stripSide(strings.TrimPrefix(line, "--- "), "a/")
	case strings.HasPrefix(line, "+++ "):
		f.sawNew = stripSide(strings.TrimPrefix(line, "+++ "), "b/")
	}
}

// consumeHunkLine reads a line while a hunk is open. A `\` line (no newline marker) is ignored; a line that is not a
// change is context, and a new `@@` header closes the hunk.
func (f *fileBuilder) consumeHunkLine(line string) {
	if line == "" {
		f.hunk.pushLine(' ', "")
		return
	}
	switch prefix := line[0]; prefix {
	case '+', '-', ' ':
		f.hunk.pushLine(prefix, line[1:])
	case '\\':
	default:
		if strings.HasPrefix(line, "@@") {
			f.closeHunk()
			f.startHunk(line)
			return
		}
		f.hunk.pushLine(' ', line)
	}
}

func (f *fileBuilder) startHunk(line string) {
	oldStart, newStart, ok := parseHunkStarts(line)
	if !ok {
		oldStart, newStart = 1, 1
	}
	f.hunk = &hunkBuilder{header: line, oldNext: oldStart, newNext: newStart}
}

func (f *fileBuilder) finish() FileDiff {
	f.closeHunk()
	switch {
	case f.status == FileStatusDeleted:
		if f.sawOld != nil {
			f.path = *f.sawOld
		} else {
			f.path = f.oldPath
		}
	case f.sawNew != nil:
		f.path = *f.sawNew
	}
	additions := 0
	deletions := 0
	for _, hunk := range f.hunks {
		additions += countKind(hunk, CellAdded)
		deletions += countKind(hunk, CellRemoved)
	}
	oldPath := ""
	if f.status == FileStatusRenamed {
		oldPath = f.oldPath
	}
	return FileDiff{
		Path:      f.path,
		OldPath:   oldPath,
		Status:    f.status,
		Additions: additions,
		Deletions: deletions,
		Binary:    f.binary,
		Hunks:     f.hunks,
	}
}

func countKind(hunk Hunk, kind CellKind) int {
	count := 0
	for _, line := range hunk.Lines {
		if line.Kind == kind {
			count++
		}
	}
	return count
}

// trimRepeatedPrefix removes every leading occurrence of prefix (Rust trim_start_matches).
func trimRepeatedPrefix(value string, prefix string) string {
	for strings.HasPrefix(value, prefix) {
		value = strings.TrimPrefix(value, prefix)
	}
	return value
}

// stripSide returns the path of a `---`/`+++` line without its side prefix, or nil for /dev/null.
func stripSide(value string, prefix string) *string {
	trimmed := strings.TrimRight(value, "\t")
	if trimmed == devNull {
		return nil
	}
	path := strings.TrimPrefix(trimmed, prefix)
	return &path
}

// parseHunkStarts extracts the old and new start lines from `@@ -a,b +c,d @@`.
func parseHunkStarts(header string) (int, int, bool) {
	rest, ok := strings.CutPrefix(header, "@@ -")
	if !ok {
		return 0, 0, false
	}
	oldPart, rest, ok := strings.Cut(rest, " +")
	if !ok {
		return 0, 0, false
	}
	newPart, _, _ := strings.Cut(rest, " ")
	oldStart, oldOK := parseLineStart(oldPart)
	newStart, newOK := parseLineStart(newPart)
	if !oldOK || !newOK {
		return 0, 0, false
	}
	return oldStart, newStart, true
}

// parseLineStart reads the start line of a `start[,count]` range.
func parseLineStart(rng string) (int, bool) {
	start, _, _ := strings.Cut(rng, ",")
	value, err := strconv.Atoi(start)
	if err != nil || value < 0 {
		return 0, false
	}
	return value, true
}

// ParsePatch parses a git unified patch into one FileDiff per file. It tolerates a patch truncated at any point,
// including mid-line.
func ParsePatch(patch string) []FileDiff {
	var files []FileDiff
	var current *fileBuilder
	body := strings.TrimSuffix(patch, "\n")
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.HasPrefix(line, diffHeader) {
			if current != nil {
				files = append(files, current.finish())
			}
			current = newFileBuilder(line)
		} else if current != nil {
			current.consume(line)
		}
	}
	if current != nil {
		files = append(files, current.finish())
	}
	return files
}

// dirBuilder is a folder of the tree under construction.
type dirBuilder struct {
	dirs  map[string]*dirBuilder
	files map[string]int
}

func newDirBuilder() *dirBuilder {
	return &dirBuilder{dirs: map[string]*dirBuilder{}, files: map[string]int{}}
}

// BuildTree builds the folder tree of the changed files: folders first, then files, alphabetical. Folders aggregate
// the counters of their content, and a chain of single-child folders is compacted into one row.
func BuildTree(files []FileDiff) []TreeNode {
	root := newDirBuilder()
	for index, file := range files {
		parts := strings.Split(file.Path, "/")
		name := parts[len(parts)-1]
		node := root
		for _, dir := range parts[:len(parts)-1] {
			if node.dirs[dir] == nil {
				node.dirs[dir] = newDirBuilder()
			}
			node = node.dirs[dir]
		}
		node.files[name] = index
	}
	return materialize(root, "", files)
}

func materialize(dir *dirBuilder, prefix string, files []FileDiff) []TreeNode {
	var nodes []TreeNode
	for _, name := range sortedKeys(dir.dirs) {
		label := name
		path := prefix + name
		current := dir.dirs[name]
		for len(current.files) == 0 && len(current.dirs) == 1 {
			nextName, next := singleChild(current.dirs)
			label += "/" + nextName
			path += "/" + nextName
			current = next
		}
		children := materialize(current, path+"/", files)
		additions := 0
		deletions := 0
		for _, child := range children {
			additions += child.Additions
			deletions += child.Deletions
		}
		nodes = append(nodes, TreeNode{Name: label, Path: path, Dir: true, Children: children, Additions: additions, Deletions: deletions})
	}
	for _, name := range sortedKeys(dir.files) {
		index := dir.files[name]
		file := files[index]
		nodes = append(nodes, TreeNode{Name: name, Path: file.Path, FileIndex: index, Additions: file.Additions, Deletions: file.Deletions})
	}
	return nodes
}

// sortedKeys returns the keys of a map in byte order, so the tree is stable.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// singleChild returns the only entry of a folder map. The caller checks that the map holds exactly one entry.
func singleChild(m map[string]*dirBuilder) (string, *dirBuilder) {
	for key, child := range m {
		return key, child
	}
	return "", nil
}
