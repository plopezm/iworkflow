use crate::core::dock::link::TabLink;
use leptos::prelude::*;

use crate::core::i18n::Msg;

use crate::core::browser::confirm;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
    TaskActionsDto, TaskDto, TaskStageDto,
};

const START: Msg = Msg {
    en: "Start",
    es: "Iniciar",
    zh: "开始",
    de: "Starten",
    fr: "Démarrer",
};

const EDIT: Msg = Msg {
    en: "Edit",
    es: "Editar",
    zh: "编辑",
    de: "Bearbeiten",
    fr: "Modifier",
};

const DELETE: Msg = Msg {
    en: "Delete",
    es: "Borrar",
    zh: "删除",
    de: "Löschen",
    fr: "Supprimer",
};

const OPEN: Msg = Msg {
    en: "Open",
    es: "Abrir",
    zh: "打开",
    de: "Öffnen",
    fr: "Ouvrir",
};

const FINISH: Msg = Msg {
    en: "Finish",
    es: "Finalizar",
    zh: "完成",
    de: "Abschließen",
    fr: "Terminer",
};

const MERGE: Msg = Msg {
    en: "Merge",
    es: "Integrar",
    zh: "合并",
    de: "Integrieren",
    fr: "Intégrer",
};

const CONFIRM_DELETE: Msg = Msg {
    en: "Delete this task from the pending list?",
    es: "¿Borrar esta tarea de la lista de pendientes?",
    zh: "要从待办列表中删除此任务吗？",
    de: "Diese Aufgabe aus der Liste der offenen Aufgaben löschen?",
    fr: "Supprimer cette tâche de la liste des tâches en attente ?",
};

const PENDING_DIALOG_ONE: Msg = Msg {
    en: "1 dialog",
    es: "1 diálogo",
    zh: "1 个对话框",
    de: "1 Dialog",
    fr: "1 dialogue",
};

const PENDING_DIALOGS_MANY: Msg = Msg {
    en: "{count} dialogs",
    es: "{count} diálogos",
    zh: "{count} 个对话框",
    de: "{count} Dialoge",
    fr: "{count} dialogues",
};

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
    pub fn is_busy(&self, task_id: &str) -> bool {
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

/// Botones de acción de una tarea según su estado (iniciar/editar/borrar en `Backlog`;
/// abrir/finalizar/integrar en el resto). Las acciones disponibles las decide el servidor.
#[component]
pub fn TaskCardActions(
    task: TaskDto,
    handlers: TaskCardHandlers,
    busy: TaskCardBusy,
) -> impl IntoView {
    let id: String = task.id.clone();
    if task.stage == TaskStageDto::Backlog {
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
                            class="btn-primary min-h-8 px-2.5 py-1 text-xs"
                            disabled=move || start_busy.get()
                            on:click=move |_| handlers.start.run(start_id.clone())
                        >
                            {START.get()}
                        </button>
                    }
                })}
            {actions
                .can_edit
                .then(|| {
                    view! {
                        <button
                            class="btn-ghost min-h-8 px-2.5 py-1 text-xs"
                            on:click=move |_| handlers.edit.run(edit_task.clone())
                        >
                            {EDIT.get()}
                        </button>
                    }
                })}
            {actions
                .can_delete
                .then(|| {
                    view! {
                        <button
                            class="btn-ghost min-h-8 px-2.5 py-1 text-xs hover:bg-error/10 hover:text-error"
                            disabled=move || delete_busy.get()
                            on:click=move |_| {
                                if confirm(CONFIRM_DELETE.get()) {
                                    handlers.delete.run(delete_id.clone());
                                }
                            }
                        >
                            {DELETE.get()}
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
                class="btn-ghost min-h-8 px-2.5 py-1 text-xs"
            >
                {OPEN.get()}
            </TabLink>
            {task
                .actions
                .can_finish
                .then(|| {
                    view! {
                        <button
                            class="btn-primary min-h-8 px-2.5 py-1 text-xs"
                            disabled=move || finish_busy.get()
                            on:click=move |_| handlers.finish.run(finish_id.clone())
                        >
                            {FINISH.get()}
                        </button>
                    }
                })}
            {task
                .actions
                .can_merge
                .then(|| {
                    view! {
                        <button
                            class="btn-primary min-h-8 px-2.5 py-1 text-xs"
                            disabled=move || merge_busy.get()
                            on:click=move |_| handlers.merge.run(merge_id.clone())
                        >
                            {MERGE.get()}
                        </button>
                    }
                })}
        }
        .into_any()
    }
}

/// Left-rail classes (full literals so Tailwind can see them) colored by stage.
pub fn stage_rail_class(stage: TaskStageDto) -> &'static str {
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
pub fn pending_dialogs_label(count: usize) -> String {
    if count == 1 {
        PENDING_DIALOG_ONE.get().to_string()
    } else {
        PENDING_DIALOGS_MANY.fmt(&[("count", &count.to_string())])
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::core::i18n::Locale;

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
    pub fn pending_dialogs_label_pluralises() {
        assert_eq!(pending_dialogs_label(1), "1 dialog");
        assert_eq!(pending_dialogs_label(3), "3 dialogs");
        assert_eq!(PENDING_DIALOG_ONE.in_locale(Locale::Es), "1 diálogo");
        assert_eq!(
            PENDING_DIALOGS_MANY
                .in_locale(Locale::Es)
                .replace("{count}", "3"),
            "3 diálogos"
        );
    }
}
