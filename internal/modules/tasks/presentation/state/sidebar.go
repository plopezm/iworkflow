package state

import (
	"sort"
	"strings"
	"unicode"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
)

// SidebarWorkspace is a workspace row of the sidebar, linked to its board.
type SidebarWorkspace struct {
	ID       string
	Name     string
	Subtitle string
	// Color is the palette key of the workspace; empty means none.
	Color string
	// Pinned reports whether the row sits in the pinned block.
	Pinned bool
	// GroupID is the group the row sits in; empty when it is pinned or ungrouped.
	GroupID string
}

// SidebarGroup is a collapsible block of the sidebar with the workspaces it holds.
type SidebarGroup struct {
	ID        string
	Name      string
	Color     string
	Collapsed bool
	// NeedsAttention reports whether a task of one of the group's workspaces needs the user.
	NeedsAttention bool
	Workspaces     []SidebarWorkspace
}

// SidebarTask is a task row of the sidebar's "needs attention" list, linked to its detail.
type SidebarTask struct {
	ID       string
	Title    string
	Subtitle string
}

// SidebarRows are the lists of the sidebar. Workspaces is every workspace in the given order. Pinned, Groups and
// Ungrouped are the same workspaces arranged by the layout (see ApplyLayout).
type SidebarRows struct {
	Workspaces []SidebarWorkspace
	Pinned     []SidebarWorkspace
	Groups     []SidebarGroup
	Ungrouped  []SidebarWorkspace
	Attention  []SidebarTask
	// NeedsAttentionIn holds the ids of the workspaces that have a task which needs the user.
	NeedsAttentionIn map[string]bool
	// LoadError is the message of a failed read of the sidebar data; empty when the data is fresh.
	LoadError string
	// Query is the trimmed text the rows were narrowed by (see FilterSidebar); empty when no filter applies.
	Query string
}

// UngroupedRows are the workspaces the flat part of the sidebar lists. Once a layout is applied these are the
// ungrouped ones. Rows that were never arranged (no pinned block, no groups, no ungrouped list) list every workspace,
// which is the flat sidebar.
func (r SidebarRows) UngroupedRows() []SidebarWorkspace {
	if r.Pinned == nil && r.Groups == nil && r.Ungrouped == nil {
		return r.Workspaces
	}
	return r.Ungrouped
}

// BuildSidebar derives the sidebar lists (function pura). Workspaces keep the order they are given in. The
// attention list holds the active tasks that need the user, the most recent update first, with the name of
// their workspace as subtitle.
func BuildSidebar(workspaces []dto.WorkspaceOptionDTO, active []dto.TaskDTO) SidebarRows {
	names := make(map[string]string, len(workspaces))
	rows := SidebarRows{Workspaces: make([]SidebarWorkspace, 0, len(workspaces)), Attention: make([]SidebarTask, 0), NeedsAttentionIn: map[string]bool{}}
	for _, workspace := range workspaces {
		names[workspace.ID] = workspace.Name
		rows.Workspaces = append(rows.Workspaces, SidebarWorkspace{ID: workspace.ID, Name: workspace.Name, Subtitle: workspace.BaseBranch, Color: workspace.Color})
	}
	rows.Ungrouped = append([]SidebarWorkspace(nil), rows.Workspaces...)
	attention := make([]dto.TaskDTO, 0)
	for _, task := range active {
		if task.NeedsAttention {
			attention = append(attention, task)
		}
	}
	sort.SliceStable(attention, func(i int, j int) bool {
		return parseTime(attention[i].UpdatedAt).After(parseTime(attention[j].UpdatedAt))
	})
	for _, task := range attention {
		rows.Attention = append(rows.Attention, SidebarTask{ID: task.ID, Title: TaskTitle(task), Subtitle: names[task.WorkspaceID]})
		rows.NeedsAttentionIn[task.WorkspaceID] = true
	}
	return rows
}

