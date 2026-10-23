package dock

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// twoTabs returns a layout with /a and /b open in the first group.
func twoTabs(t *testing.T) (*Layout, TabID, TabID) {
	t.Helper()
	layout := New()
	a := layout.Open("/a", "A", 0)
	b := layout.Open("/b", "B", 0)
	return layout, a, b
}

func tabPaths(layout *Layout, group GroupID) []string {
	out := []string{}
	if g := layout.Group(group); g != nil {
		for _, t := range g.Tabs {
			out = append(out, t.Path)
		}
	}
	return out
}

func firstSplit(t *testing.T, node Node) *Split {
	t.Helper()
	split, ok := node.(*Split)
	require.True(t, ok, "expected a split")
	return split
}

func TestOpenAddsAndActivatesTheTab(t *testing.T) {
	layout, _, b := twoTabs(t)
	group := layout.Group(layout.Focused)
	require.NotNil(t, group)
	require.Len(t, group.Tabs, 2)
	require.Equal(t, b, group.Active)
}

func TestFindPathWhereReturnsTheFirstMatchingTab(t *testing.T) {
	layout, a, _ := twoTabs(t)

	_, found, ok := layout.FindPathWhere(func(p string) bool { return p[0] == '/' })
	require.True(t, ok)
	require.Equal(t, a, found)

	_, _, ok = layout.FindPathWhere(func(p string) bool { return p == "/none" })
	require.False(t, ok)
}

func TestOpenBesideSplitsOnceThenReusesTheRightGroup(t *testing.T) {
	layout := New()
	tree := layout.Focused
	layout.Open("/tree", "T", 0)

	layout.OpenBeside("/f1", "F1", tree)
	require.Len(t, layout.Groups, 2)
	right := layout.Focused
	require.NotEqual(t, tree, right)

	layout.OpenBeside("/f2", "F2", tree)
	require.Len(t, layout.Groups, 2)
	require.Len(t, layout.Group(right).Tabs, 2)
	require.Len(t, layout.Group(tree).Tabs, 1)
	require.True(t, layout.IsConsistent())
}

func TestOpenBesideExistingPathOnlyActivates(t *testing.T) {
	layout := New()
	tree := layout.Focused
	first := layout.OpenBeside("/f1", "F1", tree)
	again := layout.OpenBeside("/f1", "F1", tree)
	require.Equal(t, first, again)
	require.Len(t, layout.Groups, 2)
}

func TestOpenExistingPathActivatesInsteadOfDuplicating(t *testing.T) {
	layout, a, _ := twoTabs(t)
	again := layout.Open("/a", "A", 0)
	require.Equal(t, a, again)
	require.Len(t, layout.Groups[0].Tabs, 2)
	require.Equal(t, a, layout.Groups[0].Active)
}

func TestCloseActiveTabSelectsNeighbourAndLastGroupSurvives(t *testing.T) {
	layout, a, b := twoTabs(t)
	layout.CloseTab(b)
	require.Equal(t, a, layout.Groups[0].Active)
	layout.CloseTab(a)
	require.Len(t, layout.Groups, 1)
	require.Empty(t, layout.Groups[0].Tabs)
	require.Equal(t, TabID(0), layout.Groups[0].Active)
}

func TestSplitGroupAddsAnEmptyFocusedGroup(t *testing.T) {
	layout, _, _ := twoTabs(t)
	newGroup := layout.SplitGroup(1, Row)
	require.Len(t, layout.Groups, 2)
	require.Equal(t, newGroup, layout.Focused)
	require.Equal(t, []GroupID{1, newGroup}, layout.LeafOrder())
	require.True(t, layout.IsConsistent())
}

func TestClosingLastTabOfASplitGroupCollapsesTheSplit(t *testing.T) {
	layout, _, _ := twoTabs(t)
	newGroup := layout.SplitGroup(1, Row)
	tab := layout.Open("/c", "C", newGroup)
	layout.CloseTab(tab)
	require.Len(t, layout.Groups, 1)
	require.Equal(t, Node(Leaf{Group: 1}), layout.Root)
	require.Equal(t, GroupID(1), layout.Focused)
}

