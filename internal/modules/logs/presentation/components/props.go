// Package components holds the templ components of the logs module. They render the props they
// receive and contain no logic: the state package builds the props, and every text is already
// translated there.
package components

// LogsID is the element id of the card; it stays in the page and only its content is morphed.
const LogsID = "logs-card"

// LogsPath is the URL that loads the content of the card. The card polls it.
const LogsPath = "/logs/card"

// refreshInterval is how often the card reloads the log (an htmx trigger expression). The e2e test
// end2end/go/logs.e2e.cjs waits up to 12 s for a refresh: keep the interval below that.
const refreshInterval = "every 5s"

// LogsCardProps configures the log card. Error replaces the log when it cannot be read. Lines are
// shown newest first.
type LogsCardProps struct {
	Title         string
	Help          string
	Error         string
	Empty         string
	TruncatedNote string
	Lines         []string
}
