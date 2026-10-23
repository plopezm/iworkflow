package dto

import "iworkflow/internal/modules/workspaces/domain/models"

// DirectoryEntryDTO is a folder shown by the directory picker.
type DirectoryEntryDTO struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	IsGitRepo bool   `json:"is_git_repo"`
}

// DirectoryListingDTO is the content of a folder shown by the directory picker.
type DirectoryListingDTO struct {
	Path string `json:"path"`
	// Parent is null at the filesystem root.
	Parent  *string             `json:"parent"`
	Entries []DirectoryEntryDTO `json:"entries"`
}

// FromDirectoryListing maps a listing to its DTO.
func FromDirectoryListing(listing models.DirectoryListing) DirectoryListingDTO {
	entries := make([]DirectoryEntryDTO, 0, len(listing.Entries))
	for _, entry := range listing.Entries {
		entries = append(entries, DirectoryEntryDTO{Name: entry.Name, Path: entry.Path, IsGitRepo: entry.IsGitRepo})
	}
	var parent *string
	if listing.Parent != "" {
		value := listing.Parent
		parent = &value
	}
	return DirectoryListingDTO{Path: listing.Path, Parent: parent, Entries: entries}
}
