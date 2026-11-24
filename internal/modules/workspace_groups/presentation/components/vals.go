package components

import "encoding/json"

// groupVals is the hx-vals of an assignment: the group the workspace moves to, or "" to take it out of its group.
func groupVals(groupID string) string {
	encoded, _ := json.Marshal(map[string]string{"group": groupID})
	return string(encoded)
}
