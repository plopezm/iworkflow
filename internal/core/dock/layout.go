// Package dock is the tabbed workspace shell: a tree of split groups, each holding tabs whose
// content is a server-rendered page. This file is the pure layout model (no HTTP, no templ),
// so it can be tested and serialized. Its JSON matches the Rust serde format so a layout saved
// in the browser by the Rust app is still read by the Go one.
package dock

import (
	"encoding/json"
	"fmt"
)

// TabID, GroupID and SplitID identify layout elements. Zero means "none": ids start at 1.
type (
	TabID   uint64
	GroupID uint64
	SplitID uint64
)

// Ratio bounds of the first child of a split.
const (
	minRatio = 0.12
	maxRatio = 0.88
)

// SplitDir places the children of a split side by side (Row) or one above the other (Column).
type SplitDir string

// Split directions. The values are the serialized form used in the saved layout.
const (
	Row    SplitDir = "Row"
	Column SplitDir = "Column"
)

// DropZone is the area of a group where a dragged tab is released.
type DropZone int

// Drop zones of a group body.
const (
	Center DropZone = iota
	Left
	Right
	Top
	Bottom
)

// Tab is one open page. Its path identifies the content and is reused when already open.
type Tab struct {
	ID    TabID  `json:"id"`
	Path  string `json:"path"`
	Title string `json:"title"`
}

// Group is a leaf of the layout: a tab bar with one active tab (zero when empty).
type Group struct {
	ID     GroupID
	Tabs   []Tab
	Active TabID
}

// Node is either a Leaf (a group) or a *Split.
type Node interface {
	isNode()
}

// Leaf references a group.
type Leaf struct {
	Group GroupID
}

// Split divides its area between First and Second; Ratio is the fraction taken by First.
type Split struct {
	ID     SplitID
	Dir    SplitDir
	Ratio  float64
	First  Node
	Second Node
}

func (Leaf) isNode()   {}
func (*Split) isNode() {}

// Layout is the whole workspace: the split tree, the groups it references and the focused group.
type Layout struct {
	Root    Node
	Groups  []*Group
	Focused GroupID
	nextID  uint64
}

// New returns the initial layout: one empty group.
func New() *Layout {
	return &Layout{
		Root:    Leaf{Group: 1},
		Groups:  []*Group{{ID: 1, Tabs: []Tab{}}},
		Focused: 1,
		nextID:  2,
	}
}

func (l *Layout) freshID() uint64 {
	id := l.nextID
	l.nextID++
	return id
}

// Group returns the group with the given id, or nil.
func (l *Layout) Group(id GroupID) *Group {
	for _, g := range l.Groups {
		if g.ID == id {
			return g
		}
	}
	return nil
}

// GroupOfTab returns the group that contains the tab (zero if none).
func (l *Layout) GroupOfTab(tab TabID) GroupID {
	for _, g := range l.Groups {
		for _, t := range g.Tabs {
			if t.ID == tab {
				return g.ID
			}
		}
	}
	return 0
}

// FindPath returns the group and tab that show path, if it is open.
func (l *Layout) FindPath(path string) (GroupID, TabID, bool) {
	return l.FindPathWhere(func(p string) bool { return p == path })
}

// FindPathWhere returns the first tab, in group order, whose path satisfies match.
func (l *Layout) FindPathWhere(match func(path string) bool) (GroupID, TabID, bool) {
	for _, g := range l.Groups {
		for _, t := range g.Tabs {
			if match(t.Path) {
				return g.ID, t.ID, true
			}
		}
	}
	return 0, 0, false
}

// LeafOrder lists the groups in the order they appear in the tree.
func (l *Layout) LeafOrder() []GroupID {
	out := []GroupID{}
	collectLeaves(l.Root, &out)
	return out
}

// VisiblePaths lists the path of the active tab of every group: what the user sees.
func (l *Layout) VisiblePaths() []string {
	out := []string{}
	for _, g := range l.Groups {
		if path, ok := activePath(g); ok {
			out = append(out, path)
		}
	}
	return out
}

// FocusedPath returns the path of the active tab of the focused group.
func (l *Layout) FocusedPath() (string, bool) {
	g := l.Group(l.Focused)
	if g == nil {
		return "", false
	}
	return activePath(g)
}

// IsPathVisible reports whether path is the active tab of some group.
func (l *Layout) IsPathVisible(path string) bool {
	for _, visible := range l.VisiblePaths() {
		if visible == path {
			return true
		}
	}
	return false
}

func activePath(g *Group) (string, bool) {
	for _, t := range g.Tabs {
		if t.ID == g.Active && g.Active != 0 {
			return t.Path, true
		}
	}
	return "", false
}

// Focus makes group the focused one, if it exists.
func (l *Layout) Focus(group GroupID) {
	if l.Group(group) != nil {
		l.Focused = group
	}
}

