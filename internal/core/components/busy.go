package components

import "github.com/a-h/templ"

// BusyClass appends the shared busy marker to base while busy is true; a busy control is always
// rendered with the spinner and inert styling.
func BusyClass(base string, busy bool) string {
	if busy {
		return base + " is-busy"
	}
	return base
}

// ariaBool renders a boolean the way ARIA attributes expect it.
func ariaBool(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

// BusyButtonProps configures a button that waits for the server or the disk.
type BusyButtonProps struct {
	Busy     bool
	Disabled bool
	// Class defaults to btn-secondary when empty.
	Class string
	// Type defaults to "button" when empty.
	Type string
	// Lang is set on the element when the label is in another language (e.g. a language choice).
	Lang string
	// Attrs carries the behaviour, usually htmx attributes (hx-post, hx-target...).
	Attrs templ.Attributes
}

func (p BusyButtonProps) class() string {
	if p.Class == "" {
		return "btn-secondary"
	}
	return p.Class
}

func (p BusyButtonProps) buttonType() string {
	if p.Type == "" {
		return "button"
	}
	return p.Type
}
