use leptos::prelude::*;
use leptos_router::components::A;

use crate::core::browser::confirm;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
    TaskActionsDto, TaskDto, TaskStageDto,
};
use crate::modules::tasks::presentation::components::stage_badge::StageBadge;

/// Fecha RFC 3339 recortada a `AAAA-MM-DD HH:MM` para mostrarla de forma compacta.
pub fn short_timestamp(rfc3339: &str) -> String {
    rfc3339.get(..16).unwrap_or(rfc3339).replace('T', " ")
}

/// Callbacks de las acciones que puede lanzar una tarjeta del tablero.
#[derive(Clone, Copy)]
pub struct TaskCardHandlers {
    pub start: Callback<String>,
    pub edit: Callback<TaskDto>,
    pub delete: Callback<String>,
    pub finish: Callback<String>,
    pub merge: Callback<String>,
}

/// Id de la tarea con la que está ocupada una acción concreta, junto a si sigue pendiente.
#[derive(Clone, Copy)]
pub struct TaskActionBusy {
    pub target: RwSignal<Option<String>>,
    pub pending: Signal<bool>,
}

impl TaskActionBusy {
    fn is_busy(&self, task_id: &str) -> bool {
        self.pending.get() && self.target.get().as_deref() == Some(task_id)
    }
}

/// Estado de ocupado de una tarjeta, independiente por acción para no bloquear el resto
/// del tablero mientras una sola tarea tiene una petición en curso.
#[derive(Clone, Copy)]
pub struct TaskCardBusy {
    pub start: TaskActionBusy,
    pub delete: TaskActionBusy,
    pub finish: TaskActionBusy,
    pub merge: TaskActionBusy,
}

/// Tarjeta de una tarea dentro de un carril del tablero.
#[component]
pub fn TaskCard(task: TaskDto, handlers: TaskCardHandlers, busy: TaskCardBusy) -> impl IntoView {
    let id: String = task.id.clone();
    let title: String = task.title.clone();
    let is_backlog: bool = task.stage == TaskStageDto::Backlog;
    let highlight: &'static str = if task.needs_attention {
        "card border-l-4 border-l-stage-attention ring-1 ring-stage-attention/30"
    } else {
        "card"
    };

    view! {
        <article class=format!("{highlight} stack gap-2")>
            <div class="row justify-between gap-2">
                <A href=format!("/tasks/{id}") attr:class="font-medium text-fg hover:text-accent line-clamp-1">
                    {title}
                </A>
                <StageBadge stage=task.stage />
            </div>
            {task
                .needs_attention
                .then(|| {
                    view! {
                        <div class="row gap-1.5">
                            <span class="badge badge-attention">"Necesita atención"</span>
                            {(task.pending_dialogs > 0)
                                .then(|| {
                                    view! {
                                        <span class="badge badge-muted">
                                            {format!("{} diálogos", task.pending_dialogs)}
                                        </span>
                                    }
                                })}
                        </div>
                    }
                })}
            <p class="muted line-clamp-2">{task.prompt.clone()}</p>
            {(!is_backlog)
                .then(|| {
                    view! { <code class="truncate font-mono text-xs text-muted">{task.branch.clone()}</code> }
                })}
            {task.error.clone().map(|e: String| view! { <p class="text-xs text-error">{e}</p> })}
            <div class="row justify-between gap-2">
                <span class="text-xs text-muted">{short_timestamp(&task.updated_at)}</span>
                <div class="row gap-1.5">
                    {if is_backlog {
                            let start_id: String = id.clone();
                            let delete_id: String = id.clone();
                            let edit_task: TaskDto = task.clone();
                            let actions: TaskActionsDto = task.actions;
                            let start_busy: Signal<bool> = {
                                let id: String = id.clone();
                                Signal::derive(move || busy.start.is_busy(&id))
                            };
                            let delete_busy: Signal<bool> = {
                                let id: String = id.clone();
                                Signal::derive(move || busy.delete.is_busy(&id))
                            };
                            view! {
                                {actions
                                    .can_start
                                    .then(|| {
                                        view! {
                                            <button
                                                class="btn-primary min-h-10 px-2.5 py-1 text-xs"
                                                disabled=move || start_busy.get()
                                                on:click=move |_| handlers.start.run(start_id.clone())
                                            >
                                                "Iniciar"
                                            </button>
                                        }
                                    })}
                                {actions
                                    .can_edit
                                    .then(|| {
                                        view! {
                                            <button
                                                class="btn-secondary min-h-10 px-2.5 py-1 text-xs"
                                                on:click=move |_| handlers.edit.run(edit_task.clone())
                                            >
                                                "Editar"
                                            </button>
                                        }
                                    })}
                                {actions
                                    .can_delete
                                    .then(|| {
                                        view! {
                                            <button
                                                class="btn-danger min-h-10 px-2.5 py-1 text-xs"
                                                disabled=move || delete_busy.get()
                                                on:click=move |_| {
                                                    if confirm("¿Borrar esta tarea de la lista de pendientes?") {
                                                        handlers.delete.run(delete_id.clone());
                                                    }
                                                }
                                            >
                                                "Borrar"
                                            </button>
                                        }
                                    })}
                            }
                                .into_any()
                        } else {
                            let finish_id: String = id.clone();
                            let merge_id: String = id.clone();
                            let finish_busy: Signal<bool> = {
                                let id: String = id.clone();
                                Signal::derive(move || busy.finish.is_busy(&id))
                            };
                            let merge_busy: Signal<bool> = {
                                let id: String = id.clone();
                                Signal::derive(move || busy.merge.is_busy(&id))
                            };
                            view! {
                                <A
                                    href=format!("/tasks/{id}")
                                    attr:class="btn-secondary min-h-10 px-2.5 py-1 text-xs"
                                >
                                    "Abrir"
                                </A>
                                {task
                                    .actions
                                    .can_finish
                                    .then(|| {
                                        view! {
                                            <button
                                                class="btn-primary min-h-10 px-2.5 py-1 text-xs"
                                                disabled=move || finish_busy.get()
                                                on:click=move |_| handlers.finish.run(finish_id.clone())
                                            >
                                                "Finalizar"
                                            </button>
                                        }
                                    })}
                                {task
                                    .actions
                                    .can_merge
                                    .then(|| {
                                        view! {
                                            <button
                                                class="btn-primary min-h-10 px-2.5 py-1 text-xs"
                                                disabled=move || merge_busy.get()
                                                on:click=move |_| handlers.merge.run(merge_id.clone())
                                            >
                                                "Integrar"
                                            </button>
                                        }
                                    })}
                            }
                                .into_any()
                        }}
                </div>
            </div>
        </article>
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn short_timestamp_keeps_minutes() {
        assert_eq!(
            short_timestamp("2026-09-28T16:39:36.638+00:00"),
            "2026-09-28 16:39"
        );
        assert_eq!(short_timestamp("corto"), "corto");
    }
}
