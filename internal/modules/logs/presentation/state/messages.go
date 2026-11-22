// Package state builds the view-model of the log card from the data the handler reads.
package state

import "iworkflow/internal/core/i18n"

// User-facing texts of the log card. Each one is written in every supported language.
var (
	msgTitle = i18n.Msg{En: "Logs", Es: "Logs", Zh: "日志", De: "Protokolle", Fr: "Journaux"}
	msgHelp  = i18n.Msg{
		En: "The most recent lines of the iWork Flow log, newest first. It refreshes by itself.",
		Es: "Las últimas líneas del log de iWork Flow, la más reciente primero. Se actualiza solo.",
		Zh: "iWork Flow 日志的最新几行，最新的在前。会自动刷新。",
		De: "Die neuesten Zeilen des iWork-Flow-Protokolls, die neueste zuerst. Es aktualisiert sich selbst.",
		Fr: "Les dernières lignes du journal d'iWork Flow, la plus récente en premier. Il s'actualise tout seul.",
	}
	msgEmpty = i18n.Msg{
		En: "The log is empty.",
		Es: "El log está vacío.",
		Zh: "日志为空。",
		De: "Das Protokoll ist leer.",
		Fr: "Le journal est vide.",
	}
	msgTruncated = i18n.Msg{
		En: "Older lines are not shown.",
		Es: "No se muestran las líneas más antiguas.",
		Zh: "未显示更早的行。",
		De: "Ältere Zeilen werden nicht angezeigt.",
		Fr: "Les lignes plus anciennes ne sont pas affichées.",
	}
	msgReadError = i18n.Msg{
		En: "The log cannot be read.",
		Es: "No se puede leer el log.",
		Zh: "无法读取日志。",
		De: "Das Protokoll kann nicht gelesen werden.",
		Fr: "Le journal est illisible.",
	}
)
