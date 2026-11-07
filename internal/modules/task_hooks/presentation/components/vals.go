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
