use leptos::prelude::*;

use crate::core::i18n::Msg;

use crate::core::components::layout::ErrorText;
use crate::core::components::progress_dialog::ProgressDialog;
use crate::core::dock::js as dock_js;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_diff_dto::TaskDiffDto;
use crate::modules::tasks::presentation::components::diff_file_card::DiffFileCard;
use crate::modules::tasks::presentation::components::diff_file_tree::DiffFileTree;
use crate::modules::tasks::presentation::components::diff_report::DiffReport;
use crate::modules::tasks::presentation::state::diff_files::FileDiff;
use crate::modules::tasks::presentation::state::diff_report_state::{DiffReportState, ReportPhase};
use crate::modules::tasks::presentation::state::diff_view_state::{
    DiffViewState, ViewMode, can_edit_file,
};

const PROGRESS_TITLE: Msg = Msg {
    en: "Generating PDF",
    es: "Generando PDF",
    zh: "正在生成 PDF",
    de: "PDF wird erstellt",
    fr: "Génération du PDF",
};

const WAITING_MESSAGE: Msg = Msg {
    en: "The agent is writing the explanation of the changes. The PDF will open when it finishes; if you cancel, the agent will keep working on its reply in the background.",
    es: "El agente está redactando la explicación de los cambios. El PDF se abrirá al terminar; si cancelas, el agente seguirá con su respuesta en segundo plano.",
    zh: "智能体正在撰写更改说明。完成后将打开 PDF；如果取消，智能体会在后台继续完成其回复。",
    de: "Der Agent verfasst die Erklärung der Änderungen. Das PDF wird nach Abschluss geöffnet; wenn du abbrichst, arbeitet der Agent im Hintergrund an seiner Antwort weiter.",
    fr: "L’agent rédige l’explication des modifications. Le PDF s’ouvrira à la fin ; si vous annulez, l’agent poursuivra sa réponse en arrière-plan.",
};

const PREPARING_MESSAGE: Msg = Msg {
    en: "Preparing the report with the changes. It may take a while if the diff is large.",
    es: "Preparando el informe con los cambios. Puede tardar si el diff es grande.",
    zh: "正在准备包含更改的报告。如果 diff 很大，可能需要一些时间。",
    de: "Der Bericht mit den Änderungen wird vorbereitet. Bei einem großen Diff kann es etwas dauern.",
    fr: "Préparation du rapport des modifications. Cela peut prendre du temps si le diff est volumineux.",
};

const NO_CHANGES_VS_BASE: Msg = Msg {
    en: "No changes compared to the base branch.",
    es: "Sin cambios respecto a la rama base.",
    zh: "与基础分支相比没有更改。",
    de: "Keine Änderungen gegenüber dem Basis-Branch.",
    fr: "Aucune modification par rapport à la branche de base.",
};

const VIEW_MODE: Msg = Msg {
    en: "View mode",
    es: "Modo de vista",
    zh: "视图模式",
    de: "Ansichtsmodus",
    fr: "Mode d’affichage",
};

const EXPAND_ALL: Msg = Msg {
    en: "Expand all",
    es: "Expandir todo",
    zh: "全部展开",
    de: "Alle ausklappen",
    fr: "Tout développer",
};

const COLLAPSE_ALL: Msg = Msg {
    en: "Collapse all",
    es: "Contraer todo",
    zh: "全部折叠",
    de: "Alle einklappen",
    fr: "Tout réduire",
};

const TREE: Msg = Msg {
    en: "Tree",
    es: "Árbol",
    zh: "树",
    de: "Baum",
    fr: "Arborescence",
};

const EXPORT_PDF: Msg = Msg {
    en: "Export PDF",
    es: "Exportar PDF",
    zh: "导出 PDF",
    de: "PDF exportieren",
    fr: "Exporter en PDF",
};

const PDF_WITH_EXPLANATIONS: Msg = Msg {
    en: "PDF with explanations",
    es: "PDF con explicaciones",
    zh: "带说明的 PDF",
    de: "PDF mit Erklärungen",
    fr: "PDF avec explications",
};

const EXPLANATION_FAILED: Msg = Msg {
    en: "Could not request the explanation from the agent: {error}",
    es: "No se pudo pedir la explicación al agente: {error}",
    zh: "无法向智能体请求说明：{error}",
    de: "Die Erklärung konnte nicht beim Agent angefordert werden: {error}",
    fr: "Impossible de demander l’explication à l’agent : {error}",
};

