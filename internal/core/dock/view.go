package dock

import (
	"net/url"
	"strconv"

	"github.com/a-h/templ"
)

// View is everything the dock fragment needs to render one layout. Launcher fills the "+" menu of
// every group; Empty is the body of a group without tabs.
type View struct {
	Layout   *Layout
	Data     string
	registry *Registry
	launcher templ.Component
	empty    templ.Component
}

// NewView builds the view of layout. Data is the serialized layout the client keeps in its storage.
func NewView(layout *Layout, registry *Registry, launcher, empty templ.Component) (View, error) {
	data, err := layout.ToJSON()
	if err != nil {
		return View{}, err
	}
	return View{Layout: layout, Data: data, registry: registry, launcher: launcher, empty: empty}, nil
}

// focusedPath is the path the address bar shows, or empty when the focused group has no tab.
func (v View) focusedPath() string {
	path, _ := v.Layout.FocusedPath()
	return path
}

// icon is the glyph of a tab path.
func (v View) icon(path string) string {
	return v.registry.Icon(path)
}

// paneSrc is the URL that renders the page of a tab.
func paneSrc(path string) string {
	return "/dock/pane?path=" + url.QueryEscape(path)
}

// id formats an identifier for a data attribute.
func id[T ~uint64](value T) string {
	return strconv.FormatUint(uint64(value), 10)
}

// ratioStyle is the flex basis of the first child of a split.
func ratioStyle(ratio float64) string {
	return "flex: 0 0 " + strconv.FormatFloat(ratio*100, 'f', -1, 64) + "%"
}

func splitContainerClass(dir SplitDir) string {
	if dir == Column {
		return "flex min-h-0 min-w-0 flex-1 flex-col"
	}
	return "flex min-h-0 min-w-0 flex-1 flex-row"
}

func dividerClass(dir SplitDir) string {
	if dir == Column {
		return "dock-divider dock-divider-col"
	}
	return "dock-divider dock-divider-row"
}

func orientation(dir SplitDir) string {
	if dir == Column {
		return "horizontal"
	}
	return "vertical"
}

func groupClass(focused bool) string {
	if focused {
		return "dock-group dock-group-focused"
	}
	return "dock-group"
}

func tabButtonClass(active bool) string {
	if active {
		return "group dock-tab dock-tab-active"
	}
	return "group dock-tab"
}

// activeTab is the active tab of a group, zero when none.
func activeTab(g *Group) TabID {
	return g.Active
}

// dropZone is one area of the drop overlay: its name and its position classes.
type dropZone struct {
	name     string
	position string
}

var dropZones = []dropZone{
	{name: "left", position: "left-0 top-0 bottom-0 w-1/4"},
	{name: "right", position: "right-0 top-0 bottom-0 w-1/4"},
	{name: "top", position: "left-1/4 right-1/4 top-0 h-1/4"},
	{name: "bottom", position: "left-1/4 right-1/4 bottom-0 h-1/4"},
	{name: "center", position: "left-1/4 right-1/4 top-1/4 bottom-1/4"},
}

func zoneClass(position string) string {
	return "absolute " + position + " dock-drop-zone"
}
