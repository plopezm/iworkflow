package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// filterFixture is a sidebar with a pinned workspace, two collapsed groups, two ungrouped workspaces and an
// attention task. Its ids name the rows: p1 pinned, w1 and w2 in g1, w3 in g2, w4 and w5 ungrouped.
func filterFixture() state.SidebarRows {
	pinned := state.SidebarWorkspace{ID: "p1", Name: "API Gateway", Subtitle: "main", Pinned: true}
	w1 := state.SidebarWorkspace{ID: "w1", Name: "api", Subtitle: "dev", GroupID: "g1"}
	w2 := state.SidebarWorkspace{ID: "w2", Name: "Café UI", Subtitle: "main", GroupID: "g1"}
	w3 := state.SidebarWorkspace{ID: "w3", Name: "docs", Subtitle: "feature/zeta", GroupID: "g2"}
	w4 := state.SidebarWorkspace{ID: "w4", Name: "Résumé", Subtitle: "main"}
	w5 := state.SidebarWorkspace{ID: "w5", Name: "web", Subtitle: "main"}
	return state.SidebarRows{
		Arranged:   true,
		Workspaces: []state.SidebarWorkspace{pinned, w1, w2, w3, w4, w5},
		Pinned:     []state.SidebarWorkspace{pinned},
		Groups: []state.SidebarGroup{
			{ID: "g1", Name: "Backend", Collapsed: true, NeedsAttention: true, Workspaces: []state.SidebarWorkspace{w1, w2}},
			{ID: "g2", Name: "Docs", Collapsed: true, Workspaces: []state.SidebarWorkspace{w3}},
		},
		Ungrouped:        []state.SidebarWorkspace{w4, w5},
		Attention:        []state.SidebarTask{{ID: "t1", Title: "fix", Subtitle: "web"}},
		NeedsAttentionIn: map[string]bool{"w1": true},
	}
}

// wantGroup is a group of the filtered rows: its id, its stored flag and the ids of the workspaces it still shows.
type wantGroup struct {
	id         string
	collapsed  bool
	workspaces []string
}

func rowIDs(workspaces []state.SidebarWorkspace) []string {
	ids := make([]string, 0, len(workspaces))
	for _, workspace := range workspaces {
		ids = append(ids, workspace.ID)
	}
	return ids
}

func groupsOf(groups []state.SidebarGroup) []wantGroup {
	out := make([]wantGroup, 0, len(groups))
	for _, group := range groups {
		out = append(out, wantGroup{id: group.ID, collapsed: group.Collapsed, workspaces: rowIDs(group.Workspaces)})
	}
	return out
}

