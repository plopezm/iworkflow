package components

import (
	"strconv"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// Texts of the file card, copied from the Rust card (src/modules/tasks/presentation/components/diff_file_card.rs).
var (
	msgBinaryFile       = i18n.Msg{En: "Binary file: content is not shown.", Es: "Fichero binario: no se muestra el contenido.", Zh: "二进制文件：不显示内容。", De: "Binärdatei: Der Inhalt wird nicht angezeigt.", Fr: "Fichier binaire : le contenu n’est pas affiché."}
	msgNoContentChanges = i18n.Msg{En: "No content changes.", Es: "Sin cambios de contenido.", Zh: "内容没有变化。", De: "Keine Inhaltsänderungen.", Fr: "Aucune modification de contenu."}
	msgBefore           = i18n.Msg{En: "Before", Es: "Antes", Zh: "之前", De: "Vorher", Fr: "Avant"}
	msgAfter            = i18n.Msg{En: "After", Es: "Después", Zh: "之后", De: "Nachher", Fr: "Après"}
)

// fileTitle is the title of a card: `old → new` for a rename, the path otherwise.
func fileTitle(file state.FileDiff) string {
	if file.OldPath != "" {
		return file.OldPath + " → " + file.Path
	}
	return file.Path
}

// cellClass is the CSS class of a split cell. A missing cell is the blank side of an addition or a removal.
func cellClass(cell *state.Cell) string {
	if cell == nil {
		return "diff-cell diff-cell-empty"
	}
	return kindClass(cell.Kind)
}

// kindClass is the CSS class of a cell or a unified line of a kind.
func kindClass(kind state.CellKind) string {
	switch kind {
	case state.CellAdded:
		return "diff-cell diff-cell-add"
	case state.CellRemoved:
		return "diff-cell diff-cell-del"
	default:
		return "diff-cell"
	}
}

// kindSign is the sign shown before a unified line.
func kindSign(kind state.CellKind) string {
	switch kind {
	case state.CellAdded:
		return "+"
	case state.CellRemoved:
		return "-"
	default:
		return " "
	}
}

// cellText is the text of a split cell; a missing cell is blank.
func cellText(cell *state.Cell) string {
	if cell == nil {
		return ""
	}
	return cell.Text
}

// cellNumber is the line number of a split cell; a missing cell has none.
func cellNumber(cell *state.Cell) string {
	if cell == nil {
		return ""
	}
	return strconv.Itoa(cell.Number)
}

// lineNumber is the number of a unified line on one side; zero means the line does not exist on that side.
func lineNumber(number int) string {
	if number == 0 {
		return ""
	}
	return strconv.Itoa(number)
}
