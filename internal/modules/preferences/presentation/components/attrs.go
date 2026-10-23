package components

import (
	"encoding/json"
	"strconv"

	"github.com/a-h/templ"

	"iworkflow/internal/modules/preferences/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/preferences/presentation/state"
)

// Routes of the network access card. The handlers mount these exact paths.
const (
	networkPath       = "/preferences/network"
	networkTargetPath = "/preferences/network/target"
	portPath          = "/preferences/port"
	requireTokenPath  = "/preferences/require-token"
	regeneratePath    = "/preferences/token/regenerate"
	restartPath       = "/preferences/restart"
	// cardID is the swap target: every action replaces the whole card.
	cardID = "network-access"
	// targetID is the swap target of the interface selector.
	targetID = "network-target"
	// restartPollInterval is how often a restarting card asks whether the server is back.
	restartPollInterval = "every 2s"
)

// postValues is the hx-vals JSON that sends one named form value.
func postValues(name string, value bool) string {
	encoded, err := json.Marshal(map[string]string{name: strconv.FormatBool(value)})
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

// networkRootAttrs makes a restarting card poll until the server answers again. Other cards need
// no polling.
func networkRootAttrs(view state.NetworkAccessView) templ.Attributes {
	if !view.Restarting {
		return nil
	}
	return templ.Attributes{
		"hx-get":     networkPath,
		"hx-trigger": restartPollInterval,
		"hx-swap":    "outerHTML",
	}
}

// actionAttrs are the htmx attributes of a control that replaces the whole card. The button is
// disabled while its request is in flight so a double click sends one request.
func actionAttrs(path string) templ.Attributes {
	return templ.Attributes{
		"hx-post":         path,
		"hx-target":       "#" + cardID,
		"hx-swap":         "outerHTML",
		"hx-disabled-elt": "this",
	}
}

// firstTarget returns the preferred interface, nil when none was detected.
func firstTarget(view state.NetworkAccessView) *dto.AccessTargetDTO {
	if len(view.Access.Targets) == 0 {
		return nil
	}
	first := view.Access.Targets[0]
	return &first
}
