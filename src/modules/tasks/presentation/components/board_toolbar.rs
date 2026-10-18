use leptos::prelude::*;

use crate::core::i18n::Msg;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::WorkspaceOptionDto;
use crate::modules::tasks::presentation::components::workspace_switcher::WorkspaceSwitcher;

const BOARD: Msg = Msg {
    en: "Board",
    es: "Tablero",
    zh: "看板",
    de: "Board",
    fr: "Tableau",
};
const NEW_TASK: Msg = Msg {
    en: "New task",
    es: "Nueva tarea",
    zh: "新建任务",
    de: "Neue Aufgabe",
    fr: "Nouvelle tâche",
};
const SHOW_DONE: Msg = Msg {
    en: "Show completed",
    es: "Mostrar terminadas",
    zh: "显示已完成",
    de: "Erledigte anzeigen",
    fr: "Afficher les terminées",
};
const FIT_VIEW: Msg = Msg {
    en: "Fit view",
    es: "Ajustar vista",
    zh: "适应视图",
    de: "Ansicht einpassen",
    fr: "Ajuster la vue",
};

/// Barra superior del tablero: selector de workspace, ajustar vista, terminadas y alta de tarea.
#[component]
pub fn BoardToolbar(
    #[prop(into)] workspaces: Signal<Vec<WorkspaceOptionDto>>,
    #[prop(into)] selected_workspace: Signal<Option<String>>,
    show_completed: RwSignal<bool>,
    on_select_workspace: Callback<String>,
    on_fit: Callback<()>,
    on_new_task: Callback<()>,
) -> impl IntoView {
    view! {
        <div class="flex shrink-0 flex-wrap items-center gap-x-4 gap-y-2 border-b border-border bg-surface px-4 py-2.5">
            <h1 class="page-title">{BOARD.get()}</h1>
            <div class="min-w-0 flex-1">
                <WorkspaceSwitcher
                    workspaces
                    selected=selected_workspace
                    on_select=on_select_workspace
                />
            </div>
            <button
                type="button"
                class="btn-secondary"
                class=("border-accent", move || show_completed.get())
                class=("text-accent", move || show_completed.get())
                aria-pressed=move || show_completed.get().to_string()
                title=move || SHOW_DONE.get()
                aria-label=move || SHOW_DONE.get()
                on:click=move |_| show_completed.update(|value: &mut bool| *value = !*value)
            >
                <span aria-hidden="true">"✓"</span>
                <span class="hidden lg:inline">{SHOW_DONE.get()}</span>
            </button>
            <button type="button" class="btn-secondary" on:click=move |_| on_fit.run(())>
                <span aria-hidden="true">"⤢"</span>
                <span class="hidden sm:inline">{FIT_VIEW.get()}</span>
            </button>
            <button type="button" class="btn-primary" on:click=move |_| on_new_task.run(())>
                <span class="text-lg leading-none">+</span>
                <span class="hidden sm:inline">{NEW_TASK.get()}</span>
            </button>
        </div>
    }
}
