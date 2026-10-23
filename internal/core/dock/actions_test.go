package dock

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func actionRegistry(t *testing.T) *Registry {
	t.Helper()
	registry, err := NewRegistry(
		Route{Name: "board", Pattern: "/", Title: func(context.Context, Params) string { return "Board" }},
		Route{Name: "jira", Pattern: "/jira", Title: func(context.Context, Params) string { return "Jira" }},
	)
	require.NoError(t, err)
	return registry
}

func TestApplyOpenUsesTheRouteTitleWhenNoneIsGiven(t *testing.T) {
	layout := New()
	err := Apply(context.Background(), layout, actionRegistry(t), "open", Request{Path: "/jira"})
	require.NoError(t, err)
	require.Equal(t, "Jira", layout.Group(1).Tabs[0].Title)
}

func TestApplyOpenRequiresAPath(t *testing.T) {
	err := Apply(context.Background(), New(), actionRegistry(t), "open", Request{})
	require.Error(t, err)
}

func TestApplyOpenOrReuseActivatesAnOpenTabOfThatRoute(t *testing.T) {
	registry := actionRegistry(t)
	layout := New()
	require.NoError(t, Apply(context.Background(), layout, registry, "open", Request{Path: "/w/1"}))
	require.NoError(t, Apply(context.Background(), layout, registry, "open", Request{Path: "/"}))
	require.NoError(t, Apply(context.Background(), layout, registry, "open_or_reuse", Request{Path: "/", Reuse: "board"}))

	require.Len(t, layout.Group(1).Tabs, 2, "no duplicate board is opened")
	require.Equal(t, "/", layout.Group(1).Tabs[1].Path)
	require.Equal(t, layout.Group(1).Tabs[1].ID, layout.Group(1).Active)
}

func TestApplyTableCoversEveryOperation(t *testing.T) {
	registry := actionRegistry(t)
	cases := []struct {
		name  string
		setup func(*Layout) TabID
		op    string
		req   func(TabID, GroupID) Request
		check func(*testing.T, *Layout, TabID)
	}{
		{
			name:  "close_tab removes the tab",
			setup: func(l *Layout) TabID { return l.Open("/a", "A", 0) },
			op:    "close_tab",
			req:   func(tab TabID, _ GroupID) Request { return Request{Tab: tab} },
			check: func(t *testing.T, l *Layout, _ TabID) { require.Empty(t, l.Group(1).Tabs) },
		},
		{
			name:  "split_group adds a row split",
			setup: func(l *Layout) TabID { return 0 },
			op:    "split_group",
			req:   func(_ TabID, g GroupID) Request { return Request{Group: g, Dir: Row} },
			check: func(t *testing.T, l *Layout, _ TabID) {
				require.Len(t, l.Groups, 2)
				require.IsType(t, &Split{}, l.Root)
			},
		},
		{
			name: "move_to_new_split moves to the right",
			// Two tabs: moving the only tab onto its own edge is a no-op (see layout_test).
			setup: func(l *Layout) TabID { l.Open("/a", "A", 0); return l.Open("/b", "B", 0) },
			op:    "move_to_new_split",
			req:   func(tab TabID, _ GroupID) Request { return Request{Tab: tab, Dir: Row} },
			check: func(t *testing.T, l *Layout, tab TabID) {
				require.Len(t, l.Groups, 2)
				require.Equal(t, tab, l.Group(l.Focused).Active)
			},
		},
		{
			name:  "set_ratio clamps",
			setup: func(l *Layout) TabID { l.SplitGroup(1, Column); return 0 },
			op:    "set_ratio",
			// New() hands out id 2 to the new group and 3 to the split.
			req: func(_ TabID, _ GroupID) Request { return Request{Split: 3, Ratio: 0.99} },
			check: func(t *testing.T, l *Layout, _ TabID) {
				ratio, ok := RatioOf(l.Root, 3)
				require.True(t, ok)
				require.InDelta(t, maxRatio, ratio, 1e-9)
			},
		},
		{
			name:  "set_title renames every tab of the path",
			setup: func(l *Layout) TabID { return l.Open("/tasks/a", "old", 0) },
			op:    "set_title",
			req:   func(_ TabID, _ GroupID) Request { return Request{Path: "/tasks/a", Title: "new"} },
			check: func(t *testing.T, l *Layout, _ TabID) { require.Equal(t, "new", l.Group(1).Tabs[0].Title) },
		},
		{
			name:  "replace with no tab opens the path",
			setup: func(l *Layout) TabID { return 0 },
			op:    "replace",
			req:   func(_ TabID, _ GroupID) Request { return Request{Path: "/jira"} },
			check: func(t *testing.T, l *Layout, _ TabID) {
				require.Equal(t, "/jira", l.Group(1).Tabs[0].Path)
			},
		},
		{
			name:  "close_and_open opens then closes the current tab",
			setup: func(l *Layout) TabID { return l.Open("/a", "A", 0) },
			op:    "close_and_open",
			req:   func(tab TabID, g GroupID) Request { return Request{Tab: tab, Group: g, Path: "/jira"} },
			check: func(t *testing.T, l *Layout, _ TabID) {
				require.Len(t, l.Group(1).Tabs, 1)
				require.Equal(t, "/jira", l.Group(1).Tabs[0].Path)
			},
		},
		{
			name:  "focus moves the focus",
			setup: func(l *Layout) TabID { l.SplitGroup(1, Row); return 0 },
			op:    "focus",
			req:   func(_ TabID, _ GroupID) Request { return Request{Group: 1} },
			check: func(t *testing.T, l *Layout, _ TabID) { require.Equal(t, GroupID(1), l.Focused) },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			layout := New()
			tab := tc.setup(layout)
			err := Apply(context.Background(), layout, registry, tc.op, tc.req(tab, 1))
			require.NoError(t, err)
			tc.check(t, layout, tab)
		})
	}
}

func TestApplyRejectsUnknownOperations(t *testing.T) {
	err := Apply(context.Background(), New(), actionRegistry(t), "explode", Request{})
	require.True(t, errors.Is(err, ErrUnknownAction))
}

func TestParseRequestReadsEveryField(t *testing.T) {
	values := url.Values{
		"path":   {"/tasks/a"},
		"title":  {"Task"},
		"group":  {"3"},
		"tab":    {"7"},
		"target": {"8"},
		"split":  {"4"},
		"after":  {"1"},
		"zone":   {"bottom"},
		"dir":    {"Column"},
		"ratio":  {"0.4"},
		"reuse":  {"board"},
	}
	req, err := ParseRequest(values)
	require.NoError(t, err)
	require.Equal(t, Request{
		Path: "/tasks/a", Title: "Task", Reuse: "board",
		Group: 3, Tab: 7, Target: 8, Split: 4,
		After: true, Zone: Bottom, Dir: Column, Ratio: 0.4,
	}, req)
}

func TestParseRequestRejectsMalformedNumbers(t *testing.T) {
	_, err := ParseRequest(url.Values{"tab": {"x"}})
	require.Error(t, err)
	_, err = ParseRequest(url.Values{"ratio": {"half"}})
	require.Error(t, err)
}
