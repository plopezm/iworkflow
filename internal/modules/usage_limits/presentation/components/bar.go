// Package components holds the templ components of the usage_limits module. They render the view model
// they receive; every text is already translated or is a brand or window identifier.
package components

// BarPath is the URL that renders the usage bar. The shell places UsageBarPlaceholder where the bar goes.
const BarPath = "/usage-limits/bar"

// BarID is the root of the usage bar; htmx swaps the whole root on every refresh.
const BarID = "usage-bar"

// windowWeek is the identifier of the weekly window (models.WindowWeek).
const windowWeek = "week"
