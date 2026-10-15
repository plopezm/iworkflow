use leptos::prelude::*;

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

const PROGRESS_TITLE: &str = "Generando PDF";
const WAITING_MESSAGE: &str = "El agente está redactando la explicación de los cambios. \
El PDF se abrirá al terminar; si cancelas, el agente seguirá con su respuesta en segundo plano.";

const PREPARING_MESSAGE: &str =
    "Preparando el informe con los cambios. Puede tardar si el diff es grande.";

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
        return view! { <p class="muted">"Sin cambios respecto a la rama base."</p> }.into_any();
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
            WAITING_MESSAGE.to_string()
        } else {
            PREPARING_MESSAGE.to_string()
        }
    });
    view! {
        <div class="stack @container">
            <ProgressDialog
                title=Signal::derive(|| PROGRESS_TITLE.to_string())
                message=progress_message
                open=busy
                on_cancel=on_close
            />
            <div class="diff-toolbar">
                <div class="diff-segmented" role="group" aria-label="Modo de vista">
                    <ViewModeButton state mode=ViewMode::Split />
                    <ViewModeButton state mode=ViewMode::Unified />
                </div>
                <button type="button" class="btn-secondary" on:click=move |_| state.expand_all()>
                    "Expandir todo"
                </button>
                <button type="button" class="btn-secondary" on:click=move |_| state.collapse_all()>
                    "Contraer todo"
                </button>
                <button
                    type="button"
                    class="btn-secondary"
                    aria-pressed=move || state.tree_open.get().to_string()
                    on:click=move |_| state.tree_open.update(|open: &mut bool| *open = !*open)
                >
                    "Árbol"
                </button>
                <button type="button" class="btn-secondary" on:click=move |_| report.open_plain()>
                    "Exportar PDF"
                </button>
                <button
                    type="button"
                    class="btn-secondary"
                    on:click=move |_| report.request_explanation()
                >
                    "PDF con explicaciones"
                </button>
            </div>
            {move || match report.phase.get() {
                ReportPhase::Idle => ().into_any(),
                ReportPhase::Waiting | ReportPhase::Preparing => ().into_any(),
                ReportPhase::Failed(error) => {
                    view! {
                        <div class="notice animate-fade-in flex items-center justify-between gap-2">
                            <ErrorText message=format!(
                                "No se pudo pedir la explicación al agente: {error}",
                            ) />
                            <span class="flex gap-2">
                                <button
                                    type="button"
                                    class="btn-secondary"
                                    on:click=move |_| report.request_explanation()
                                >
                                    "Reintentar"
                                </button>
                                <button type="button" class="btn-ghost" on:click=move |_| report.close()>
                                    "Cerrar"
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
                    {format!("Commits ({}) y resumen", commit_count)}
                </button>
                <Show when=move || state.summary_open.get()>
                    <div class="stack p-3">
                        <pre class="rounded-lg bg-surface-2/40 p-3 text-sm">{commits.clone()}</pre>
                        <pre class="rounded-lg bg-surface-2/40 p-3 text-sm">{stat.clone()}</pre>
                    </div>
                </Show>
            </div>
            {truncated
                .then(|| view! { <p class="notice notice-warn animate-fade-in">"Parche truncado: revisa el resto con git."</p> })}
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
                            return view! { <p class="muted">"Sin cambios de ficheros."</p> }.into_any();
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
