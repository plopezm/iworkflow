use leptos::ev::SubmitEvent;
use leptos::prelude::*;

use crate::core::i18n::Msg;

use crate::core::components::form::{
    ActionError, NumberField, SelectField, SelectOption, TextArea, TextField,
};
use crate::core::components::modal::Modal;
use crate::core::dock::link::TabLink;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
    BranchOptionDto, TaskPriorityDto, WorkspaceOptionDto,
};
use crate::modules::tasks::presentation::components::image_picker::{ImagePicker, paste_callback};
use crate::modules::tasks::presentation::state::board_state::BoardState;

const REMOTE_BRANCH: Msg = Msg {
    en: "{name} (remote)",
    es: "{name} (remota)",
    zh: "{name}（远程）",
    de: "{name} (remote)",
    fr: "{name} (distante)",
};

const LOADING_BRANCHES: Msg = Msg {
    en: "Loading branches…",
    es: "Cargando ramas…",
    zh: "正在加载分支…",
    de: "Branches werden geladen…",
    fr: "Chargement des branches…",
};

const BASE_BRANCH: Msg = Msg {
    en: "Base branch",
    es: "Rama base",
    zh: "基础分支",
    de: "Basis-Branch",
    fr: "Branche de base",
};

const WORKSPACE_BRANCH: Msg = Msg {
    en: "Workspace branch ({branch})",
    es: "Rama del workspace ({branch})",
    zh: "工作区分支（{branch}）",
    de: "Branch des Workspace ({branch})",
    fr: "Branche de l’espace de travail ({branch})",
};

const NEW_TASK: Msg = Msg {
    en: "New task",
    es: "Nueva tarea",
    zh: "新建任务",
    de: "Neue Aufgabe",
    fr: "Nouvelle tâche",
};

const SELECT_WORKSPACE: Msg = Msg {
    en: "Select a workspace…",
    es: "Selecciona un workspace…",
    zh: "选择一个工作区…",
    de: "Workspace auswählen …",
    fr: "Sélectionnez un espace de travail…",
};

const REGISTER_FIRST: Msg = Msg {
    en: "Register a workspace first.",
    es: "Registra primero un workspace.",
    zh: "请先注册一个工作区。",
    de: "Registriere zuerst einen Workspace.",
    fr: "Enregistrez d’abord un espace de travail.",
};
const GO_TO_WORKSPACES: Msg = Msg {
    en: "Go to Workspaces",
    es: "Ir a Workspaces",
    zh: "前往工作区",
    de: "Zu den Workspaces",
    fr: "Aller aux espaces de travail",
};
const TITLE_OPTIONAL: Msg = Msg {
    en: "Title (optional)",
    es: "Título (opcional)",
    zh: "标题（可选）",
    de: "Titel (optional)",
    fr: "Titre (facultatif)",
};

const DERIVED_FROM_PROMPT: Msg = Msg {
    en: "Derived from the prompt",
    es: "Se deriva del prompt",
    zh: "由提示词生成",
    de: "Wird aus dem Prompt abgeleitet",
    fr: "Déduit du prompt",
};

const PROMPT_PLACEHOLDER: Msg = Msg {
    en: "Describe what the agent should do…",
    es: "Describe lo que debe hacer el agente…",
    zh: "描述智能体应该做什么…",
    de: "Beschreibe, was der Agent tun soll …",
    fr: "Décrivez ce que l’agent doit faire…",
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
    zh: "占一个核心的百分比",
    de: "% eines Kerns",
    fr: "% d’un cœur",
};

