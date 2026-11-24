package components

// Crumb is one step of a Breadcrumb. A crumb without Href is the current page.
type Crumb struct {
	Label string
	Href  string
}
