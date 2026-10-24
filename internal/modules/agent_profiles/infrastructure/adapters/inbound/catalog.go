package inbound

import (
	"context"

	"iworkflow/internal/modules/agent_profiles/domain/models"
	"iworkflow/internal/modules/agent_profiles/presentation/state"
)

// catalog is what the routing editors and the roles card need besides the profiles: the model
// options, the reasoning levels and the roles. A failed load is kept as text, so the screen still
// renders and shows the failure.
type catalog struct {
	models   []state.Option
	levels   []string
	roles    []models.AgentRole
	err      string
	rolesErr string
}

// loadCatalog reads the models and the roles. The first failure is the one shown above the editor,
// as in the Rust screen.
func (h *Handlers) loadCatalog(ctx context.Context) catalog {
	cat := catalog{levels: models.ThinkingLevels()}
	options, err := h.uses.ListModelOptions.Execute(ctx)
	if err != nil {
		cat.err = err.Error()
	} else {
		cat.models = modelOptions(options)
	}
	roles, err := h.uses.ListAgentRoles.Execute(ctx)
	if err != nil {
		cat.rolesErr = err.Error()
		if cat.err == "" {
			cat.err = cat.rolesErr
		}
	} else {
		cat.roles = roles
	}
	return cat
}

// profileEditor is the routing editor of a profile, with the rows the editor should show.
func profileEditor(name string, rows []state.RoutingRow, cat catalog) state.EditorView {
	return state.EditorView{
		FormID:      "profiles-routing",
		SaveURL:     profileRoutingURL(name),
		RowsURL:     profileRoutingRowsURL(name),
		AddSelectID: "profiles-add-agent",
		Rows:        rows,
		Models:      cat.models,
		Levels:      cat.levels,
		Addable:     state.AddableAgents(roleNames(cat.roles), rows),
	}
}

// globalEditor is the routing editor of the global routing.
func globalEditor(rows []state.RoutingRow, cat catalog) state.EditorView {
	return state.EditorView{
		FormID:      "models-routing",
		SaveURL:     modelsRoutingURL(),
		RowsURL:     modelsRoutingRowsURL(),
		AddSelectID: "models-add-agent",
		Rows:        rows,
		Models:      cat.models,
		Levels:      cat.levels,
		Addable:     state.AddableAgents(roleNames(cat.roles), rows),
	}
}

// profilesView builds the profiles screen for the selection. The selection is resolved the way the
// Rust screen did: the selected profile, else the active one, else the first one.
func (h *Handlers) profilesView(ctx context.Context, selected string, confirmDelete bool) state.ProfilesView {
	cat := h.loadCatalog(ctx)
	roles := state.RolesView{Items: roleItems(cat.roles), Error: cat.rolesErr, NewRoleURL: roleDialogURL(profilesScope)}
	list := state.ListView{
		ID:        profilesListID,
		DialogID:  profilesDialogID,
		SelectURL: selectURL(),
		NewURL:    profileDialogURL("new"),
		ImportURL: profileDialogURL("import"),
	}
	store, err := h.uses.ListProfiles.Execute(ctx)
	if err != nil {
		list.Error = err.Error()
		return state.ProfilesView{List: list, Detail: state.DetailView{Error: err.Error()}, Roles: roles}
	}
	names, active := profileNames(store)
	current := state.ResolveSelected(names, active, selected)
	for _, name := range names {
		list.Items = append(list.Items, state.ListItem{Name: name, Active: name == active, Selected: name == current})
	}
	var detail state.DetailView
	if profile, found := findProfile(store, current); found {
		detail = detailView(profile, active, cat, confirmDelete)
	}
	detail.Error = firstText(detail.Error, cat.err)
	return state.ProfilesView{List: list, Detail: detail, Roles: roles}
}

// modelsView builds the global routing screen. A failed read of the routing replaces the rows.
func (h *Handlers) modelsView(ctx context.Context) state.ModelsView {
	cat := h.loadCatalog(ctx)
	routing, err := h.uses.GetGlobalRouting.Execute(ctx)
	editor := globalEditor(nil, cat)
	if err != nil {
		editor.Error = err.Error()
	} else {
		editor.Rows = state.RowsFromRouting(routingToView(routing))
		editor.Addable = state.AddableAgents(roleNames(cat.roles), editor.Rows)
	}
	editor.Error = firstText(editor.Error, cat.err)
	return state.ModelsView{Editor: editor, NewRoleURL: roleDialogURL(modelsScope)}
}

func detailView(profile models.Profile, active string, cat catalog, confirmDelete bool) state.DetailView {
	name := profile.Name.String()
	rows := state.RowsFromRouting(routingToView(profile.Routing))
	return state.DetailView{
		Present:         true,
		Name:            name,
		Active:          name == active,
		ConfirmDelete:   confirmDelete,
		Error:           cat.err,
		Routing:         profileEditor(name, rows, cat),
		DetailURL:       profileDetailURL(name),
		DuplicateURL:    nameDialogURL("duplicate", name, name),
		RenameURL:       nameDialogURL("rename", name, name),
		DeleteURL:       profileURL(name),
		ExportURL:       exportURL(name),
		ExportNoticeURL: exportNoticeURL(name),
		JSONURL:         jsonURL(name),
		Filename:        name + ".profile.json",
		Selected:        name,
	}
}

// profileNames returns the names in store order and the active name ("" when none).
func profileNames(store models.ProfileStore) ([]string, string) {
	names := make([]string, 0, len(store.Profiles))
	for _, profile := range store.Profiles {
		names = append(names, profile.Name.String())
	}
	active := ""
	if store.Active != nil {
		active = store.Active.String()
	}
	return names, active
}

// findProfile returns the profile with the given name; it reports false when there is none.
func findProfile(store models.ProfileStore, name string) (models.Profile, bool) {
	for _, profile := range store.Profiles {
		if profile.Name.String() == name {
			return profile, true
		}
	}
	return models.Profile{}, false
}

func firstText(first string, second string) string {
	if first != "" {
		return first
	}
	return second
}
