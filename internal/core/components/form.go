package components

import (
	"strconv"
	"strings"

	"github.com/a-h/templ"
)

// FormOption is one choice of a select field: stable value and visible label.
type FormOption struct {
	Value string
	Label string
}

// ParsePositiveNumber reads an optional positive integer field: it returns false when the text is
// empty, zero, negative or not a number.
func ParsePositiveNumber(value string) (uint32, bool) {
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 32)
	if err != nil || parsed == 0 {
		return 0, false
	}
	return uint32(parsed), true
}

// FieldProps configures a text input or a text area.
type FieldProps struct {
	Label       string
	Name        string
	Value       string
	Placeholder string
	Required    bool
	// Large makes a text area taller and its text bigger.
	Large bool
	// Attrs carries the behaviour, usually htmx attributes.
	Attrs templ.Attributes
}

// SelectFieldProps configures a select. Placeholder, when set, adds an empty first option.
type SelectFieldProps struct {
	Label       string
	Name        string
	Value       string
	Placeholder string
	Required    bool
	Options     []FormOption
	Attrs       templ.Attributes
}

// NumberFieldProps configures an optional numeric field with its unit (empty means "no value").
type NumberFieldProps struct {
	Label       string
	Name        string
	Value       string
	Unit        string
	Placeholder string
	Attrs       templ.Attributes
}

// NumberInputProps configures a compact numeric pill whose value is set from the outside.
type NumberInputProps struct {
	Label       string
	Name        string
	Value       string
	Unit        string
	Placeholder string
	Disabled    bool
	Attrs       templ.Attributes
}

func textAreaClass(large bool) string {
	if large {
		return "input input-lg min-h-[50vh] resize-y"
	}
	return "input min-h-24"
}
