package models

// ProfileCatalog is the gentle-pi profile catalog (`profiles.json`).
type ProfileCatalog struct {
	// Active is the profile currently selected in the file, nil when none is.
	Active *string
	// Names are the profile names, sorted.
	Names []string
}