// ApplyLayout repartitions the workspaces of rows by the layout (función pura). Each workspace appears once: a
// pinned one only in the pinned block, then in the first group that lists it, then as ungrouped. Groups keep the
// layout order and empty groups stay. A workspace the layout does not mention is appended to the ungrouped ones,
// and ids the rows do not know are ignored. A group needs attention when a task of one of its own workspaces does.
func ApplyLayout(rows SidebarRows, layout models.SidebarLayout) SidebarRows {
	byID := make(map[string]SidebarWorkspace, len(rows.Workspaces))
	for _, workspace := range rows.Workspaces {
		byID[workspace.ID] = workspace
	}
	placed := make(map[string]bool, len(rows.Workspaces))
	out := SidebarRows{Workspaces: rows.Workspaces, Attention: rows.Attention, NeedsAttentionIn: rows.NeedsAttentionIn, LoadError: rows.LoadError}
	for _, id := range layout.Pinned {
		workspace, known := byID[id]
		if !known || placed[id] {
			continue
		}
		placed[id] = true
		workspace.Pinned = true
		out.Pinned = append(out.Pinned, workspace)
	}
	out.Groups = make([]SidebarGroup, 0, len(layout.Groups))
	for _, group := range layout.Groups {
		view := SidebarGroup{ID: group.ID, Name: group.Name, Color: group.Color, Collapsed: group.Collapsed, Workspaces: []SidebarWorkspace{}}
		for _, id := range group.Workspaces {
			workspace, known := byID[id]
			if !known || placed[id] {
				continue
			}
			placed[id] = true
			workspace.GroupID = group.ID
			view.Workspaces = append(view.Workspaces, workspace)
		}
		out.Groups = append(out.Groups, view)
	}
	for _, id := range layout.Ungrouped {
		workspace, known := byID[id]
		if !known || placed[id] {
			continue
		}
		placed[id] = true
		out.Ungrouped = append(out.Ungrouped, workspace)
	}
	for _, workspace := range rows.Workspaces {
		if !placed[workspace.ID] {
			placed[workspace.ID] = true
			out.Ungrouped = append(out.Ungrouped, workspace)
		}
	}
	for i := range out.Groups {
		out.Groups[i].NeedsAttention = needsAttention(out.Groups[i].Workspaces, rows.NeedsAttentionIn)
	}
	return out
}

// needsAttention reports whether one of the workspaces has a task that needs the user.
func needsAttention(workspaces []SidebarWorkspace, attention map[string]bool) bool {
	for _, workspace := range workspaces {
		if attention[workspace.ID] {
			return true
		}
	}
	return false
}

// FilterSidebar narrows the workspace rows to the ones matching query (función pura). A row matches when its name
// or its subtitle contains the query, ignoring case and accents. The query is trimmed, and an empty one returns rows
// unchanged. Groups without a match are dropped; the groups that remain are shown expanded, while the stored
// Collapsed flag of the input is left as it is. The attention list is never filtered.
func FilterSidebar(rows SidebarRows, query string) SidebarRows {
	trimmed := strings.TrimSpace(query)
	needle := foldText(trimmed)
	if needle == "" {
		return rows
	}
	matches := func(workspace SidebarWorkspace) bool {
		return strings.Contains(foldText(workspace.Name), needle) || strings.Contains(foldText(workspace.Subtitle), needle)
	}
	out := rows
	out.Query = trimmed
	out.Workspaces = keepMatching(rows.Workspaces, matches)
	out.Pinned = keepMatching(rows.Pinned, matches)
	out.Ungrouped = keepMatching(rows.Ungrouped, matches)
	if rows.Groups != nil {
		out.Groups = make([]SidebarGroup, 0, len(rows.Groups))
		for _, group := range rows.Groups {
			kept := keepMatching(group.Workspaces, matches)
			if len(kept) == 0 {
				continue
			}
			group.Workspaces = kept
			group.Collapsed = false
			out.Groups = append(out.Groups, group)
		}
	}
	return out
}

// keepMatching returns the rows that match, in their order. A nil list stays nil, so an arranged sidebar and a flat
// one keep telling themselves apart (see UngroupedRows).
func keepMatching(workspaces []SidebarWorkspace, matches func(SidebarWorkspace) bool) []SidebarWorkspace {
	if workspaces == nil {
		return nil
	}
	kept := make([]SidebarWorkspace, 0, len(workspaces))
	for _, workspace := range workspaces {
		if matches(workspace) {
			kept = append(kept, workspace)
		}
	}
	return kept
}

// accentBases maps each accented letter of the Latin alphabet to the plain letter it carries.
var accentBases = buildAccentBases(map[rune]string{
	'a': "àáâãäåāăą",
	'c': "çćĉċč",
	'd': "ďđ",
	'e': "èéêëēĕėęě",
	'g': "ĝğġģ",
	'h': "ĥħ",
	'i': "ìíîïĩīĭįı",
	'j': "ĵ",
	'k': "ķ",
	'l': "ĺļľŀł",
	'n': "ñńņňŉ",
	'o': "òóôõöøōŏő",
	'r': "ŕŗř",
	's': "śŝşš",
	't': "ţťŧ",
	'u': "ùúûüũūŭůűų",
	'w': "ŵ",
	'y': "ýÿŷ",
	'z': "źżž",
})

func buildAccentBases(groups map[rune]string) map[rune]rune {
	bases := make(map[rune]rune)
	for plain, accented := range groups {
		for _, accent := range accented {
			bases[accent] = plain
		}
	}
	return bases
}

// foldText lowercases text and drops the accents of its Latin letters, so that a query matches a name whatever its
// case or its accents.
func foldText(text string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		if plain, ok := accentBases[r]; ok {
			r = plain
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}
