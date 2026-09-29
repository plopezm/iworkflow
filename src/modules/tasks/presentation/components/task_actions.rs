use leptos::prelude::*;

use crate::core::browser::confirm;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::TaskActionsDto;

/// Callbacks de las acciones sobre una tarea.
#[derive(Clone, Copy)]
pub struct TaskActionHandlers {
    pub abort: Callback<()>,
    pub finish: Callback<()>,
    pub resume: Callback<()>,
    pub merge: Callback<()>,
    pub discard: Callback<()>,
    pub toggle_diff: Callback<()>,
}

/// Ejecuta `action` solo si el usuario confirma `message`.
fn confirmed(message: &'static str, action: Callback<()>) -> impl Fn(leptos::ev::MouseEvent) {
    move |_| {
        if confirm(message) {
            action.run(());
        }
    }
}

/// Botonera contextual según las acciones que permite el estado de la tarea.
#[component]
pub fn TaskActions(
    actions: TaskActionsDto,
    handlers: TaskActionHandlers,
    #[prop(into)] diff_visible: Signal<bool>,
    #[prop(into)] busy: Signal<bool>,
) -> impl IntoView {
    view! {
        <div class="row">
            {actions
                .can_abort
                .then(|| {
                    view! {
                        <button class="btn-secondary" disabled=move || busy.get() on:click=move |_| handlers.abort.run(())>
                            "Abortar ejecución"
                        </button>
                    }
                })}
            {actions
                .can_finish
                .then(|| {
                    view! {
                        <button
                            class="btn-primary"
                            disabled=move || busy.get()
                            on:click=confirmed(
                                "¿Finalizar la tarea? Se cerrará el agente y se consolidarán los cambios para revisión.",
                                handlers.finish,
                            )
                        >
                            "Finalizar y revisar"
                        </button>
                    }
                })}
            {actions
                .can_resume
                .then(|| {
                    view! {
                        <button class="btn-secondary" disabled=move || busy.get() on:click=move |_| handlers.resume.run(())>
                            "Reanudar agente"
                        </button>
                    }
                })}
            {actions
                .can_view_diff
                .then(|| {
                    view! {
                        <button class="btn-secondary" on:click=move |_| handlers.toggle_diff.run(())>
                            {move || if diff_visible.get() { "Ocultar diff" } else { "Ver diff" }}
                        </button>
                    }
                })}
            {actions
                .can_merge
                .then(|| {
                    view! {
                        <button
                            class="btn-primary"
                            disabled=move || busy.get()
                            on:click=confirmed(
                                "¿Integrar la rama en la base? Se eliminará el worktree.",
                                handlers.merge,
                            )
                        >
                            "Merge"
                        </button>
                    }
                })}
            {actions
                .can_discard
                .then(|| {
                    view! {
                        <button
                            class="btn-danger"
                            disabled=move || busy.get()
                            on:click=confirmed(
                                "¿Descartar la tarea? Se borrarán el worktree y la rama sin integrar.",
                                handlers.discard,
                            )
                        >
                            "Descartar"
                        </button>
                    }
                })}
        </div>
    }
}
