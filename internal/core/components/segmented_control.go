package components

import "github.com/a-h/templ"

// SegmentOption is one segment: a stable value and its already translated label.
type SegmentOption struct {
	Value string
	Label string
	Attrs templ.Attributes
}

// SegmentedControlProps configures a row of mutually exclusive segments.
type SegmentedControlProps struct {
	Options   []SegmentOption
	Selected  string
	AriaLabel string
	Class     string
}

// segmentClass is the class of a segment, marking the selected one.
func segmentClass(selected bool) string {
	if selected {
		return "segment segment-selected"
	}
	return "segment"
}
