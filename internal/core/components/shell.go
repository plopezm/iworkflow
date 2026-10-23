package components

import "strconv"

// SidebarStorageKey is the localStorage key of the desktop sidebar state. Its value is "0"
// (hidden) or "1" (shown); the Rust version stored the same key.
const SidebarStorageKey = "iwf-sidebar-open"

// PanelSide is the edge of the layout a collapsible panel is anchored to.
type PanelSide int

// Panel edges.
const (
	LeftPanel PanelSide = iota
	RightPanel
)

// anchor pins the panel to its edge and draws the border on the inner side.
func (s PanelSide) anchor() string {
	if s == RightPanel {
		return "right-0 border-l"
	}
	return "left-0 border-r"
}

// HandleGlyph is the glyph of the collapse handle: it points toward the direction of the collapse.
func HandleGlyph(side PanelSide, open bool) string {
	if (side == LeftPanel) == open {
		return "‹"
	}
	return "›"
}

// ParseSidebarOpen reads the stored sidebar state. Only "0" means hidden; anything else, or no
// stored value, keeps it open.
func ParseSidebarOpen(stored string, found bool) bool {
	return !found || stored != "0"
}

// hiddenStyle hides an element with an inline display rule. Used instead of the hidden attribute
// on elements whose own classes set a display value (a class would override the attribute).
func hiddenStyle(hidden bool) string {
	if hidden {
		return "display: none"
	}
	return ""
}

func sidebarRowClass(selected bool) string {
	if selected {
		return "flex items-stretch rounded-md bg-surface-3"
	}
	return "flex items-stretch rounded-md hover:bg-surface-2"
}

func tabClass(selected bool) string {
	if selected {
		return "tab tab-selected"
	}
	return "tab"
}

func fmtCount(count int) string {
	return strconv.Itoa(count)
}
