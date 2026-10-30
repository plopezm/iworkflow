package models

// ExportedProfile is one profile inside its exchange envelope (`gentle-pi.agent_model_profile`).
type ExportedProfile struct {
	Name    ProfileName
	Routing map[string]RoutingEntry
}

// NewExportedProfile copies a profile into its exchange shape.
func NewExportedProfile(profile Profile) ExportedProfile {
	return ExportedProfile(profile)
}

// ToProfile converts the exchange shape back into a profile.
func (e ExportedProfile) ToProfile() Profile {
	return NewProfile(e.Name, e.Routing)
}
