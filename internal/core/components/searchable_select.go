package components

import (
	"context"

	"iworkflow/internal/core/i18n"
)

// User-facing texts of the searchable select (web/static/js/searchable_select.js): the placeholder of the filter input
// and the line shown when no option matches. The script reads them from data attributes of the select.
var (
	msgSearchablePlaceholder = i18n.Msg{En: "Filter…", Es: "Filtrar…", Zh: "筛选…", De: "Filtern…", Fr: "Filtrer…"}
	msgSearchableEmpty       = i18n.Msg{En: "No matches", Es: "Sin coincidencias", Zh: "没有匹配项", De: "Keine Treffer", Fr: "Aucun résultat"}
)

// SearchablePlaceholder is the placeholder of the filter input of a searchable select.
func SearchablePlaceholder(ctx context.Context) string {
	return msgSearchablePlaceholder.T(ctx)
}

// SearchableEmpty is the line the dropdown of a searchable select shows when the filter matches no option.
func SearchableEmpty(ctx context.Context) string {
	return msgSearchableEmpty.T(ctx)
}
