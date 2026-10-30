package components

import "github.com/a-h/templ"

// SelectOption is one entry of an OptionSelect, optionally grouped under an <optgroup>.
type SelectOption struct {
	Value string
	Label string
	Group string
}

// OptionSelectGroup is a run of options that share a group name ("" is ungrouped).
type OptionSelectGroup struct {
	Name    string
	Options []SelectOption
}

// GroupOptions groups the options by Group, keeping the order of the first appearance of each
// group and of the options inside it.
func GroupOptions(options []SelectOption) []OptionSelectGroup {
	groups := []OptionSelectGroup{}
	for _, option := range options {
		index := -1
		for i, group := range groups {
			if group.Name == option.Group {
				index = i
				break
			}
		}
		if index < 0 {
			groups = append(groups, OptionSelectGroup{Name: option.Group})
			index = len(groups) - 1
		}
		groups[index].Options = append(groups[index].Options, option)
	}
	return groups
}

// OptionSelectProps configures a compact pill drop-down. The label is not shown (it is the
// accessible name); DefaultLabel is the empty first option.
type OptionSelectProps struct {
	Label        string
	Value        string
	DefaultLabel string
	Options      []SelectOption
	Disabled     bool
	// Icon is an optional visual prefix (a small SVG), rendered before the select.
	Icon string
	// Class adds classes to the outer label.
	Class string
	// Attrs carries the behaviour, usually htmx attributes (hx-post, hx-trigger="change").
	Attrs templ.Attributes
}

// optionSelectClass returns "select-pill" with extra classes when specified, avoiding trailing whitespace.
func optionSelectClass(class string) string {
	if class != "" {
		return "select-pill " + class
	}
	return "select-pill"
}

// currentLabel is the text of the selected option, or DefaultLabel when none matches.
func currentLabel(p OptionSelectProps) string {
	for _, option := range p.Options {
		if option.Value == p.Value {
			return option.Label
		}
	}
	return p.DefaultLabel
}
