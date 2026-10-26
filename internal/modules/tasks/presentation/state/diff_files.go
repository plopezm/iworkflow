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

// TreeNode is a folder or a file of the changed-files tree. Status and FileIndex (the position in the files the tree
// was built from) are only meaningful for a file (Dir is false).
type TreeNode struct {
	Name      string
	Path      string
	Dir       bool
	Children  []TreeNode
	Status    FileStatus
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

// fileBuilder collects one file section while its lines are read. oldSide and newSide are the paths that the `---` and
// `+++` lines name; an empty one is a side with no path (/dev/null, or a line that was never read), and the name of the
// header stands in for it.
type fileBuilder struct {
	path    string
	oldPath string
	status  FileStatus
	binary  bool
	hunks   []Hunk
	hunk    *hunkBuilder
	oldSide string
	newSide string
}

// newFileBuilder reads the names of a `diff --git a/<old> b/<new>` header.
func newFileBuilder(line string) *fileBuilder {
	oldName, newName := splitHeaderNames(strings.TrimPrefix(line, diffHeader))
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
		f.oldPath = unquotePath(strings.TrimPrefix(line, "rename from "))
	case strings.HasPrefix(line, "rename to "):
		f.status = FileStatusRenamed
		f.path = unquotePath(strings.TrimPrefix(line, "rename to "))
	case strings.HasPrefix(line, "Binary files"), strings.HasPrefix(line, "GIT binary patch"):
		f.binary = true
	case strings.HasPrefix(line, "--- "):
		f.oldSide = stripSide(strings.TrimPrefix(line, "--- "), "a/")
	case strings.HasPrefix(line, "+++ "):
		f.newSide = stripSide(strings.TrimPrefix(line, "+++ "), "b/")
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
		if f.oldSide != "" {
			f.path = f.oldSide
		} else {
			f.path = f.oldPath
		}
	case f.newSide != "":
		f.path = f.newSide
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

// stripSide returns the path of a `---`/`+++` line without its side prefix, or "" for /dev/null.
func stripSide(value string, prefix string) string {
	trimmed := strings.TrimRight(value, "\t")
	if trimmed == devNull {
		return ""
	}
	return strings.TrimPrefix(unquotePath(trimmed), prefix)
}

// splitHeaderNames reads the old and the new name of the rest of a `diff --git` line (without the prefix). Git quotes a
// name that holds special bytes, and it can quote only one of the two, so a quoted name is read up to its closing quote
// and an unquoted one up to the separator. A name that cannot be split is returned as it is, on both sides.
func splitHeaderNames(rest string) (string, string) {
	oldToken, newToken, ok := cutHeaderNames(rest)
	if !ok {
		path := unquotePath(rest)
		return path, path
	}
	oldName := trimRepeatedPrefix(unquotePath(oldToken), "a/")
	newName := strings.TrimPrefix(unquotePath(newToken), "b/")
	return oldName, newName
}

// cutHeaderNames separates the old token from the new token of a header. A quoted new side starts with ` "b/`, which a
// plain name can never contain; otherwise the first ` b/` separates them, as before quoting was handled.
func cutHeaderNames(rest string) (string, string, bool) {
	if strings.HasPrefix(rest, `"`) {
		end := closingQuote(rest)
		if end > 0 && end+1 < len(rest) && rest[end+1] == ' ' {
			return rest[:end+1], rest[end+2:], true
		}
		return "", "", false
	}
	if index := strings.Index(rest, ` "b/`); index >= 0 {
		return rest[:index], rest[index+1:], true
	}
	if index := strings.Index(rest, " b/"); index >= 0 {
		return rest[:index], rest[index+1:], true
	}
	return "", "", false
}

// closingQuote returns the index of the quote that closes the quoted name at the start of value, or -1. An escaped quote
// does not close it.
func closingQuote(value string) int {
	for index := 1; index < len(value); index++ {
		switch value[index] {
		case '\\':
			index++
		case '"':
			return index
		}
	}
	return -1
}

// unquotePath undoes git's C-style quoting of a path (`"a/caf\303\251"`): the quotes come off and the escapes, octal
// bytes included, are decoded. A value that is not quoted is returned as it is.
func unquotePath(value string) string {
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
		return value
	}
	body := value[1 : len(value)-1]
	var out strings.Builder
	for index := 0; index < len(body); index++ {
		if body[index] != '\\' || index+1 >= len(body) {
			out.WriteByte(body[index])
			continue
		}
		index++
		if octal, ok := octalByte(body, index); ok {
			out.WriteByte(octal)
			index += 2
			continue
		}
		out.WriteByte(unescapeChar(body[index]))
	}
	return out.String()
}

// unescapeChar is the byte a one-character C escape stands for. An unknown escape keeps its character.
func unescapeChar(char byte) byte {
	switch char {
	case 'a':
		return '\a'
	case 'b':
		return '\b'
	case 't':
		return '\t'
	case 'n':
		return '\n'
	case 'v':
		return '\v'
	case 'f':
		return '\f'
	case 'r':
		return '\r'
	}
	return char
}

// octalByte reads the three octal digits of a `\NNN` escape that starts at index, the byte they name.
func octalByte(body string, index int) (byte, bool) {
	if index+2 >= len(body) || !isOctalDigit(body[index]) || !isOctalDigit(body[index+1]) || !isOctalDigit(body[index+2]) {
		return 0, false
	}
	value, err := strconv.ParseUint(body[index:index+3], 8, 8)
	if err != nil {
		return 0, false
	}
	return byte(value), true
}

func isOctalDigit(char byte) bool {
	return char >= '0' && char <= '7'
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
		nodes = append(nodes, TreeNode{Name: name, Path: file.Path, Status: file.Status, FileIndex: index, Additions: file.Additions, Deletions: file.Deletions})
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
