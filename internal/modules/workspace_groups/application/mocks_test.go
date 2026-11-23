package application_test

import (
	"context"
	"sort"
	"strconv"

	gerrors "iworkflow/internal/modules/workspace_groups/domain/errors"
	"iworkflow/internal/modules/workspace_groups/domain/models"
)

// fakeRepo is an in-memory GroupRepository. workspaces holds the ids that "exist". When failWith is set every call
// fails with it before it reads or writes anything, so a failed call never changes the stored state.
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
	if f.failWith != nil {
		return nil, f.failWith
	}
	return append([]models.Group(nil), f.groups...), nil
}

func (f *fakeRepo) SaveGroup(_ context.Context, group models.Group) error {
	if f.failWith != nil {
		return f.failWith
	}
	f.groups = append(f.groups, group)
	return nil
}

func (f *fakeRepo) UpdateGroup(_ context.Context, group models.Group) error {
	if f.failWith != nil {
		return f.failWith
	}
	i := f.index(group.ID())
	if i < 0 {
		return gerrors.ErrGroupNotFound
	}
	f.groups[i] = group
	return nil
}

func (f *fakeRepo) DeleteGroup(_ context.Context, id string) error {
	if f.failWith != nil {
		return f.failWith
	}
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
	return nil
}

func (f *fakeRepo) SetCollapsed(_ context.Context, id string, collapsed bool) error {
	if f.failWith != nil {
		return f.failWith
	}
	i := f.index(id)
	if i < 0 {
		return gerrors.ErrGroupNotFound
	}
	group := f.groups[i]
	f.groups[i], _ = models.NewGroup(group.ID(), group.Name(), group.Color(), collapsed)
	return nil
}

func (f *fakeRepo) Memberships(context.Context) (map[string]string, error) {
	if f.failWith != nil {
		return nil, f.failWith
	}
	copied := map[string]string{}
	for k, v := range f.members {
		copied[k] = v
	}
	return copied, nil
}

func (f *fakeRepo) Assign(_ context.Context, workspaceID string, groupID string) error {
	if f.failWith != nil {
		return f.failWith
	}
	if !f.workspaces[workspaceID] {
		return gerrors.ErrWorkspaceNotFound
	}
	if f.index(groupID) < 0 {
		return gerrors.ErrGroupNotFound
	}
	f.members[workspaceID] = groupID
	return nil
}

func (f *fakeRepo) Unassign(_ context.Context, workspaceID string) error {
	if f.failWith != nil {
		return f.failWith
	}
	delete(f.members, workspaceID)
	return nil
}

func (f *fakeRepo) Pinned(context.Context) ([]string, error) {
	if f.failWith != nil {
		return nil, f.failWith
	}
	return append([]string(nil), f.pins...), nil
}

func (f *fakeRepo) Pin(_ context.Context, workspaceID string) error {
	if f.failWith != nil {
		return f.failWith
	}
	if !f.workspaces[workspaceID] {
		return gerrors.ErrWorkspaceNotFound
	}
	for _, id := range f.pins {
		if id == workspaceID {
			return nil
		}
	}
	f.pins = append(f.pins, workspaceID)
	return nil
}

func (f *fakeRepo) Unpin(_ context.Context, workspaceID string) error {
	if f.failWith != nil {
		return f.failWith
	}
	kept := f.pins[:0]
	for _, id := range f.pins {
		if id != workspaceID {
			kept = append(kept, id)
		}
	}
	f.pins = kept
	return nil
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
	return "g" + strconv.Itoa(s.next)
}
