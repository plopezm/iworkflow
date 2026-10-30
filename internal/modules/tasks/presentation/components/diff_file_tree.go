package components

import (
	"strconv"

	"iworkflow/internal/core/i18n"
)

// msgChangedFiles names the tree, copied from the Rust tree (src/modules/tasks/presentation/components/diff_file_tree.rs).
var msgChangedFiles = i18n.Msg{En: "Changed files", Es: "Ficheros modificados", Zh: "已修改的文件", De: "Geänderte Dateien", Fr: "Fichiers modifiés"}

// The indentation of a tree row, in rem: a base padding plus one step per depth level.
const (
	treeBasePaddingRem = 0.75
	treeIndentRem      = 0.875
)

// indentStyle is the left padding of a row at depth.
func indentStyle(depth int) string {
	padding := treeBasePaddingRem + treeIndentRem*float64(depth)
	return "padding-left: " + strconv.FormatFloat(padding, 'f', -1, 64) + "rem"
}
