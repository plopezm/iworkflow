package components

import (
	"context"
	"encoding/json"
	"strconv"
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

// groupToggleTitle is the tooltip of a group toggle: the action it performs.
func groupToggleTitle(ctx context.Context, collapsed bool) string {
	if collapsed {
		return msgExpandGroup.T(ctx)
	}
	return msgCollapseGroup.T(ctx)
}
