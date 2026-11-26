package pages

// editGroupsSlotID is the id of the edit tab group selector. The lazy slot, the loaded selector and the empty placeholder
// all carry it: the slot is swapped out with outerHTML, and later swaps still target this id.
const editGroupsSlotID = "workspace-edit-groups"

// editGroupsURL is the address of the edit tab group selector of one workspace.
func editGroupsURL(id string) string { return "/workspaces/" + id + "/edit/groups" }