// Activate selects tab in its group and focuses that group.
func (l *Layout) Activate(tab TabID) {
	gid := l.GroupOfTab(tab)
	if gid == 0 {
		return
	}
	l.Group(gid).Active = tab
	l.Focused = gid
}

// Open shows path: activates it if already open, otherwise adds a tab to group (or to the
// focused group when group is zero or unknown) and activates it.
func (l *Layout) Open(path, title string, group GroupID) TabID {
	if _, tab, ok := l.FindPath(path); ok {
		l.Activate(tab)
		return tab
	}
	target := l.Focused
	if group != 0 && l.Group(group) != nil {
		target = group
	}
	id := TabID(l.freshID())
	if g := l.Group(target); g != nil {
		g.Tabs = append(g.Tabs, Tab{ID: id, Path: path, Title: title})
		g.Active = id
	}
	l.Focused = target
	return id
}

// OpenBeside opens path in the group right of from (the next leaf), or in a new split to its
// right when there is none. If path is already open it is only activated.
func (l *Layout) OpenBeside(path, title string, from GroupID) TabID {
	if _, _, ok := l.FindPath(path); ok || l.Group(from) == nil {
		return l.Open(path, title, from)
	}
	order := l.LeafOrder()
	target := l.splitOrNext(order, from)
	return l.Open(path, title, target)
}

func (l *Layout) splitOrNext(order []GroupID, from GroupID) GroupID {
	for i, g := range order {
		if g == from && i+1 < len(order) {
			return order[i+1]
		}
	}
	return l.SplitGroup(from, Row)
}

// CloseGroup removes a split without losing tabs: they move to the neighbour (the previous leaf,
// or the next one for the first group). It does nothing when it is the only group.
func (l *Layout) CloseGroup(gid GroupID) {
	if len(l.Groups) <= 1 {
		return
	}
	order := l.LeafOrder()
	position := indexOf(order, gid)
	if position < 0 {
		return
	}
	neighbour := GroupID(0)
	switch {
	case position > 0:
		neighbour = order[position-1]
	case len(order) > 1:
		neighbour = order[1]
	}
	if g := l.Group(gid); g != nil && neighbour != 0 {
		if dest := l.Group(neighbour); dest != nil && len(g.Tabs) > 0 {
			dest.Tabs = append(dest.Tabs, g.Tabs...)
			if g.Active != 0 {
				dest.Active = g.Active
			}
		}
	}
	l.removeGroup(gid)
	if neighbour != 0 {
		l.Focus(neighbour)
	}
}

// ReplacePath changes the path and title of an existing tab. If another tab already shows the
// new path, this tab is closed and that one activated instead.
func (l *Layout) ReplacePath(tab TabID, path, title string) {
	if _, existing, ok := l.FindPath(path); ok && existing != tab {
		l.CloseTab(tab)
		l.Activate(existing)
		return
	}
	for _, g := range l.Groups {
		for i := range g.Tabs {
			if g.Tabs[i].ID == tab {
				g.Tabs[i].Path = path
				g.Tabs[i].Title = title
			}
		}
	}
}

// SetTitle updates the title of every tab showing path.
func (l *Layout) SetTitle(path, title string) {
	for _, g := range l.Groups {
		for i := range g.Tabs {
			if g.Tabs[i].Path == path {
				g.Tabs[i].Title = title
			}
		}
	}
}

func (l *Layout) removeGroup(gid GroupID) {
	if len(l.Groups) <= 1 {
		return
	}
	order := l.LeafOrder()
	position := max(indexOf(order, gid), 0)
	if next := withoutGroup(l.Root, gid); next != nil {
		l.Root = next
	}
	l.Groups = slicesDeleteGroup(l.Groups, gid)
	if l.Focused != gid {
		return
	}
	remaining := l.LeafOrder()
	switch {
	case len(remaining) == 0:
		l.Focused = 1
	case position-1 >= 0 && position-1 < len(remaining):
		l.Focused = remaining[position-1]
	default:
		l.Focused = remaining[0]
	}
}

// CloseTab closes a tab. A group left empty disappears, unless it is the last one.
func (l *Layout) CloseTab(tab TabID) {
	gid := l.GroupOfTab(tab)
	if gid == 0 {
		return
	}
	g := l.Group(gid)
	index := tabIndex(g.Tabs, tab)
	g.Tabs = removeTab(g.Tabs, tab)
	if g.Active == tab {
		g.Active = neighbourTab(g.Tabs, index)
	}
	if len(g.Tabs) == 0 {
		l.removeGroup(gid)
	}
}

// CloseOthers closes every other tab of the group of tab.
func (l *Layout) CloseOthers(tab TabID) {
	gid := l.GroupOfTab(tab)
	if gid == 0 {
		return
	}
	others := []TabID{}
	for _, t := range l.Group(gid).Tabs {
		if t.ID != tab {
			others = append(others, t.ID)
		}
	}
	for _, other := range others {
		l.CloseTab(other)
	}
}