const RETRY: Msg = Msg {
    en: "Retry",
    es: "Reintentar",
    zh: "重试",
    de: "Erneut versuchen",
    fr: "Réessayer",
};

const CLOSE: Msg = Msg {
    en: "Close",
    es: "Cerrar",
    zh: "关闭",
    de: "Schließen",
    fr: "Fermer",
};

const COMMITS_AND_SUMMARY: Msg = Msg {
    en: "Commits ({n}) and summary",
    es: "Commits ({n}) y resumen",
    zh: "提交（{n}）和摘要",
    de: "Commits ({n}) und Zusammenfassung",
    fr: "Commits ({n}) et résumé",
};

const TRUNCATED: Msg = Msg {
    en: "Patch truncated: review the rest with git.",
    es: "Parche truncado: revisa el resto con git.",
    zh: "补丁已截断：请使用 git 查看其余部分。",
    de: "Patch gekürzt: Den Rest mit git prüfen.",
    fr: "Patch tronqué : consultez le reste avec git.",
};

const NO_FILE_CHANGES: Msg = Msg {
    en: "No file changes.",
    es: "Sin cambios de ficheros.",
    zh: "文件没有更改。",
    de: "Keine Dateiänderungen.",
    fr: "Aucune modification de fichier.",
};

/// Clase CSS de una línea de un parche unificado.
pub fn line_class(line: &str) -> &'static str {
    if line.starts_with("diff --git") {
        "diff-file"
    } else if line.starts_with("+++") || line.starts_with("---") {
        "diff-meta"
    } else if line.starts_with("@@") {
        "diff-hunk"
    } else if line.starts_with('+') {
        "diff-add"
    } else if line.starts_with('-') {
        "diff-del"
    } else {
        "diff-ctx"
    }
}

/// One option of the Split/Unified toggle.
#[component]
fn ViewModeButton(state: DiffViewState, mode: ViewMode) -> impl IntoView {
    view! {
        <button
            type="button"
            class="diff-segment"
            class:diff-segment-active=move || state.view_mode.get() == mode
            aria-pressed=move || (state.view_mode.get() == mode).to_string()
            on:click=move |_| state.view_mode.set(mode)
        >
            {mode.label()}
        </button>
    }
}

