package inbound

import "net/url"

// basePath is the prefix of the htmx fragment endpoints. It is kept apart from the dock tab paths
// (/profiles, /models), so a fragment request never opens a tab.
const basePath = "/agent-profiles"

// Dock tab paths of the two screens.
const (
	ProfilesPath = "/profiles"
	ModelsPath   = "/models"
)

// Swap targets of the screens. Ids carry the screen prefix, so two screens can be open together.
const (
	profilesDialogID = "profiles-dialog"
	profilesListID   = "profiles-list"
	modelsDialogID   = "models-dialog"
	modelsScope      = "models"
	profilesScope    = "profiles"
)

func selectURL() string { return basePath + "/select" }

func profileDialogURL(kind string) string { return basePath + "/dialogs/" + kind }

func nameDialogURL(kind string, source string, selected string) string {
	query := url.Values{"source": {source}, "selected": {selected}}
	return basePath + "/dialogs/" + kind + "?" + query.Encode()
}

func roleDialogURL(scope string) string {
	return basePath + "/dialogs/role?" + url.Values{"scope": {scope}}.Encode()
}

func nameHintURL() string { return basePath + "/name-hint" }
func roleHintURL() string { return basePath + "/role-hint" }
func rolesURL() string    { return basePath + "/roles" }
func importURL() string   { return basePath + "/profiles/import" }
func profilesURL() string { return basePath + "/profiles" }

func profileURL(name string) string       { return basePath + "/profiles/" + name }
func profileDetailURL(name string) string { return profileURL(name) + "/detail" }
func duplicateURL(name string) string     { return profileURL(name) + "/duplicate" }
func renameURL(name string) string        { return profileURL(name) + "/rename" }
func exportURL(name string) string        { return profileURL(name) + "/export" }
func exportNoticeURL(name string) string  { return profileURL(name) + "/export-notice" }
func jsonURL(name string) string          { return profileURL(name) + "/json" }

func profileRoutingURL(name string) string     { return profileURL(name) + "/routing" }
func profileRoutingRowsURL(name string) string { return profileRoutingURL(name) + "/rows" }

func modelsRoutingURL() string     { return basePath + "/models/routing" }
func modelsRoutingRowsURL() string { return modelsRoutingURL() + "/rows" }
