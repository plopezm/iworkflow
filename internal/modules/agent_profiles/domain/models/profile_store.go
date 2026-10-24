package models

// ProfileStore is the content of profiles.json: the active profile and the profile list.
type ProfileStore struct {
	Active   *ProfileName
	Profiles []Profile
}

// NewProfileStore builds a store. A nil profile list becomes an empty one.
func NewProfileStore(active *ProfileName, profiles []Profile) ProfileStore {
	if profiles == nil {
		profiles = []Profile{}
	}
	return ProfileStore{Active: active, Profiles: profiles}
}

// Find returns the profile with the given name, or nil when there is none. The pointer refers to
// the store's own slice, so changes through it are visible in the store.
func (s *ProfileStore) Find(name ProfileName) *Profile {
	for i := range s.Profiles {
		if s.Profiles[i].Name == name {
			return &s.Profiles[i]
		}
	}
	return nil
}

// Contains reports whether a profile with the given name is in the store.
func (s *ProfileStore) Contains(name ProfileName) bool {
	return s.Find(name) != nil
}
