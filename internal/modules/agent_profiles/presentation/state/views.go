package state

// ListItem is one profile chip of the profiles list.
type ListItem struct {
	Name     string
	Active   bool
	Selected bool
}

// ListView is the props of the profile selector: the card with the chips of the profiles, or an
// error when the store cannot be read. Other screens reuse it by filling the same props.
type ListView struct {
	// ID is the id of the card: the swap target of a chip selection.
	ID string
	// DialogID is the dialog host that the new and import buttons open into.
	DialogID string
	Items    []ListItem
	Error    string
	// SelectURL answers a chip selection with the card (the swap target) and the detail (out of
	// band). The chip sends its name as the "selected" parameter.
	SelectURL string
	// NewURL and ImportURL open the name and import dialogs.
	NewURL    string
	ImportURL string
}

// RoleItem is one agent role of the roles card.
type RoleItem struct {
	Name        string
	Description string
}

// RolesView is the agent roles card.
type RolesView struct {
	Items []RoleItem
	Error string
	// NewRoleURL opens the role dialog for the screen that owns the card.
	NewRoleURL string
}

// EditorView is the routing editor of a profile or of the global routing. Scope prefixes its ids
// (profiles or models), so two screens can be open at once.
type EditorView struct {
	// FormID is the id of the form that wraps the rows; it is also the swap target of the editor.
	FormID string
	// SaveURL persists the rows; RowsURL applies one row action and re-renders the editor.
	SaveURL string
	RowsURL string
	// AddSelectID is the id of the "agent to add" select, which role creation refreshes out of band.
	AddSelectID string
	Rows        []RoutingRow
	Models      []Option
	Levels      []string
	Addable     []string
	Error       string
}

// DetailView is the right-hand card of the profiles screen.
type DetailView struct {
	// Present is false when there is no profile yet: the empty state is shown instead.
	Present bool
	Name    string
	Active  bool
	// ConfirmDelete shows the confirmation panel above the routing editor.
	ConfirmDelete bool
	// Error is the first failure of the store or of the catalog, shown above the editor.
	Error   string
	Routing EditorView
	// Links of the actions of the profile.
	DetailURL       string
	DuplicateURL    string
	RenameURL       string
	DeleteURL       string
	ExportURL       string
	ExportNoticeURL string
	JSONURL         string
	// Filename is the download name of the exported profile.
	Filename string
	// Selected is the profile the actions act on, sent back so the server can move the selection.
	Selected string
}

// ProfilesView is the whole profiles screen.
type ProfilesView struct {
	List   ListView
	Detail DetailView
	Roles  RolesView
}

// ModelsView is the global routing screen.
type ModelsView struct {
	Editor     EditorView
	NewRoleURL string
}

// NameDialogView is the dialog that asks for a profile name (new, duplicate or rename).
type NameDialogView struct {
	// ID is the dialog host, the swap target of the form.
	ID          string
	Title       string
	SubmitLabel string
	Action      string
	HintURL     string
	Value       string
	Selected    string
	// Hint explains why Value is not valid. It is shown only once the user has typed something, while
	// the submit button is disabled from the start, as in the Rust dialog.
	Hint    string
	Invalid bool
	Error   string
}

// ImportDialogView is the dialog that imports a profile from a file or from pasted JSON.
type ImportDialogView struct {
	ID       string
	Action   string
	Raw      string
	RenameTo string
	Error    string
}

// RoleDialogView is the dialog that creates an agent role. Scope names the screen that opened it.
type RoleDialogView struct {
	ID          string
	Scope       string
	SubmitLabel string
	Action      string
	HintURL     string
	Selected    string
	Name        string
	Description string
	Prompt      string
	Hint        string
	Invalid     bool
	Error       string
}
