use crate::core::dock::context::{use_tab_nav, use_tab_params};
use leptos::prelude::*;

use crate::core::components::form::{ActionError, NumberField, SelectField, TextArea, TextField};
use crate::core::components::layout::ErrorText;
use crate::core::i18n::Msg;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::TaskDto;
use crate::modules::tasks::presentation::components::existing_attachments::ExistingAttachments;
use crate::modules::tasks::presentation::components::image_picker::{ImagePicker, paste_callback};
use crate::modules::tasks::presentation::components::task_modal::priority_select_options;
use crate::modules::tasks::presentation::state::graph_model::back_to_board_path;
use crate::modules::tasks::presentation::state::task_edit_state::TaskEditState;

const LOADING_TASK: Msg = Msg {
    en: "Loading task…",
    es: "Cargando tarea…",
    zh: "正在加载任务…",
    de: "Aufgabe wird geladen…",
    fr: "Chargement de la tâche…",
};

const TASK_NOT_EDITABLE: Msg = Msg {
    en: "This task can no longer be edited.",
    es: "Esta tarea ya no se puede editar.",
    zh: "此任务无法再编辑。",
    de: "Diese Aufgabe kann nicht mehr bearbeitet werden.",
    fr: "Cette tâche ne peut plus être modifiée.",
};

const EDIT_TASK: Msg = Msg {
    en: "Edit task",
    es: "Editar tarea",
    zh: "编辑任务",
    de: "Aufgabe bearbeiten",
    fr: "Modifier la tâche",
};

const TITLE_OPTIONAL: Msg = Msg {
    en: "Title (optional)",
    es: "Título (opcional)",
    zh: "标题（可选）",
    de: "Titel (optional)",
    fr: "Titre (facultatif)",
};

const TITLE_PLACEHOLDER: Msg = Msg {
    en: "Derived from the prompt",
    es: "Se deriva del prompt",
    zh: "由提示词自动生成",
    de: "Wird aus dem Prompt abgeleitet",
    fr: "Déduit du prompt",
};

const PRIORITY: Msg = Msg {
    en: "Priority",
    es: "Prioridad",
    zh: "优先级",
    de: "Priorität",
    fr: "Priorité",
};

const ADVANCED_OPTIONS: Msg = Msg {
    en: "Advanced options",
    es: "Opciones avanzadas",
    zh: "高级选项",
    de: "Erweiterte Optionen",
    fr: "Options avancées",
};

const MAX_RAM: Msg = Msg {
    en: "Max. RAM",
    es: "Memoria RAM máx.",
    zh: "最大内存",
    de: "Max. RAM",
    fr: "RAM max.",
};

const MAX_CPU: Msg = Msg {
    en: "Max. CPU",
    es: "CPU máx.",
    zh: "最大 CPU",
    de: "Max. CPU",
    fr: "CPU max.",
};

const PERCENT_OF_CORE: Msg = Msg {
    en: "% of one core",
    es: "% de un núcleo",
    zh: "占单个核心的百分比",
    de: "% eines Kerns",
    fr: "% d'un cœur",
};

const DEFAULT_FROM_SETTINGS: Msg = Msg {
    en: "Default (settings)",
    es: "Por defecto (ajustes)",
    zh: "默认（设置）",
    de: "Standard (Einstellungen)",
    fr: "Par défaut (paramètres)",
};

const PROMPT: Msg = Msg {
    en: "Prompt",
    es: "Prompt",
    zh: "提示词",
    de: "Prompt",
    fr: "Prompt",
};

const PROMPT_PLACEHOLDER: Msg = Msg {
    en: "Describe what the agent should do…",
    es: "Describe lo que debe hacer el agente…",
    zh: "描述智能体要做的事情…",
    de: "Beschreibe, was der Agent tun soll…",
    fr: "Décrivez ce que l'agent doit faire…",
};

const CANCEL: Msg = Msg {
    en: "Cancel",
    es: "Cancelar",
    zh: "取消",
    de: "Abbrechen",
    fr: "Annuler",
};

const SAVING: Msg = Msg {
    en: "Saving…",
    es: "Guardando…",
    zh: "正在保存…",
    de: "Wird gespeichert…",
    fr: "Enregistrement…",
};

const SAVE_CHANGES: Msg = Msg {
    en: "Save changes",
    es: "Guardar cambios",
    zh: "保存更改",
    de: "Änderungen speichern",
    fr: "Enregistrer les modifications",
};