func TestReorderTabMovesItAfterTheTargetInTheSameGroup(t *testing.T) {
	layout, a, b := twoTabs(t)
	c := layout.Open("/c", "C", 0)
	layout.ReorderTab(a, c, true)
	require.Equal(t, []string{"/b", "/c", "/a"}, tabPaths(layout, 1))
	require.Equal(t, a, layout.Groups[0].Active)
	layout.ReorderTab(c, b, false)
	require.Equal(t, []string{"/c", "/b", "/a"}, tabPaths(layout, 1))
}

func TestReorderTabOntoItselfOrUnknownTabChangesNothing(t *testing.T) {
	layout, a, _ := twoTabs(t)
	layout.ReorderTab(a, a, true)
	layout.ReorderTab(a, 999, true)
	require.Equal(t, []string{"/a", "/b"}, tabPaths(layout, 1))
}

func TestReorderTabAcrossGroupsInsertsNextToTheTarget(t *testing.T) {
	layout, a, b := twoTabs(t)
	layout.MoveTab(b, 1, Right)
	right := layout.GroupOfTab(b)
	c := layout.Open("/c", "C", right)
	layout.ReorderTab(a, c, false)
	require.Equal(t, []string{"/b", "/a", "/c"}, tabPaths(layout, right))
	require.Equal(t, right, layout.Focused)
	require.Len(t, layout.Groups, 1, "emptied source group is removed")
	require.True(t, layout.IsConsistent())
}

func TestMoveTabToEdgeCreatesANewSplitOnThatSide(t *testing.T) {
	layout, a, _ := twoTabs(t)
	layout.MoveTab(a, 1, Left)
	require.Len(t, layout.Groups, 2)
	order := layout.LeafOrder()
	require.Equal(t, GroupID(1), order[1], "the original group stays on the right")
	require.Equal(t, order[0], layout.GroupOfTab(a))
	require.True(t, layout.IsConsistent())
}

func TestMoveTabToCenterOfAnotherGroupRemovesTheEmptiedSource(t *testing.T) {
	layout, a, b := twoTabs(t)
	layout.MoveTab(b, 1, Bottom)
	require.NotZero(t, layout.GroupOfTab(b))
	layout.MoveTab(b, 1, Center)
	require.Len(t, layout.Groups, 1, "emptied group is removed")
	require.Equal(t, GroupID(1), layout.GroupOfTab(a))
	require.Equal(t, GroupID(1), layout.GroupOfTab(b))
	require.Equal(t, Node(Leaf{Group: 1}), layout.Root)
}

func TestMovingTheOnlyTabOntoItsOwnEdgeIsANoop(t *testing.T) {
	layout := New()
	a := layout.Open("/a", "A", 0)
	layout.MoveTab(a, 1, Right)
	require.Len(t, layout.Groups, 1)
}

func TestSetRatioIsClamped(t *testing.T) {
	layout, _, _ := twoTabs(t)
	layout.SplitGroup(1, Column)
	split := firstSplit(t, layout.Root)
	layout.SetRatio(split.ID, 0.0)
	require.InDelta(t, minRatio, firstSplit(t, layout.Root).Ratio, 1e-9)

	layout.SetRatio(split.ID, 2.0)
	require.InDelta(t, maxRatio, firstSplit(t, layout.Root).Ratio, 1e-9)
}

func TestShapeIgnoresRatiosButRatioOfReadsThem(t *testing.T) {
	layout, _, _ := twoTabs(t)
	layout.SplitGroup(1, Row)
	before := Shape(layout.Root)
	split := firstSplit(t, layout.Root)
	layout.SetRatio(split.ID, 0.3)
	require.Equal(t, before, Shape(layout.Root))
	ratio, ok := RatioOf(layout.Root, split.ID)
	require.True(t, ok)
	require.InDelta(t, 0.3, ratio, 1e-9)
	_, ok = RatioOf(layout.Root, 999)
	require.False(t, ok)
}

func TestReplacePathMergesIntoAnExistingTabForThatPath(t *testing.T) {
	layout, a, b := twoTabs(t)
	layout.ReplacePath(b, "/a", "A")
	require.Len(t, layout.Groups[0].Tabs, 1)
	require.Equal(t, a, layout.Groups[0].Active)
}