func TestFilterSidebarNarrowsTheRowsByNameOrSubtitle(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		query     string
		pinned    []string
		groups    []wantGroup
		ungrouped []string
		count     int
	}{
		{name: "an empty query keeps every row", query: "", pinned: []string{"p1"}, groups: []wantGroup{{"g1", true, []string{"w1", "w2"}}, {"g2", true, []string{"w3"}}}, ungrouped: []string{"w4", "w5"}, count: 6},
		{name: "a blank query is an empty query", query: "   ", pinned: []string{"p1"}, groups: []wantGroup{{"g1", true, []string{"w1", "w2"}}, {"g2", true, []string{"w3"}}}, ungrouped: []string{"w4", "w5"}, count: 6},
		{name: "the match ignores case and hides the groups without a match", query: "API", pinned: []string{"p1"}, groups: []wantGroup{{"g1", false, []string{"w1"}}}, ungrouped: []string{}, count: 2},
		{name: "a query inside a collapsed group shows that group expanded", query: "café", pinned: []string{}, groups: []wantGroup{{"g1", false, []string{"w2"}}}, ungrouped: []string{}, count: 1},
		{name: "an accent in the query matches the plain letter in the name", query: "CAFE", pinned: []string{}, groups: []wantGroup{{"g1", false, []string{"w2"}}}, ungrouped: []string{}, count: 1},
		{name: "a plain query matches a name written with accents", query: "resume", pinned: []string{}, groups: []wantGroup{}, ungrouped: []string{"w4"}, count: 1},
		{name: "an accented uppercase query matches a name written with accents", query: "RÉSUMÉ", pinned: []string{}, groups: []wantGroup{}, ungrouped: []string{"w4"}, count: 1},
		{name: "the query is trimmed before matching", query: "  café  ", pinned: []string{}, groups: []wantGroup{{"g1", false, []string{"w2"}}}, ungrouped: []string{}, count: 1},
		{name: "the subtitle, the branch, is matched too", query: "feature", pinned: []string{}, groups: []wantGroup{{"g2", false, []string{"w3"}}}, ungrouped: []string{}, count: 1},
		{name: "a subtitle shared by several rows counts every match", query: "main", pinned: []string{"p1"}, groups: []wantGroup{{"g1", false, []string{"w2"}}}, ungrouped: []string{"w4", "w5"}, count: 4},
		{name: "no match leaves every list empty", query: "zzz", pinned: []string{}, groups: []wantGroup{}, ungrouped: []string{}, count: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rows := filterFixture()

			got := state.FilterSidebar(rows, tt.query)

			assert.Equal(t, tt.pinned, rowIDs(got.Pinned))
			assert.Equal(t, tt.groups, groupsOf(got.Groups))
			assert.Equal(t, tt.ungrouped, rowIDs(got.Ungrouped))
			assert.Len(t, got.Workspaces, tt.count, "the count of the sidebar is the number of matches")
			assert.Equal(t, rows.Attention, got.Attention, "the attention list is never filtered")
			assert.True(t, rows.Groups[0].Collapsed, "the stored state of the input group is not changed")
			assert.Len(t, rows.Workspaces, 6, "the input rows are not changed")
		})
	}
}

func TestFilterSidebarWithAnEmptyQueryReturnsTheRowsUnchanged(t *testing.T) {
	t.Parallel()
	rows := filterFixture()

	got := state.FilterSidebar(rows, "")

	assert.Equal(t, rows, got)
}

func TestFilterSidebarKeepsAFlatSidebarFlat(t *testing.T) {
	t.Parallel()
	rows := state.BuildSidebar([]dto.WorkspaceOptionDTO{{ID: "w1", Name: "api", BaseBranch: "main"}, {ID: "w2", Name: "web", BaseBranch: "dev"}}, nil)

	got := state.FilterSidebar(rows, "WEB")

	assert.Nil(t, got.Pinned, "a sidebar that was never arranged has no pinned block")
	assert.Nil(t, got.Groups, "a sidebar that was never arranged has no groups")
	assert.Equal(t, []state.SidebarWorkspace{{ID: "w2", Name: "web", Subtitle: "dev"}}, got.UngroupedRows())
}

func TestFilterSidebarRecordsTheQueryItNarrowedBy(t *testing.T) {
	t.Parallel()

	got := state.FilterSidebar(filterFixture(), " Café ")

	assert.Equal(t, "Café", got.Query)
	assert.Equal(t, "", state.FilterSidebar(filterFixture(), "").Query)
}

// RawQuery is the text as typed, for the search input; Query is the trimmed text the rows were matched with.
func TestFilterSidebarKeepsTheRawQueryForTheInput(t *testing.T) {
	t.Parallel()

	got := state.FilterSidebar(filterFixture(), "café ")

	assert.Equal(t, "café ", got.RawQuery)
	assert.Equal(t, "café", got.Query)
	assert.Equal(t, "   ", state.FilterSidebar(filterFixture(), "   ").RawQuery, "a blank query is echoed as typed")
}

// Filtering reports whether a query narrows the rows: a blank one does not.
func TestFilteringIsOnlyForAQueryWithText(t *testing.T) {
	t.Parallel()

	assert.False(t, state.FilterSidebar(filterFixture(), "").Filtering())
	assert.False(t, state.FilterSidebar(filterFixture(), " 	 ").Filtering())
	assert.True(t, state.FilterSidebar(filterFixture(), " api ").Filtering())
}
