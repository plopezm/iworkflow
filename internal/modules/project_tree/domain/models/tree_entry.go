package models

// TreeEntryKind tells directories from files in a listing.
type TreeEntryKind uint8

const (
	TreeEntryDirectory TreeEntryKind = iota + 1
	TreeEntryFile
)

// TreeEntry is one child of a listed directory.
type TreeEntry struct {
	Name string
	// Path is relative to the project root, `/`-separated.
	Path string
	Kind TreeEntryKind
}

// DirectoryContents is one listed directory level.
type DirectoryContents struct {
	// Path is relative to the project root; empty means the root itself.
	Path    string
	Entries []TreeEntry
}

// ProjectDirectory is a directory level together with the name of the workspace it belongs to.
type ProjectDirectory struct {
	WorkspaceName string
	Contents      DirectoryContents
}

// ProjectFile is a text file of the project, addressed relative to the project root.
type ProjectFile struct {
	Name    string
	Path    string
	Content string
}