func TestCloseGroupRemovesAnEmptySplit(t *testing.T) {
	layout, _, _ := twoTabs(t)
	g := layout.SplitGroup(1, Row)
	layout.CloseGroup(g)
	require.Len(t, layout.Groups, 1)
	require.Equal(t, Node(Leaf{Group: 1}), layout.Root)
	require.Equal(t, GroupID(1), layout.Focused)
}

func TestCloseGroupMergesTabsIntoTheNeighbour(t *testing.T) {
	layout, _, _ := twoTabs(t)
	g := layout.SplitGroup(1, Row)
	c := layout.Open("/c", "C", g)
	layout.CloseGroup(g)
	require.Len(t, layout.Groups, 1)
	require.Equal(t, GroupID(1), layout.GroupOfTab(c))
	require.Len(t, layout.Groups[0].Tabs, 3)
	require.Equal(t, c, layout.Groups[0].Active)
}

func TestCloseGroupOnTheFirstGroupMergesIntoTheNext(t *testing.T) {
	layout, a, _ := twoTabs(t)
	g := layout.SplitGroup(1, Row)
	layout.CloseGroup(1)
	require.Equal(t, []GroupID{g}, layout.LeafOrder())
	require.Equal(t, g, layout.GroupOfTab(a))
	require.Equal(t, g, layout.Focused)
}

func TestCloseGroupKeepsTheOnlyGroup(t *testing.T) {
	layout, _, _ := twoTabs(t)
	layout.CloseGroup(1)
	require.Len(t, layout.Groups, 1)
	require.Len(t, layout.Groups[0].Tabs, 2)
}

func TestJSONRoundTripAndRejectsInconsistentData(t *testing.T) {
	layout, _, _ := twoTabs(t)
	layout.SplitGroup(1, Row)
	saved, err := layout.ToJSON()
	require.NoError(t, err)

	restored, ok := FromJSON(saved)
	require.True(t, ok)
	require.Equal(t, layout, restored)

	broken := *layout
	broken.Groups = broken.Groups[:1]
	brokenJSON, err := broken.ToJSON()
	require.NoError(t, err)
	_, ok = FromJSON(brokenJSON)
	require.False(t, ok)

	_, ok = FromJSON("not json")
	require.False(t, ok)
}

func TestFocusedPathFollowsTheFocusedGroup(t *testing.T) {
	layout, _, _ := twoTabs(t)
	path, ok := layout.FocusedPath()
	require.True(t, ok)
	require.Equal(t, "/b", path)

	g := layout.SplitGroup(1, Row)
	_, ok = layout.FocusedPath()
	require.False(t, ok, "new group is empty")
	layout.Open("/c", "C", g)
	path, ok = layout.FocusedPath()
	require.True(t, ok)
	require.Equal(t, "/c", path)
	layout.Focus(1)
	path, _ = layout.FocusedPath()
	require.Equal(t, "/b", path)
}

func TestVisiblePathsListsTheActiveTabOfEveryGroup(t *testing.T) {
	layout, _, _ := twoTabs(t)
	g := layout.SplitGroup(1, Row)
	layout.Open("/c", "C", g)
	paths := layout.VisiblePaths()
	require.ElementsMatch(t, []string{"/b", "/c"}, paths)
	require.True(t, layout.IsPathVisible("/c"))
	require.False(t, layout.IsPathVisible("/a"))
}

func TestLayoutJSONMatchesTheRustSerdeShape(t *testing.T) {
	layout := New()
	layout.Open("/a", "A", 0)
	layout.SplitGroup(1, Row)

	saved, err := layout.ToJSON()
	require.NoError(t, err)
	require.JSONEq(t, `{
		"root": {"Split": {"id": 4, "dir": "Row", "ratio": 0.5,
			"first": {"Group": 1}, "second": {"Group": 3}}},
		"groups": [
			{"id": 1, "tabs": [{"id": 2, "path": "/a", "title": "A"}], "active": 2},
			{"id": 3, "tabs": [], "active": null}
		],
		"focused": 3,
		"next_id": 5
	}`, saved)
}
