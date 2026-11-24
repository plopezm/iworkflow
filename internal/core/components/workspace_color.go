package components

import (
	"github.com/a-h/templ"

	"iworkflow/internal/core/i18n"
)

// customColorClass marks a workspace whose color is a free "#rrggbb" value instead of a palette key; the value
// itself reaches CSS through WorkspaceColorStyle.
const customColorClass string = "ws-custom"

var msgCustomColor = i18n.Msg{En: "Custom color", Es: "Color personalizado", Zh: "自定义颜色", De: "Eigene Farbe", Fr: "Couleur personnalisée"}

// IsHexColor reports whether the text is a lowercase "#rrggbb" color, the value a browser color input submits.
func IsHexColor(value string) bool {
	if len(value) != 7 || value[0] != '#' {
		return false
	}
	for _, c := range value[1:] {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// WorkspaceColorStyle is the inline style that sets --ws for a custom color; palette keys and unknown values give
// none, because their class sets it. The value is validated, so it is safe CSS.
func WorkspaceColorStyle(color string) templ.SafeCSS {
	if IsHexColor(color) {
		return templ.SafeCSS("--ws:" + color + ";")
	}
	return ""
}

// workspaceColors are the palette keys of a workspace, in display order. The CSS of each key lives in
// web/tailwind.css (.ws-<key>); the workspaces domain validates the same list.
var workspaceColors = [...]string{"red", "orange", "amber", "green", "teal", "cyan", "blue", "violet", "pink", "gray"}

// WorkspaceColorKeys returns the palette keys in display order.
func WorkspaceColorKeys() []string { return append([]string(nil), workspaceColors[:]...) }

// WorkspaceColorClass is the CSS class that sets the color of a palette key; unknown or empty keys give
// no class, so the element keeps its default look.
func WorkspaceColorClass(key string) string {
	if IsHexColor(key) {
		return customColorClass
	}
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

// WorkspaceNameClass is the class list of the name: carries the color (for its dot) when the key is known, plain otherwise.
func WorkspaceNameClass(color string) string {
	if class := WorkspaceColorClass(color); class != "" {
		return "ws-name " + class
	}
	return ""
}

// defaultCustomColor is what the browser picker opens on when the workspace has no custom color yet.
const defaultCustomColor string = "#8e4ec6"

// customValue is the submitted value of the custom swatch: the chosen color, or empty until one is picked.
func customValue(color string) string {
	if IsHexColor(color) {
		return color
	}
	return ""
}

// customPickerValue is the color the native picker starts on.
func customPickerValue(color string) string {
	if IsHexColor(color) {
		return color
	}
	return defaultCustomColor
}
