// Package dto maps the HTTP form of the branch screen to the values the use cases take.
package dto

import (
	"net/url"
	"strings"

	"iworkflow/internal/modules/workspace_branches/domain/models"
)

// DeleteForm is the body of a delete request: the references of the branches to delete and the filter the list shows.
type DeleteForm struct {
	// Refs are the references in the order sent, without blanks and duplicates.
	Refs []models.Ref
	// Query is the filter to keep in the refreshed list.
	Query string
}

// DeleteFormFrom reads the repeated "branch" values and the "q" filter. The values are not validated here: an
// invalid one becomes a reference the use case reports per branch.
func DeleteFormFrom(values url.Values) DeleteForm {
	form := DeleteForm{Query: strings.TrimSpace(values.Get("q"))}
	seen := map[string]bool{}
	for _, value := range values["branch"] {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		form.Refs = append(form.Refs, models.ParseRef(value))
	}
	return form
}
