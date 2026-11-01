package models

// FileEntry is a file or a folder inside a browsed folder.
type FileEntry struct {
	Name  string
	Path  string
	IsDir bool
}

// Listing is the content of a folder: the folders first, then the files, each group sorted by name.
type Listing struct {
	Path    string
	Parent  string
	Entries []FileEntry
	// Truncated reports that more entries matched than the listing holds.
	Truncated bool
}