/// Página de edición. Se reconstruye por cada id (el router reutiliza el componente).
#[component]
pub fn TaskEditPage() -> impl IntoView {
    let params = use_tab_params();
    let task_id: Memo<String> =
        Memo::new(move |_| params.with(|p| p.get("id").unwrap_or_default()));

    view! { {move || view! { <TaskEditContent task_id=task_id.get() /> }} }
}

#[component]
fn TaskEditContent(task_id: String) -> impl IntoView {
    let state: TaskEditState = TaskEditState::new(task_id);
    let nav = use_tab_nav();
    let leave: Callback<()> = Callback::new(move |_| {
        let workspace_id: Option<String> = state
            .task
            .get_untracked()
            .and_then(Result::ok)
            .map(|t: TaskDto| t.workspace_id);
        nav.close_and_open(&back_to_board_path(workspace_id.as_deref()));
    });

    // Precarga una sola vez, cuando llega la tarea.
    let prefilled: RwSignal<bool> = RwSignal::new(false);
    Effect::new(move |_| {
        if let Some(Ok(task)) = state.task.get()
            && !prefilled.get_untracked()
        {
            state.prefill(&task);
            prefilled.set(true);
        }
    });
    Effect::new(move |_| {
        if state.saved() {
            leave.run(());
        }
    });

    view! {
        <div class="h-full min-h-0 overflow-y-auto">
            <div class="mx-auto w-full max-w-5xl p-4 pb-10 md:p-6 md:pb-12">
            <Transition fallback=|| view! { <p class="muted">{LOADING_TASK.get()}</p> }>
                {move || Suspend::new(async move {
                    match state.task.await {
                        Err(e) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                        Ok(task) if !task.actions.can_edit => {
                            view! { <ErrorText message=TASK_NOT_EDITABLE.get() /> }
                                .into_any()
                        }
                        Ok(_) => {
                            view! {
                                <form
                                    class="card stack gap-4 p-5 md:p-6"
                                    on:submit=move |ev: leptos::ev::SubmitEvent| {
                                        ev.prevent_default();
                                        state.submit();
                                    }
                                >
                                    <h1 class="page-title">{EDIT_TASK.get()}</h1>
                                    <TextField
                                        label=TITLE_OPTIONAL.get()
                                        value=state.form_title
                                        placeholder=TITLE_PLACEHOLDER.get()
                                    />
                                    <SelectField
                                        label=PRIORITY.get()
                                        value=state.form_priority
                                        options=priority_select_options()
                                    />
                                    <label class="row min-h-10 items-center gap-2 text-sm text-fg">
                                        <input
                                            type="checkbox"
                                            class="h-5 w-5"
                                            prop:checked=move || state.form_show_advanced.get()
                                            on:change=move |ev| {
                                                state.form_show_advanced.set(event_target_checked(&ev))
                                            }
                                        />
                                        {ADVANCED_OPTIONS.get()}
                                    </label>
                                    <Show when=move || state.form_show_advanced.get()>
                                        <NumberField
                                            label=MAX_RAM.get()
                                            unit="MiB"
                                            placeholder=DEFAULT_FROM_SETTINGS.get()
                                            value=state.form_memory_limit
                                        />
                                        <NumberField
                                            label=MAX_CPU.get()
                                            unit=PERCENT_OF_CORE.get()
                                            placeholder=DEFAULT_FROM_SETTINGS.get()
                                            value=state.form_cpu_limit
                                        />
                                    </Show>
                                    <TextArea
                                        label=PROMPT.get()
                                        value=state.form_prompt
                                        placeholder=PROMPT_PLACEHOLDER.get()
                                        required=true
                                        large=true
                                        on_paste=paste_callback(state.images)
                                    />
                                    <Transition>
                                        <ExistingAttachments
                                            items=Signal::derive(move || state.visible_attachments())
                                            on_remove=Callback::new(move |id: String| {
                                                state.mark_removed(&id)
                                            })
                                        />
                                    </Transition>
                                    <ImagePicker attachments=state.images />
                                    <ActionError result=state.update.value().into() />
                                    <div class="row justify-end gap-2">
                                        <button
                                            type="button"
                                            class="btn-secondary"
                                            on:click=move |_| leave.run(())
                                        >
                                            {CANCEL.get()}
                                        </button>
                                        <button
                                            type="submit"
                                            class="btn-primary"
                                            disabled=move || state.update.pending().get()
                                        >
                                            {move || {
                                                if state.update.pending().get() {
                                                    SAVING.get()
                                                } else {
                                                    SAVE_CHANGES.get()
                                                }
                                            }}
                                        </button>
                                    </div>
                                </form>
                            }
                                .into_any()
                        }
                    }
                })}
            </Transition>
            </div>
        </div>
    }
}
