package components

import (
	"strconv"
	"strings"

	"iworkflow/internal/core/i18n"
)

// Texts of the diff view, copied from the Rust view (src/modules/tasks/presentation/components/diff_view.rs). The
// texts of the PDF report are not here: they come with the report.
var (
	msgNoChangesVsBase   = i18n.Msg{En: "No changes compared to the base branch.", Es: "Sin cambios respecto a la rama base.", Zh: "与基础分支相比没有更改。", De: "Keine Änderungen gegenüber dem Basis-Branch.", Fr: "Aucune modification par rapport à la branche de base."}
	msgViewMode          = i18n.Msg{En: "View mode", Es: "Modo de vista", Zh: "视图模式", De: "Ansichtsmodus", Fr: "Mode d’affichage"}
	msgExpandAll         = i18n.Msg{En: "Expand all", Es: "Expandir todo", Zh: "全部展开", De: "Alle ausklappen", Fr: "Tout développer"}
	msgCollapseAll       = i18n.Msg{En: "Collapse all", Es: "Contraer todo", Zh: "全部折叠", De: "Alle einklappen", Fr: "Tout réduire"}
	msgTree              = i18n.Msg{En: "Tree", Es: "Árbol", Zh: "树", De: "Baum", Fr: "Arborescence"}
	msgCommitsAndSummary = i18n.Msg{En: "Commits ({n}) and summary", Es: "Commits ({n}) y resumen", Zh: "提交（{n}）和摘要", De: "Commits ({n}) und Zusammenfassung", Fr: "Commits ({n}) et résumé"}
	msgTruncated         = i18n.Msg{En: "Patch truncated: review the rest with git.", Es: "Parche truncado: revisa el resto con git.", Zh: "补丁已截断：请使用 git 查看其余部分。", De: "Patch gekürzt: Den Rest mit git prüfen.", Fr: "Patch tronqué : consultez le reste avec git."}
	msgNoFileChanges     = i18n.Msg{En: "No file changes.", Es: "Sin cambios de ficheros.", Zh: "文件没有更改。", De: "Keine Dateiänderungen.", Fr: "Aucune modification de fichier."}
)

// boolText is the value of a pressed or expanded attribute.
func boolText(value bool) string {
	return strconv.FormatBool(value)
}

// diffSegmentClass is the CSS class of one option of the layout toggle; the option in use is highlighted.
func diffSegmentClass(active bool) string {
	if active {
		return "diff-segment diff-segment-active"
	}
	return "diff-segment"
}

// countText is a count written as a number.
func countText(count int) string {
	return strconv.Itoa(count)
}

// joinLines joins the lines of a preformatted block.
func joinLines(lines []string) string {
	return strings.Join(lines, "\n")
}
