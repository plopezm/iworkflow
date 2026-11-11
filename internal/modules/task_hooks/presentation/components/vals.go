package components

import (
	"encoding/json"

	"iworkflow/internal/modules/task_hooks/presentation/state"
)

// hostTarget is the selector of the screen: every action replaces it.
const hostTarget = "#" + state.HostID

// hookVals is the query of the editor that opens a stored hook.
func hookVals(id string) string {
	return jsonVals(map[string]string{"hook": id})
}

// kindVals is the hx-vals of a kind button: the kind it switches the editor to.
func kindVals(kind string) string {
	return jsonVals(map[string]string{"kind": kind})
}

// assignmentVals is the hx-vals of a stored assignment's toggle: its values, with the enabled flag set to the new
// state. A disabled assignment omits the flag, as the checkbox of the form does.
func assignmentVals(form state.AssignmentForm, enabled bool) string {
	values := map[string]string{
		"id": form.ID, "project": form.Project, "event": form.Event, "phase": form.Phase, "on_failure": form.OnFailure,
	}
	if enabled {
		values["enabled"] = "on"
	}
	return jsonVals(values)
}

// orderVals is the hx-vals of a move: the group the move is in and the whole order of that group.
func orderVals(group state.Group, order string) string {
	return jsonVals(map[string]string{"project": group.Project, "event": group.Event, "phase": group.Phase, "order": order})
}

// browseVals is the hx-vals of a browser button: the field it browses and the folder to list or the path to pick.
func browseVals(field string, path string, pick string) string {
	values := map[string]string{state.BrowseField: field}
	if path != "" {
		values[state.BrowsePathField] = path
	}
	if pick != "" {
		values[state.PickField] = pick
	}
	return jsonVals(values)
}

// closeVals is the hx-vals that load the editor again without the browser.
func closeVals() string {
	return jsonVals(map[string]string{state.BrowseField: ""})
}

// jsonVals encodes a map of strings for an hx-vals attribute. A map of strings always encodes, so Marshal cannot fail
// here and its error is not checked.
func jsonVals(values map[string]string) string {
	encoded, _ := json.Marshal(values)
	return string(encoded)
}

// advancedOpen keeps the advanced options unfolded when they hold a value or the folder browser is working on one.
func advancedOpen(view state.Screen) bool {
	editor := view.Editor
	if editor == nil {
		return false
	}
	browsing := view.Browser != nil && view.Browser.Field == "working_dir"
	return editor.Args != "" || editor.Env != "" || editor.WorkingDir != "" || browsing
}
