package components

// workspaceColors are the palette keys of a workspace, in display order. The CSS of each key lives in
// web/tailwind.css (.ws-<key>); the workspaces domain validates the same list.
var workspaceColors = [...]string{"red", "orange", "amber", "green", "teal", "cyan", "blue", "violet", "pink", "gray"}

// WorkspaceColorKeys returns the palette keys in display order.
func WorkspaceColorKeys() []string { return append([]string(nil), workspaceColors[:]...) }

// WorkspaceColorClass is the CSS class that sets the color of a palette key; unknown or empty keys give
// no class, so the element keeps its default look.
func WorkspaceColorClass(key string) string {
	for _, known := range workspaceColors {
		if known == key {
			return "ws-" + key
		}
	}
	return ""
}

// ColorPickerProps configures ColorPicker.
type ColorPickerProps struct {
	Label     string
	NoneLabel string
	Name      string
	// Value is the selected palette key; empty selects "no color".
	Value string
}

// WorkspaceNameClass is the class list of the name pill: colored when the key is known, plain otherwise.
func WorkspaceNameClass(color string) string {
	if class := WorkspaceColorClass(color); class != "" {
		return "ws-chip " + class
	}
	return ""
}