const START_NOW: Msg = Msg {
    en: "Start now",
    es: "Iniciar ahora",
    zh: "立即开始",
    de: "Jetzt starten",
    fr: "Démarrer maintenant",
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

const CREATE_TASK: Msg = Msg {
    en: "Create task",
    es: "Crear tarea",
    zh: "创建任务",
    de: "Aufgabe erstellen",
    fr: "Créer la tâche",
};

const WORKSPACE: Msg = Msg {
    en: "Workspace",
    es: "Workspace",
    zh: "工作区",
    de: "Workspace",
    fr: "Espace de travail",
};

/// Opciones del desplegable de prioridad, de menor a mayor.
pub fn priority_select_options() -> Vec<SelectOption> {
    TaskPriorityDto::ALL
        .into_iter()
        .map(|p: TaskPriorityDto| SelectOption {
            value: p.as_str().to_string(),
            label: p.label().to_string(),
        })
        .collect()
}

/// Etiqueta de una rama en el desplegable de rama base.
pub fn branch_option_label(branch: &BranchOptionDto) -> String {
    if branch.remote {
        REMOTE_BRANCH.fmt(&[("name", &branch.name)])
    } else {
        branch.name.clone()
    }
}

/// Desplegable con las ramas locales y remotas del workspace elegido; vacío = rama base del
/// workspace.
#[component]
fn BaseBranchSelect(
    state: BoardState,
    workspaces: Signal<Vec<WorkspaceOptionDto>>,
) -> impl IntoView {
    view! {
        <Suspense fallback=|| {
            view! { <p class="muted text-sm">{LOADING_BRANCHES.get()}</p> }
        }>
            {move || {
                let branches: Vec<BranchOptionDto> = state
                    .branches
                    .get()
                    .and_then(Result::ok)
                    .unwrap_or_default();
                let workspace_id: String = state.form_workspace_id.get();
                if workspace_id.is_empty() {
                    return ().into_any();
                }
                let default_branch: String = workspaces
                    .with(|all: &Vec<WorkspaceOptionDto>| {
                        all.iter()
                            .find(|w: &&WorkspaceOptionDto| w.id == workspace_id)
                            .map(|w: &WorkspaceOptionDto| w.base_branch.clone())
                            .unwrap_or_default()
                    });
                let options: Vec<SelectOption> = branches
                    .iter()
                    .map(|b: &BranchOptionDto| SelectOption {
                        value: b.name.clone(),
                        label: branch_option_label(b),
                    })
                    .collect();
                view! {
                    <SelectField
                        label=BASE_BRANCH.get()
                        value=state.form_base_branch
                        options=options
                        placeholder=WORKSPACE_BRANCH.fmt(&[("branch", &default_branch)])
                    />
                }
                    .into_any()
            }}
        </Suspense>
    }
}

/// Modal de alta de tarea (Backlog o iniciada). La edición vive en su propia pantalla.
#[component]
pub fn TaskModal(state: BoardState, workspaces: Signal<Vec<WorkspaceOptionDto>>) -> impl IntoView {
    let pending: Signal<bool> = Signal::derive(move || state.create.pending().get());
    let has_workspaces: Memo<bool> =
        Memo::new(move |_| workspaces.with(|w: &Vec<WorkspaceOptionDto>| !w.is_empty()));
    let options: Memo<Vec<SelectOption>> = Memo::new(move |_| {
        workspaces.with(|all: &Vec<WorkspaceOptionDto>| {
            all.iter()
                .map(|w: &WorkspaceOptionDto| SelectOption {
                    value: w.id.clone(),
                    label: format!("{} ({})", w.name, w.base_branch),
                })
                .collect()
        })
    });

    let submit = move |ev: SubmitEvent| {
        ev.prevent_default();
        state.submit();
    };

    view! {
        <Modal
            title=NEW_TASK.get().to_string()
            open=state.modal_open
            on_close=Callback::new(move |_| state.close_modal())
        >
            <form class="stack" on:submit=submit>
                {move || if state.form_link.with(Option::is_some) {
                        // Una tarea enlazada vive en el workspace del tablero: no se elige.
                        ().into_any()
                    } else if has_workspaces.get() {
                        view! {
                            <SelectField
                                label=WORKSPACE.get()
                                value=state.form_workspace_id
                                options=options.get()
                                placeholder=SELECT_WORKSPACE.get()
                                required=true
                            />
                        }
                            .into_any()
                    } else {
                        view! {
                            <p class="muted">
                                {REGISTER_FIRST.get()} " "
                                <TabLink href="/workspaces" class="text-accent">
                                    {GO_TO_WORKSPACES.get()}
                                </TabLink>
                            </p>
                        }
                            .into_any()
                }}
                <BaseBranchSelect state=state workspaces=workspaces />
                <TextField
                    label=TITLE_OPTIONAL.get()
                    value=state.form_title
                    placeholder=DERIVED_FROM_PROMPT.get()
                />
                <TextArea
                    label="Prompt"
                    value=state.form_prompt
                    placeholder=PROMPT_PLACEHOLDER.get()
                    required=true
                    on_paste=paste_callback(state.images)
                />
                <ImagePicker attachments=state.images />
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
                        on:change=move |ev| state.form_show_advanced.set(event_target_checked(&ev))
                    />
                    {ADVANCED_OPTIONS.get()}
                </label>
                <Show when=move || state.form_show_advanced.get()>
                    <NumberField
                        label=MAX_RAM.get()
                        unit="MiB"
                        placeholder="Por defecto (ajustes)"
                        value=state.form_memory_limit
                    />
                    <NumberField
                        label=MAX_CPU.get()
                        unit=PERCENT_OF_CORE.get()
                        placeholder="Por defecto (ajustes)"
                        value=state.form_cpu_limit
                    />
                </Show>
                // Una tarea enlazada espera a sus predecesoras: no puede iniciarse al crearla.
                <Show when=move || state.form_link.with(Option::is_none)>
                    <label class="row min-h-10 items-center gap-2 text-sm text-fg">
                        <input
                            type="checkbox"
                            class="h-5 w-5"
                            prop:checked=move || state.form_start_now.get()
                            on:change=move |ev| state.form_start_now.set(event_target_checked(&ev))
                        />
                        {START_NOW.get()}
                    </label>
                </Show>
                <ActionError result=state.create.value().into() />
                <div class="row justify-end gap-2">
                    <button type="button" class="btn-secondary" on:click=move |_| state.close_modal()>
                        {CANCEL.get()}
                    </button>
                    <button
                        type="submit"
                        class="btn-primary"
                        disabled=move || pending.get() || !has_workspaces.get()
                    >
                        {move || {
                            if pending.get() { SAVING.get() } else { CREATE_TASK.get() }
                        }}
                    </button>
                </div>
            </form>
        </Modal>
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::core::i18n::Locale;

    #[test]
    fn priority_options_cover_every_priority_in_ascending_order() {
        let options: Vec<SelectOption> = priority_select_options();
        let labels: Vec<&str> = options
            .iter()
            .map(|o: &SelectOption| o.label.as_str())
            .collect();
        let values: Vec<&str> = options
            .iter()
            .map(|o: &SelectOption| o.value.as_str())
            .collect();

        assert_eq!(labels, vec!["Low", "Normal", "High", "Urgent"]);
        assert_eq!(values, vec!["low", "normal", "high", "urgent"]);
    }

    #[test]
    fn remote_branches_are_labelled() {
        let remote: BranchOptionDto = BranchOptionDto {
            name: "origin/develop".to_string(),
            remote: true,
            current: false,
        };
        let local: BranchOptionDto = BranchOptionDto {
            name: "main".to_string(),
            remote: false,
            current: true,
        };

        assert_eq!(branch_option_label(&remote), "origin/develop (remote)");
        assert_eq!(
            REMOTE_BRANCH
                .in_locale(Locale::Es)
                .replace("{name}", "origin/develop"),
            "origin/develop (remota)"
        );
        assert_eq!(branch_option_label(&local), "main");
    }
}