// SplitGroup divides group creating a new empty group on its right (Row) or below (Column),
// and focuses it.
func (l *Layout) SplitGroup(group GroupID, dir SplitDir) GroupID {
	return l.addSplit(group, dir, false)
}

// addSplit places a new empty group next to target and returns it.
func (l *Layout) addSplit(target GroupID, dir SplitDir, newFirst bool) GroupID {
	newGroup := GroupID(l.freshID())
	splitID := SplitID(l.freshID())
	l.Root = splitLeaf(l.Root, target, newGroup, splitID, dir, newFirst)
	l.Groups = append(l.Groups, &Group{ID: newGroup, Tabs: []Tab{}})
	l.Focused = newGroup
	return newGroup
}

// MoveTab moves a tab into group target (Center) or into a new split on the given edge of it.
func (l *Layout) MoveTab(tab TabID, target GroupID, zone DropZone) {
	source := l.GroupOfTab(tab)
	if source == 0 || l.Group(target) == nil {
		return
	}
	sourceLen := len(l.Group(source).Tabs)
	if source == target && (zone == Center || sourceLen <= 1) {
		l.Activate(tab)
		return
	}
	entry, ok := l.detachTab(source, tab)
	if !ok {
		return
	}
	destination := target
	if zone != Center {
		destination = l.addSplit(target, edgeDir(zone), edgeFirst(zone))
	}
	if g := l.Group(destination); g != nil {
		g.Active = entry.ID
		g.Tabs = append(g.Tabs, entry)
	}
	l.Focused = destination
	if g := l.Group(source); g != nil && len(g.Tabs) == 0 {
		l.removeGroup(source)
	}
}

// ReorderTab places tab just before (after == false) or after target, in target's group.
func (l *Layout) ReorderTab(tab, target TabID, after bool) {
	if tab == target {
		return
	}
	source := l.GroupOfTab(tab)
	destination := l.GroupOfTab(target)
	if source == 0 || destination == 0 {
		return
	}
	entry, ok := l.detachTab(source, tab)
	if !ok {
		return
	}
	if g := l.Group(destination); g != nil {
		index := len(g.Tabs)
		if i := tabIndex(g.Tabs, target); i >= 0 {
			index = i
			if after {
				index++
			}
		}
		g.Active = entry.ID
		g.Tabs = insertTab(g.Tabs, index, entry)
	}
	l.Focused = destination
	if source != destination {
		if g := l.Group(source); g != nil && len(g.Tabs) == 0 {
			l.removeGroup(source)
		}
	}
}

// MoveToNewSplit moves tab into a new split next to its own group.
func (l *Layout) MoveToNewSplit(tab TabID, dir SplitDir) {
	gid := l.GroupOfTab(tab)
	if gid == 0 {
		return
	}
	zone := Right
	if dir == Column {
		zone = Bottom
	}
	l.MoveTab(tab, gid, zone)
}

// SetRatio sets the fraction taken by the first child of split, clamped to the allowed range.
func (l *Layout) SetRatio(split SplitID, ratio float64) {
	setSplitRatio(l.Root, split, ratio)
}

// IsConsistent checks that the tree and the groups agree; saved data is untrusted.
func (l *Layout) IsConsistent() bool {
	leaves := l.LeafOrder()
	seen := map[GroupID]bool{}
	for _, id := range leaves {
		if seen[id] || l.Group(id) == nil {
			return false
		}
		seen[id] = true
	}
	if len(leaves) != len(l.Groups) || l.Group(l.Focused) == nil {
		return false
	}
	for _, g := range l.Groups {
		if g.Active != 0 && tabIndex(g.Tabs, g.Active) < 0 {
			return false
		}
	}
	return true
}

// ToJSON serializes the layout in the format the Rust version stored in localStorage.
func (l *Layout) ToJSON() (string, error) {
	data, err := json.Marshal(toWire(l))
	if err != nil {
		return "", fmt.Errorf("marshal dock layout: %w", err)
	}
	return string(data), nil
}

// FromJSON restores a saved layout. It returns false when the JSON is invalid or inconsistent.
func FromJSON(data string) (*Layout, bool) {
	var wire layoutWire
	if err := json.Unmarshal([]byte(data), &wire); err != nil {
		return nil, false
	}
	layout, ok := fromWire(wire)
	if !ok || !layout.IsConsistent() {
		return nil, false
	}
	return layout, true
}

// Shape returns a copy of the tree with every ratio set to zero: two layouts with the same
// shape render the same DOM, so dragging a divider does not rebuild the panes.
func Shape(node Node) Node {
	switch n := node.(type) {
	case Leaf:
		return n
	case *Split:
		return &Split{ID: n.ID, Dir: n.Dir, First: Shape(n.First), Second: Shape(n.Second)}
	default:
		return node
	}
}

