package components

import "encoding/json"

// orderVals is the hx-vals of a move: the whole order of the workspace hooks, comma-separated.
func orderVals(order string) string {
	return jsonVals(map[string]string{"order": order})
}

// kindVals is the hx-vals of a kind button: the kind it switches the editor to.
func kindVals(kind string) string {
	return jsonVals(map[string]string{"kind": kind})
}

// enabledVals is the hx-vals of the toggle: the checkbox-like field is sent only to enable the hook.
func enabledVals(enable bool) string {
	if !enable {
		return jsonVals(map[string]string{})
	}
	return jsonVals(map[string]string{"enabled": "on"})
}

// jsonVals encodes a map of strings for an hx-vals attribute. A map of strings always encodes, so Marshal cannot fail
// here and its error is not checked.
func jsonVals(values map[string]string) string {
	encoded, _ := json.Marshal(values)
	return string(encoded)
}

// browseVals is the hx-vals of a browser button: the field it browses and the folder to list or the path to pick.
func browseVals(field string, path string, pick string) string {
	values := map[string]string{"browse": field}
	if path != "" {
		values["browse_path"] = path
	}
	if pick != "" {
		values["pick"] = pick
	}
	return jsonVals(values)
}

// closeVals is the hx-vals that load the editor again without the browser.
func closeVals() string {
	return jsonVals(map[string]string{"browse": ""})
}
