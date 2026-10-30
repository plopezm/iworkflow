package components

// ComposerFileItem is one suggestion of the @ completion. Insert is the text after the @ that the menu inserts: the
// path for the current project, or label:path for a related project. Label is the path the menu shows. Project is the
// label of the related project the file belongs to, and it is empty for the current project.
type ComposerFileItem struct {
	Insert  string
	Label   string
	Project string
}
