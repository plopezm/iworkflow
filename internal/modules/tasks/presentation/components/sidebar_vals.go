package components

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/a-h/templ"

	"iworkflow/internal/modules/tasks/presentation/state"
)

// The hx-vals of the group and workspace actions of the sidebar. The server reads them as form fields.

// jsonVals encodes the values an hx-vals attribute sends.
func jsonVals(values map[string]string) string {
	encoded, _ := json.Marshal(values)
	return string(encoded)
}

// groupCollapseVals is the hx-vals of a group toggle: the state the group switches to.
func groupCollapseVals(collapsed bool) string {
	return jsonVals(map[string]string{"collapsed": strconv.FormatBool(collapsed)})
}

// groupEditVals is the hx-vals of a group recolor: the name is kept, the color is set.
func groupEditVals(name string, color string) string {
	return jsonVals(map[string]string{"name": name, "color": color})
}

// groupColorVals is the hx-vals of a group rename: the color is kept. The rename prompt supplies the name itself
// (see web/static/js/group_prompt.js).
func groupColorVals(color string) string {
	return jsonVals(map[string]string{"color": color})
}

// groupChevron is the mark of a group toggle: closed or open.
func groupChevron(collapsed bool) string {
	if collapsed {
		return "▸"
	}
	return "▾"
}

// workspacePinVals is the hx-vals of a pin toggle: the state the workspace switches to.
func workspacePinVals(pinned bool) string {
	return jsonVals(map[string]string{"pinned": strconv.FormatBool(pinned)})
}

// workspaceGroupVals is the hx-vals of a move: the group the workspace goes to, empty to take it out of its group.
func workspaceGroupVals(groupID string) string {
	return jsonVals(map[string]string{"group": groupID})
}

// groupToggleAttrs are the attributes of a group toggle. While a filter is active the group is shown open whatever its
// stored state, so the toggle is disabled and posts nothing: it must not change the stored state.
func groupToggleAttrs(group state.SidebarGroup, filtering bool) templ.Attributes {
	if filtering {
		return templ.Attributes{"disabled": true, "aria-disabled": "true"}
	}
	return templ.Attributes{
		"hx-post": "/workspace-groups/" + group.ID + "/collapse",
		"hx-vals": groupCollapseVals(!group.Collapsed),
		"hx-swap": "none",
	}
}

// groupToggleClass is the class of a group toggle: a pointer only when it acts.
func groupToggleClass(filtering bool) string {
	if filtering {
		return "flex min-w-0 flex-1 cursor-default items-center gap-1.5 text-left text-[13px] font-medium text-fg"
	}
	return "flex min-w-0 flex-1 cursor-pointer items-center gap-1.5 text-left text-[13px] font-medium text-fg"
}

// groupToggleTitle is the tooltip of a group toggle: the action it performs, or why it does nothing while filtering.
func groupToggleTitle(ctx context.Context, collapsed bool, filtering bool) string {
	if filtering {
		return msgGroupShownOpen.T(ctx)
	}
	if collapsed {
		return msgExpandGroup.T(ctx)
	}
	return msgCollapseGroup.T(ctx)
}
