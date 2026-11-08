package models

// DirEntry is one file or folder of a Listing.
type DirEntry struct {
	Name  string
	Path  string
	IsDir bool
}

// Listing is the content of a folder, as the browser of the editor shows it: folders first, then files. Parent is
// empty at the root.
type Listing struct {
	Path    string
	Parent  string
	Entries []DirEntry
}
