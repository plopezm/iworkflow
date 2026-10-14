use leptos::prelude::*;
use crate::core::dock::link::TabLink;

use crate::core::browser::confirm;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
    TaskActionsDto, TaskDto, TaskPriorityDto, TaskStageDto,
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
    let show_prompt: bool = prompt_adds_info(&task.title, &task.prompt);
    let article_class: String = format!(
        "card {} animate-fade-in stack gap-2.5{}",
        stage_rail_class(task.stage),
        if task.needs_attention {
            " animate-pulse-glow"
        } else {
            ""
        }
    );

    view! {
        <article class=article_class>
            <div class="row justify-between gap-2">
                <TabLink
                    href=format!("/tasks/{id}")
                    class="line-clamp-2 font-medium text-fg hover:text-accent transition-colors"
                >
                    {title}
                </TabLink>
                <div class="row shrink-0 gap-1.5">
                    <StageBadge stage=task.stage />
                    {(task.pending_dialogs > 0)
                        .then(|| {
                            view! {
                                <span class="badge badge-muted">
                                    {pending_dialogs_label(task.pending_dialogs)}
                                </span>
                            }
                        })}
                </div>
            </div>
            {(task.priority != TaskPriorityDto::Normal)
                .then(|| {
                    view! {
                        <div class="row gap-1.5">
                            <span class=format!(
                                "badge badge-{}",
                                task.priority.variant(),
                            )>{task.priority.label()}</span>
                        </div>
                    }
                })}
            {show_prompt.then(|| view! { <p class="muted line-clamp-2">{task.prompt.clone()}</p> })}
            {(!is_backlog)
                .then(|| {
                    view! {
                        <code class="truncate rounded-md bg-surface-2/60 px-1.5 py-0.5 font-mono text-[10px] text-muted">
                            {task.branch.clone()}
                        </code>
                    }
                })}
            {task.error.clone().map(|e: String| view! { <p class="text-xs text-error">{e}</p> })}
            <div class="row justify-between gap-2 pt-1">
                <span class="text-[11px] text-muted">{short_timestamp(&task.updated_at)}</span>
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
                                                class="btn-primary min-h-9 px-3 py-1.5 text-xs"
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
                                                class="btn-ghost min-h-9 px-3 py-1.5 text-xs"
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
                                                class="btn-ghost min-h-9 px-3 py-1.5 text-xs hover:bg-error/10 hover:text-error"
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
                                <TabLink
                                    href=format!("/tasks/{id}")
                                    class="btn-ghost min-h-9 px-3 py-1.5 text-xs"
                                >
                                    "Abrir"
                                </TabLink>
                                {task
                                    .actions
                                    .can_finish
                                    .then(|| {
                                        view! {
                                            <button
                                                class="btn-primary min-h-9 px-3 py-1.5 text-xs"
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
                                                class="btn-primary min-h-9 px-3 py-1.5 text-xs"
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

/// Left-rail classes (full literals so Tailwind can see them) colored by stage.
fn stage_rail_class(stage: TaskStageDto) -> &'static str {
    match stage {
        TaskStageDto::Backlog => "border-l-[3px] border-l-stage-backlog",
        TaskStageDto::Working => "border-l-[3px] border-l-stage-working",
        TaskStageDto::Attention => "border-l-[3px] border-l-stage-attention",
        TaskStageDto::Review => "border-l-[3px] border-l-stage-review",
        TaskStageDto::Done => "border-l-[3px] border-l-stage-done",
    }
}

/// Whether the prompt adds information beyond the title (titles are often the prompt
/// truncated with a trailing '…').
pub fn prompt_adds_info(title: &str, prompt: &str) -> bool {
    let prompt: &str = prompt.trim();
    if prompt.is_empty() {
        return false;
    }
    let stem: &str = title.trim().trim_end_matches('…').trim_end();
    stem.is_empty() || !prompt.starts_with(stem)
}

/// Etiqueta con singular/plural del número de diálogos pendientes.
fn pending_dialogs_label(count: usize) -> String {
    if count == 1 {
        "1 diálogo".to_string()
    } else {
        format!("{count} diálogos")
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

    #[test]
    fn prompt_hidden_when_it_repeats_the_title() {
        assert!(!prompt_adds_info("Arreglar login", "Arreglar login"));
        assert!(!prompt_adds_info(
            "Arreglar el log…",
            " Arreglar el login del panel "
        ));
        assert!(!prompt_adds_info("t", "  "));
    }

    #[test]
    fn prompt_shown_when_it_differs_from_the_title() {
        assert!(prompt_adds_info("Arreglar login", "Revisar el flujo OAuth"));
        assert!(prompt_adds_info("", "algo"));
    }

    #[test]
    fn stage_rail_is_colored_per_stage() {
        assert!(stage_rail_class(TaskStageDto::Working).ends_with("stage-working"));
        assert!(stage_rail_class(TaskStageDto::Done).ends_with("stage-done"));
    }

    #[test]
    fn pending_dialogs_label_pluralises() {
        assert_eq!(pending_dialogs_label(1), "1 diálogo");
        assert_eq!(pending_dialogs_label(3), "3 diálogos");
    }
}
