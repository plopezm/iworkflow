package application_test

import (
	"context"
	"sort"

	gerrors "iworkflow/internal/modules/workspace_groups/domain/errors"
	"iworkflow/internal/modules/workspace_groups/domain/models"
)

// fakeRepo is an in-memory GroupRepository. workspaces holds the ids that "exist".
type fakeRepo struct {
	groups     []models.Group
	members    map[string]string
	pins       []string
	workspaces map[string]bool
	failWith   error
}

func newFakeRepo(workspaces ...string) *fakeRepo {
	known := map[string]bool{}
	for _, id := range workspaces {
		known[id] = true
	}
	return &fakeRepo{members: map[string]string{}, workspaces: known}
}

func (f *fakeRepo) index(id string) int {
	for i, group := range f.groups {
		if group.ID() == id {
			return i
		}
	}
	return -1
}

func (f *fakeRepo) ListGroups(context.Context) ([]models.Group, error) {
	return append([]models.Group(nil), f.groups...), f.failWith
}

func (f *fakeRepo) SaveGroup(_ context.Context, group models.Group) error {
	f.groups = append(f.groups, group)
	return f.failWith
}

func (f *fakeRepo) UpdateGroup(_ context.Context, group models.Group) error {
	i := f.index(group.ID())
	if i < 0 {
		return gerrors.ErrGroupNotFound
	}
	f.groups[i] = group
	return f.failWith
}

func (f *fakeRepo) DeleteGroup(_ context.Context, id string) error {
	i := f.index(id)
	if i < 0 {
		return gerrors.ErrGroupNotFound
	}
	f.groups = append(f.groups[:i], f.groups[i+1:]...)
	for workspace, group := range f.members {
		if group == id {
			delete(f.members, workspace)
		}
	}
	return f.failWith
}

func (f *fakeRepo) SetCollapsed(_ context.Context, id string, collapsed bool) error {
	i := f.index(id)
	if i < 0 {
		return gerrors.ErrGroupNotFound
	}
	group := f.groups[i]
	f.groups[i], _ = models.NewGroup(group.ID(), group.Name(), group.Color(), collapsed)
	return f.failWith
}

func (f *fakeRepo) Memberships(context.Context) (map[string]string, error) {
	copied := map[string]string{}
	for k, v := range f.members {
		copied[k] = v
	}
	return copied, f.failWith
}

func (f *fakeRepo) Assign(_ context.Context, workspaceID string, groupID string) error {
	if !f.workspaces[workspaceID] {
		return gerrors.ErrWorkspaceNotFound
	}
	if f.index(groupID) < 0 {
		return gerrors.ErrGroupNotFound
	}
	f.members[workspaceID] = groupID
	return f.failWith
}

func (f *fakeRepo) Unassign(_ context.Context, workspaceID string) error {
	delete(f.members, workspaceID)
	return f.failWith
}

func (f *fakeRepo) Pinned(context.Context) ([]string, error) {
	return append([]string(nil), f.pins...), f.failWith
}

func (f *fakeRepo) Pin(_ context.Context, workspaceID string) error {
	if !f.workspaces[workspaceID] {
		return gerrors.ErrWorkspaceNotFound
	}
	for _, id := range f.pins {
		if id == workspaceID {
			return nil
		}
	}
	f.pins = append(f.pins, workspaceID)
	return f.failWith
}

func (f *fakeRepo) Unpin(_ context.Context, workspaceID string) error {
	kept := f.pins[:0]
	for _, id := range f.pins {
		if id != workspaceID {
			kept = append(kept, id)
		}
	}
	f.pins = kept
	return f.failWith
}

func (f *fakeRepo) groupIDs() []string {
	ids := make([]string, 0, len(f.groups))
	for _, group := range f.groups {
		ids = append(ids, group.ID())
	}
	sort.Strings(ids)
	return ids
}

// sequenceIDs hands out g1, g2, ...
type sequenceIDs struct{ next int }

func (s *sequenceIDs) NewID() string {
	s.next++
	return "g" + string(rune('0'+s.next))
}
