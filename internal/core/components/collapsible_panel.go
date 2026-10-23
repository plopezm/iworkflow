package components

// CollapsiblePanelProps configures a side panel with a header (title and counter) and a body that
// can be folded away. Its texts are supplied by the caller.
type CollapsiblePanelProps struct {
	Title         string
	Count         int
	Collapsed     bool
	ExpandLabel   string
	CollapseLabel string
}

// collapsiblePanelClass is the class of the panel root for its collapsed state.
func collapsiblePanelClass(collapsed bool) string {
	if collapsed {
		return "side-panel side-panel-collapsed"
	}
	return "side-panel"
}

// toggleLabel is the accessible name of the toggle: the action it will perform.
func toggleLabel(collapsed bool, expand, collapse string) string {
	if collapsed {
		return expand
	}
	return collapse
}
