use leptos::prelude::*;

use crate::core::components::layout::ErrorText;
use crate::core::dock::context::{DockContext, use_dock, use_tab_params};
use crate::core::i18n::Msg;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_diff_dto::TaskDiffDto;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::TaskDto;
use crate::modules::tasks::presentation::components::diff_view::DiffView;
use crate::modules::tasks::presentation::pages::task_detail_page::diff_path;
use crate::modules::tasks::presentation::state::diff_report_state::DiffReportState;
use crate::modules::tasks::presentation::state::diff_view_state::DiffViewState;
use crate::modules::tasks::presentation::state::task_detail_state::TaskDetailState;

const DIFF_TAB_TITLE: Msg = Msg {
    en: "Diff · {title}",
    es: "Diff · {title}",
    zh: "差异 · {title}",
    de: "Diff · {title}",
    fr: "Diff · {title}",
};

const CALCULATING_DIFF: Msg = Msg {
    en: "Computing diff…",
    es: "Calculando diff…",
    zh: "正在计算差异…",
    de: "Diff wird berechnet…",
    fr: "Calcul du diff…",
};

const CHANGES_REPORT: Msg = Msg {
    en: "Changes report",
    es: "Informe de cambios",
    zh: "变更报告",
    de: "Änderungsbericht",
    fr: "Rapport de modifications",
};

const NO_DIFF: Msg = Msg {
    en: "No diff available for this task.",
    es: "Sin diff disponible para esta tarea.",
    zh: "此任务没有可用的差异。",
    de: "Kein Diff für diese Aufgabe verfügbar.",
    fr: "Aucun diff disponible pour cette tâche.",
};

/// Title of the dock tab for the diff of a task titled `task_title` (pure).
pub fn diff_tab_title(task_title: &str) -> String {
    DIFF_TAB_TITLE.fmt(&[("title", task_title)])
}

/// Diff de una tarea en su propia pestaña, con todo el ancho del grupo. Se reconstruye por
/// cada id (el contenido de la pestaña se reutiliza si cambia la ruta).
#[component]
pub fn TaskDiffPage() -> impl IntoView {
    let params = use_tab_params();
    let task_id: Memo<String> =
        Memo::new(move |_| params.with(|p| p.get("id").unwrap_or_default()));

    view! { {move || view! { <TaskDiffContent task_id=task_id.get() /> }} }
}

#[component]
fn TaskDiffContent(task_id: String) -> impl IntoView {
    let state: TaskDetailState = TaskDetailState::new(task_id);
    // This tab only exists to show the diff, so it is always requested.
    state.show_diff.set(true);
    let report: DiffReportState = DiffReportState::new(state.task_id, state.transcript, state.send);
    let patch: Signal<String> = Signal::derive(move || {
        state
            .diff
            .get()
            .flatten()
            .and_then(|r: Result<TaskDiffDto, ServerFnError>| r.ok())
            .map(|d: TaskDiffDto| d.patch)
            .unwrap_or_default()
    });
    let diff_state: DiffViewState = DiffViewState::new(state.task_id, patch, state.diff_refresh);
    let can_edit_files: Signal<bool> = Signal::derive(move || {
        state
            .task
            .get()
            .and_then(|r: Result<TaskDto, ServerFnError>| r.ok())
            .is_some_and(|t: TaskDto| t.actions.can_edit_files)
    });

    let dock: DockContext = use_dock();
    let path: String = diff_path(&state.task_id.get_value());
    Effect::new(move |_| {
        if let Some(Ok(task)) = state.task.get() {
            dock.set_title(&path, &diff_tab_title(&task.title));
        }
    });

    view! {
        <div class="h-full min-h-0 overflow-y-auto p-3">
            <Transition fallback=|| view! { <p class="muted">{CALCULATING_DIFF.get()}</p> }>
                {move || Suspend::new(async move {
                    match state.diff.await {
                        Some(Ok(diff)) => {
                            let title: String = state
                                .task
                                .get()
                                .and_then(|r: Result<TaskDto, ServerFnError>| r.ok())
                                .map(|t: TaskDto| t.title)
                                .unwrap_or_else(|| CHANGES_REPORT.get().to_string());
                            view! { <DiffView diff title report state=diff_state can_edit_files /> }
                                .into_any()
                        }
                        Some(Err(e)) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                        None => {
                            view! { <p class="muted">{NO_DIFF.get()}</p> }
                                .into_any()
                        }
                    }
                })}
            </Transition>
        </div>
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::core::i18n::Locale;

    #[test]
    fn tab_title_names_the_task() {
        assert_eq!(diff_tab_title("Fix login"), "Diff · Fix login");
        assert_eq!(
            DIFF_TAB_TITLE
                .in_locale(Locale::Es)
                .replace("{title}", "Arreglar login"),
            "Diff · Arreglar login"
        );
    }

    #[test]
    fn diff_path_points_to_the_diff_tab_route() {
        assert_eq!(diff_path("abc"), "/tasks/abc/diff");
    }
}