#[component]
pub fn DiffView(
    diff: TaskDiffDto,
    #[prop(into)] title: String,
    report: DiffReportState,
    state: DiffViewState,
    #[prop(into)] can_edit_files: Signal<bool>,
) -> impl IntoView {
    if diff.patch.is_empty() && diff.commits.is_empty() {
        return view! { <p class="muted">{NO_CHANGES_VS_BASE.get()}</p> }.into_any();
    }
    let report_diff: TaskDiffDto = diff.clone();
    let commit_count: usize = diff.commits.len();
    let commits: String = diff.commits.join("\n");
    let stat: String = diff.stat.clone();
    let truncated: bool = diff.truncated;
    let on_close: Callback<()> = Callback::new(move |_| report.close());
    let busy: Signal<bool> = Signal::derive(move || report.phase.with(ReportPhase::is_busy));
    let progress_message: Signal<String> = Signal::derive(move || {
        if report.phase.with(ReportPhase::is_waiting) {
            WAITING_MESSAGE.get().to_string()
        } else {
            PREPARING_MESSAGE.get().to_string()
        }
    });
    view! {
        <div class="stack @container">
            <ProgressDialog
                title=Signal::derive(|| PROGRESS_TITLE.get().to_string())
                message=progress_message
                open=busy
                on_cancel=on_close
            />
            <div class="diff-toolbar">
                <div class="diff-segmented" role="group" aria-label=VIEW_MODE.get()>
                    <ViewModeButton state mode=ViewMode::Split />
                    <ViewModeButton state mode=ViewMode::Unified />
                </div>
                <button type="button" class="btn-secondary" on:click=move |_| state.expand_all()>
                    {EXPAND_ALL.get()}
                </button>
                <button type="button" class="btn-secondary" on:click=move |_| state.collapse_all()>
                    {COLLAPSE_ALL.get()}
                </button>
                <button
                    type="button"
                    class="btn-secondary"
                    aria-pressed=move || state.tree_open.get().to_string()
                    on:click=move |_| state.tree_open.update(|open: &mut bool| *open = !*open)
                >
                    {TREE.get()}
                </button>
                <button type="button" class="btn-secondary" on:click=move |_| report.open_plain()>
                    {EXPORT_PDF.get()}
                </button>
                <button
                    type="button"
                    class="btn-secondary"
                    on:click=move |_| report.request_explanation()
                >
                    {PDF_WITH_EXPLANATIONS.get()}
                </button>
            </div>
            {move || match report.phase.get() {
                ReportPhase::Idle => ().into_any(),
                ReportPhase::Waiting | ReportPhase::Preparing => ().into_any(),
                ReportPhase::Failed(error) => {
                    view! {
                        <div class="notice animate-fade-in flex items-center justify-between gap-2">
                            <ErrorText message=EXPLANATION_FAILED.fmt(&[("error", &error)]) />
                            <span class="flex gap-2">
                                <button
                                    type="button"
                                    class="btn-secondary"
                                    on:click=move |_| report.request_explanation()
                                >
                                    {RETRY.get()}
                                </button>
                                <button type="button" class="btn-ghost" on:click=move |_| report.close()>
                                    {CLOSE.get()}
                                </button>
                            </span>
                        </div>
                    }
                        .into_any()
                }
                ReportPhase::Plain => {
                    view! {
                        <DiffReport title=title.clone() diff=report_diff.clone() on_close />
                    }
                        .into_any()
                }
                ReportPhase::Ready(text) => {
                    view! {
                        <DiffReport
                            title=title.clone()
                            diff=report_diff.clone()
                            explanation=text
                            on_close
                        />
                    }
                        .into_any()
                }
            }}
            <div class="diff-summary">
                <button
                    type="button"
                    class="diff-summary-toggle"
                    aria-expanded=move || state.summary_open.get().to_string()
                    on:click=move |_| state.summary_open.update(|open: &mut bool| *open = !*open)
                >
                    <span aria-hidden="true">{move || if state.summary_open.get() { "▾" } else { "▸" }}</span>
                    {COMMITS_AND_SUMMARY.fmt(&[("n", &commit_count.to_string())])}
                </button>
                <Show when=move || state.summary_open.get()>
                    <div class="stack p-3">
                        <pre class="rounded-lg bg-surface-2/40 p-3 text-sm">{commits.clone()}</pre>
                        <pre class="rounded-lg bg-surface-2/40 p-3 text-sm">{stat.clone()}</pre>
                    </div>
                </Show>
            </div>
            {truncated
                .then(|| view! { <p class="notice notice-warn animate-fade-in">{TRUNCATED.get()}</p> })}
            <div class="diff-layout">
                <Show when=move || state.tree_open.get() && !state.files.with(Vec::is_empty)>
                    <aside
                        class="diff-sidebar"
                        style=move || {
                            state
                                .sidebar_ratio
                                .get()
                                .map(|ratio: f64| format!("flex: 0 0 {}%", ratio * 100.0))
                        }
                    >
                        <DiffFileTree state />
                    </aside>
                    <div
                        class="diff-resizer"
                        data-dock-divider=""
                        role="separator"
                        aria-orientation="vertical"
                        on:pointerdown=move |ev: leptos::ev::PointerEvent| {
                            dock_js::start_resize(
                                ev,
                                false,
                                move |ratio: f64| state.sidebar_ratio.set(Some(ratio)),
                            );
                        }
                    ></div>
                </Show>
                <div class="diff-cards">
                    {move || {
                        let files: Vec<FileDiff> = state.files.get();
                        if files.is_empty() {
                            return view! { <p class="muted">{NO_FILE_CHANGES.get()}</p> }.into_any();
                        }
                        let editable_all: bool = can_edit_files.get();
                        files
                            .into_iter()
                            .map(|file: FileDiff| {
                                let editable: bool = can_edit_file(&file, editable_all);
                                view! { <DiffFileCard file state editable /> }
                            })
                            .collect_view()
                            .into_any()
                    }}
                </div>
            </div>
        </div>
    }
    .into_any()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn classifies_patch_lines() {
        assert_eq!(line_class("diff --git a/x b/x"), "diff-file");
        assert_eq!(line_class("+++ b/x"), "diff-meta");
        assert_eq!(line_class("@@ -1 +1 @@"), "diff-hunk");
        assert_eq!(line_class("+nuevo"), "diff-add");
        assert_eq!(line_class("-viejo"), "diff-del");
        assert_eq!(line_class(" igual"), "diff-ctx");
    }
}