// RatioOf returns the current ratio of split, if it is in the tree.
func RatioOf(node Node, split SplitID) (float64, bool) {
	n, ok := node.(*Split)
	if !ok {
		return 0, false
	}
	if n.ID == split {
		return n.Ratio, true
	}
	if ratio, found := RatioOf(n.First, split); found {
		return ratio, true
	}
	return RatioOf(n.Second, split)
}

// detachTab removes tab from source, moving the active selection to a neighbour if needed.
func (l *Layout) detachTab(source GroupID, tab TabID) (Tab, bool) {
	g := l.Group(source)
	index := tabIndex(g.Tabs, tab)
	if index < 0 {
		return Tab{}, false
	}
	entry := g.Tabs[index]
	g.Tabs = removeTab(g.Tabs, tab)
	if g.Active == tab {
		g.Active = neighbourTab(g.Tabs, index)
	}
	return entry, true
}

func edgeDir(zone DropZone) SplitDir {
	if zone == Top || zone == Bottom {
		return Column
	}
	return Row
}

// edgeFirst reports whether the new group goes before the target (left or top).
func edgeFirst(zone DropZone) bool {
	return zone == Left || zone == Top
}

// neighbourTab picks the tab that takes the place of a removed one at index (or the last one).
func neighbourTab(tabs []Tab, index int) TabID {
	switch {
	case index < len(tabs):
		return tabs[index].ID
	case len(tabs) > 0:
		return tabs[len(tabs)-1].ID
	default:
		return 0
	}
}

func tabIndex(tabs []Tab, id TabID) int {
	for i, t := range tabs {
		if t.ID == id {
			return i
		}
	}
	return -1
}

func indexOf(groups []GroupID, id GroupID) int {
	for i, g := range groups {
		if g == id {
			return i
		}
	}
	return -1
}

func removeTab(tabs []Tab, id TabID) []Tab {
	out := make([]Tab, 0, len(tabs))
	for _, t := range tabs {
		if t.ID != id {
			out = append(out, t)
		}
	}
	return out
}

func insertTab(tabs []Tab, index int, entry Tab) []Tab {
	out := make([]Tab, 0, len(tabs)+1)
	out = append(out, tabs[:index]...)
	out = append(out, entry)
	return append(out, tabs[index:]...)
}

func slicesDeleteGroup(groups []*Group, id GroupID) []*Group {
	out := make([]*Group, 0, len(groups))
	for _, g := range groups {
		if g.ID != id {
			out = append(out, g)
		}
	}
	return out
}

// withoutGroup removes the leaf gid from the tree; nil when nothing is left.
func withoutGroup(node Node, gid GroupID) Node {
	switch n := node.(type) {
	case Leaf:
		if n.Group == gid {
			return nil
		}
		return n
	case *Split:
		first := withoutGroup(n.First, gid)
		second := withoutGroup(n.Second, gid)
		switch {
		case first != nil && second != nil:
			return &Split{ID: n.ID, Dir: n.Dir, Ratio: n.Ratio, First: first, Second: second}
		case first != nil:
			return first
		default:
			return second
		}
	default:
		return node
	}
}

// splitLeaf replaces the leaf target by a split holding target and the new group.
func splitLeaf(node Node, target, newGroup GroupID, splitID SplitID, dir SplitDir, newFirst bool) Node {
	switch n := node.(type) {
	case Leaf:
		if n.Group != target {
			return n
		}
		old := Leaf{Group: target}
		fresh := Leaf{Group: newGroup}
		first, second := Node(old), Node(fresh)
		if newFirst {
			first, second = fresh, old
		}
		return &Split{ID: splitID, Dir: dir, Ratio: 0.5, First: first, Second: second}
	case *Split:
		return &Split{
			ID:     n.ID,
			Dir:    n.Dir,
			Ratio:  n.Ratio,
			First:  splitLeaf(n.First, target, newGroup, splitID, dir, newFirst),
			Second: splitLeaf(n.Second, target, newGroup, splitID, dir, newFirst),
		}
	default:
		return node
	}
}

func setSplitRatio(node Node, split SplitID, value float64) bool {
	n, ok := node.(*Split)
	if !ok {
		return false
	}
	if n.ID == split {
		n.Ratio = min(max(value, minRatio), maxRatio)
		return true
	}
	return setSplitRatio(n.First, split, value) || setSplitRatio(n.Second, split, value)
}

func collectLeaves(node Node, out *[]GroupID) {
	switch n := node.(type) {
	case Leaf:
		*out = append(*out, n.Group)
	case *Split:
		collectLeaves(n.First, out)
		collectLeaves(n.Second, out)
	}
}
