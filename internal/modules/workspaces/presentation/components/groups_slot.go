package components

import "net/url"

// registerGroupsSlotID is the id of the registration group selector. The lazy slot, the loaded selector and the empty
// placeholder all carry it: the slot is swapped out with outerHTML, and later swaps still target this id.
const registerGroupsSlotID = "workspace-register-groups"

// registerGroupsURL is the address of the registration group selector, keeping the chosen group.
func registerGroupsURL(selected string) string {
	return "/workspaces/register/groups?group_id=" + url.QueryEscape(selected)
}
