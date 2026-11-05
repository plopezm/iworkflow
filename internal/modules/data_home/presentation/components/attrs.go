package components

import "github.com/a-h/templ"

// dataHomeRootAttrs makes a restarting card poll until the server answers again. Other cards need no
// polling.
func dataHomeRootAttrs(p DataHomeCardProps) templ.Attributes {
	if !p.Restarting {
		return nil
	}
	return templ.Attributes{
		"hx-get":     DataHomePath,
		"hx-trigger": restartPollInterval,
		"hx-swap":    "outerHTML",
	}
}
