package components

import (
	"context"
	"strings"

	"github.com/a-h/templ"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// Texts of the printable diff report, copied from the Rust report (src/modules/tasks/presentation/components/diff_report.rs).
var (
	msgSaveAsPDF           = i18n.Msg{En: "Save as PDF", Es: "Guardar como PDF", Zh: "另存为 PDF", De: "Als PDF speichern", Fr: "Enregistrer en PDF"}
	msgReportClose         = i18n.Msg{En: "Close", Es: "Cerrar", Zh: "关闭", De: "Schließen", Fr: "Fermer"}
	msgReportCommits       = i18n.Msg{En: "Commits ({n})", Es: "Commits ({n})", Zh: "提交（{n}）", De: "Commits ({n})", Fr: "Commits ({n})"}
	msgReportSummary       = i18n.Msg{En: "Summary", Es: "Resumen", Zh: "摘要", De: "Zusammenfassung", Fr: "Résumé"}
	msgReportExplanation   = i18n.Msg{En: "Agent explanation", Es: "Explicación del agente", Zh: "智能体说明", De: "Erklärung des Agents", Fr: "Explication de l’agent"}
	msgReportChangesByFile = i18n.Msg{En: "Changes by file", Es: "Cambios por fichero", Zh: "按文件划分的更改", De: "Änderungen pro Datei", Fr: "Modifications par fichier"}
	msgReportProgressTitle = i18n.Msg{En: "Generating PDF", Es: "Generando PDF", Zh: "正在生成 PDF", De: "PDF wird erstellt", Fr: "Génération du PDF"}
	msgReportWaiting       = i18n.Msg{En: "The agent is writing the explanation of the changes. The PDF will open when it finishes; if you cancel, the agent will keep working on its reply in the background.", Es: "El agente está redactando la explicación de los cambios. El PDF se abrirá al terminar; si cancelas, el agente seguirá con su respuesta en segundo plano.", Zh: "智能体正在撰写更改说明。完成后将打开 PDF；如果取消，智能体会在后台继续完成其回复。", De: "Der Agent verfasst die Erklärung der Änderungen. Das PDF wird nach Abschluss geöffnet; wenn du abbrichst, arbeitet der Agent im Hintergrund an seiner Antwort weiter.", Fr: "L’agent rédige l’explication des modifications. Le PDF s’ouvrira à la fin ; si vous annulez, l’agent poursuivra sa réponse en arrière-plan."}
	msgReportPreparing     = i18n.Msg{En: "Preparing the report with the changes. It may take a while if the diff is large.", Es: "Preparando el informe con los cambios. Puede tardar si el diff es grande.", Zh: "正在准备包含更改的报告。如果 diff 很大，可能需要一些时间。", De: "Der Bericht mit den Änderungen wird vorbereitet. Bei einem großen Diff kann es etwas dauern.", Fr: "Préparation du rapport des modifications. Cela peut prendre du temps si le diff est volumineux."}
)

// lineClass is the CSS class of one line of a unified patch: the header of the file, the header of the hunk, the
// added, removed and context lines.
func lineClass(line string) string {
	switch {
	case strings.HasPrefix(line, "diff --git"):
		return "diff-file"
	case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
		return "diff-meta"
	case strings.HasPrefix(line, "@@"):
		return "diff-hunk"
	case strings.HasPrefix(line, "+"):
		return "diff-add"
	case strings.HasPrefix(line, "-"):
		return "diff-del"
	default:
		return "diff-ctx"
	}
}

// diffReportCancelAttrs returns the htmx attributes that close the report: they answer the diff view region that
// renders it, so the overlay of the report swaps back to the diff.
func diffReportCancelAttrs(taskID string, scope string) templ.Attributes {
	return templ.Attributes{
		"hx-get":    state.DiffReportPath(taskID, scope, "close"),
		"hx-target": "#" + state.DiffViewRegionID(scope),
		"hx-swap":   "outerHTML",
	}
}

// diffReportPollAttrs returns the htmx attributes that ask again for the report that waits for the agent. The answer
// swaps the diff view region, and with it the element that polls, so the polling stops with the wait.
func diffReportPollAttrs(taskID string, scope string, since int) templ.Attributes {
	return templ.Attributes{
		"hx-get":     state.DiffReportPollPath(taskID, scope, since),
		"hx-trigger": "every 2s",
		"hx-target":  "#" + state.DiffViewRegionID(scope),
		"hx-swap":    "outerHTML",
	}
}

// diffReportProgressTitle is the title of the progress dialog of the report.
func diffReportProgressTitle(ctx context.Context, phase state.ReportPhase) string {
	return msgReportProgressTitle.T(ctx)
}

// diffReportProgressMessage is the message of the progress dialog of the report: the agent is writing the
// explanation, or the content of the report is being prepared to be rendered.
func diffReportProgressMessage(ctx context.Context, phase state.ReportPhase) string {
	if phase.IsWaiting() {
		return msgReportWaiting.T(ctx)
	}
	return msgReportPreparing.T(ctx)
}
