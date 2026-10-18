use leptos::prelude::*;

use crate::core::components::form::{ActionError, SelectField, SelectOption, TextArea, TextField};
use crate::core::components::layout::ErrorText;
use crate::core::dock::context::{use_tab_nav, use_tab_params};
use crate::core::dock::link::TabLink;
use crate::core::i18n::Msg;
use crate::modules::jira::infrastructure::adapters::inbound::dto::jira_dto::{
    ConversionBranchDto, ConversionPreparationDto, ConversionWorkspaceDto, ConvertedIssueDto,
};
use crate::modules::jira::presentation::state::jira_convert_state::JiraConvertState;

/// The new task opens on its own unless something needs the user's attention first (the agent
/// could not start, or the task could not be linked back to the Jira issue).
fn opens_automatically(converted: &ConvertedIssueDto) -> bool {
    converted.start_error.is_none()
}

const PRIORITIES: [(&str, Msg); 4] = [
    ("low", PRIORITY_LOW),
    ("normal", PRIORITY_NORMAL),
    ("high", PRIORITY_HIGH),
    ("urgent", PRIORITY_URGENT),
];

const PRIORITY_LOW: Msg = Msg {
    en: "Low",
    es: "Baja",
    zh: "低",
    de: "Niedrig",
    fr: "Basse",
};

const PRIORITY_NORMAL: Msg = Msg {
    en: "Normal",
    es: "Normal",
    zh: "普通",
    de: "Normal",
    fr: "Normale",
};

const PRIORITY_HIGH: Msg = Msg {
    en: "High",
    es: "Alta",
    zh: "高",
    de: "Hoch",
    fr: "Haute",
};

const PRIORITY_URGENT: Msg = Msg {
    en: "Urgent",
    es: "Urgente",
    zh: "紧急",
    de: "Dringend",
    fr: "Urgente",
};

const BRANCH_REMOTE: Msg = Msg {
    en: "{name} (remote)",
    es: "{name} (remota)",
    zh: "{name}（远程）",
    de: "{name} (remote)",
    fr: "{name} (distante)",
};

const PREPARING: Msg = Msg {
    en: "Preparing conversion…",
    es: "Preparando conversión…",
    zh: "正在准备转换…",
    de: "Konvertierung wird vorbereitet…",
    fr: "Préparation de la conversion…",
};

const HEADING: Msg = Msg {
    en: "Convert {key} into an iworkflow task",
    es: "Convertir {key} en tarea de iworkflow",
    zh: "将 {key} 转换为 iworkflow 任务",
    de: "{key} in eine iworkflow-Aufgabe umwandeln",
    fr: "Convertir {key} en tâche iworkflow",
};

const REVIEW_HINT: Msg = Msg {
    en: "Review how the Jira issue is mapped and edit the prompt before creating the task.",
    es: "Revisa cómo se mapea la tarea de Jira y edita el prompt antes de crear la tarea.",
    zh: "在创建任务之前，请检查 Jira 任务的映射方式并编辑提示词。",
    de: "Prüfe, wie die Jira-Aufgabe abgebildet wird, und bearbeite den Prompt, bevor du die Aufgabe erstellst.",
    fr: "Vérifiez comment la tâche Jira est mappée et modifiez le prompt avant de créer la tâche.",
};

const VIEW_JIRA_ISSUE: Msg = Msg {
    en: "View Jira issue",
    es: "Ver tarea de Jira",
    zh: "查看 Jira 任务",
    de: "Jira-Aufgabe ansehen",
    fr: "Voir la tâche Jira",
};

const WORKSPACE: Msg = Msg {
    en: "Workspace",
    es: "Workspace",
    zh: "工作区",
    de: "Workspace",
    fr: "Espace de travail",
};

const PICK_WORKSPACE: Msg = Msg {
    en: "Select a workspace…",
    es: "Selecciona un workspace…",
    zh: "选择一个工作区…",
    de: "Workspace auswählen…",
    fr: "Sélectionnez un espace de travail…",
};

const NO_WORKSPACE_PREFIX: Msg = Msg {
    en: "Register a workspace first in ",
    es: "Registra primero un workspace en ",
    zh: "请先在",
    de: "Registriere zuerst einen Workspace unter ",
    fr: "Enregistrez d'abord un espace de travail dans ",
};

const WORKSPACES: Msg = Msg {
    en: "Workspaces",
    es: "Workspaces",
    zh: "工作区",
    de: "Workspaces",
    fr: "Espaces de travail",
};

const NO_WORKSPACE_SUFFIX: Msg = Msg {
    en: ".",
    es: ".",
    zh: "中注册一个工作区。",
    de: ".",
    fr: ".",
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
    en: "Workspace branch",
    es: "Rama del workspace",
    zh: "工作区分支",
    de: "Branch des Workspace",
    fr: "Branche de l'espace de travail",
};

const TITLE: Msg = Msg {
    en: "Title",
    es: "Título",
    zh: "标题",
    de: "Titel",
    fr: "Titre",
};

const PRIORITY: Msg = Msg {
    en: "Priority",
    es: "Prioridad",
    zh: "优先级",
    de: "Priorität",
    fr: "Priorité",
};

const PROMPT: Msg = Msg {
    en: "Prompt",
    es: "Prompt",
    zh: "提示词",
    de: "Prompt",
    fr: "Prompt",
};

const START_NOW: Msg = Msg {
    en: "Start now",
    es: "Iniciar ahora",
    zh: "立即开始",
    de: "Jetzt starten",
    fr: "Démarrer maintenant",
};

const OPEN_CREATED: Msg = Msg {
    en: "Open the created task",
    es: "Abrir la tarea creada",
    zh: "打开已创建的任务",
    de: "Erstellte Aufgabe öffnen",
    fr: "Ouvrir la tâche créée",
};

const CREATING: Msg = Msg {
    en: "Creating…",
    es: "Creando…",
    zh: "创建中…",
    de: "Wird erstellt…",
    fr: "Création…",
};

const CREATE_TASK: Msg = Msg {
    en: "Create task",
    es: "Crear tarea",
    zh: "创建任务",
    de: "Aufgabe erstellen",
    fr: "Créer la tâche",
};

fn priority_options() -> Vec<SelectOption> {
    PRIORITIES
        .into_iter()
        .map(|(value, label): (&str, Msg)| SelectOption {
            value: value.to_string(),
            label: label.get().to_string(),
        })
        .collect()
}

fn branch_label(branch: &ConversionBranchDto) -> String {
    if branch.remote {
        BRANCH_REMOTE.fmt(&[("name", &branch.name)])
    } else {
        branch.name.clone()
    }
}

/// Convierte una tarea de Jira en una tarea de iworkflow, con mapeo y prompt editables.
#[component]
pub fn JiraConvertPage() -> impl IntoView {
    let params = use_tab_params();
    let key: Memo<String> = Memo::new(move |_| params.with(|p| p.get("key").unwrap_or_default()));
    view! { {move || view! { <JiraConvertContent key=key.get() /> }} }
}

#[component]
fn JiraConvertContent(key: String) -> impl IntoView {
    let state: JiraConvertState = JiraConvertState::new(key.clone());
    let nav = use_tab_nav();
    // The new task replaces this tab so the user lands on it.
    Effect::new(move |_| {
        if let Some(converted) = state.converted()
            && opens_automatically(&converted)
        {
            nav.replace(&format!("/tasks/{}", converted.task_id));
        }
    });

    view! {
        <div class="h-full min-h-0 overflow-y-auto">
            <div class="mx-auto w-full max-w-5xl p-4 pb-10 md:p-6 md:pb-12">
                <Transition fallback=|| view! { <p class="muted">{PREPARING.get()}</p> }>
                    {move || Suspend::new(async move {
                        match state.preparation.await {
                            Err(e) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                            Ok(prepared) => convert_form(state, prepared).into_any(),
                        }
                    })}
                </Transition>
            </div>
        </div>
    }
}

fn convert_form(state: JiraConvertState, prepared: ConversionPreparationDto) -> impl IntoView {
    let pending: Signal<bool> = state.convert.pending().into();
    let detail_path: String = format!("/jira/{}", prepared.issue.key);
    let heading: String = HEADING.fmt(&[("key", &prepared.issue.key)]);
    let workspaces: Vec<ConversionWorkspaceDto> = prepared.workspaces;
    let has_workspaces: bool = !workspaces.is_empty();
    let workspace_options: Vec<SelectOption> = workspaces
        .iter()
        .map(|w: &ConversionWorkspaceDto| SelectOption {
            value: w.id.clone(),
            label: format!("{} ({})", w.name, w.base_branch),
        })
        .collect();
    view! {
        <form
            class="card stack gap-4 p-5 md:p-6"
            on:submit=move |ev: leptos::ev::SubmitEvent| {
                ev.prevent_default();
                state.submit();
            }
        >
            <h1 class="page-title">{heading}</h1>
            <p class="muted text-sm">
                {REVIEW_HINT.get()}" "
                <TabLink href=detail_path class="text-accent">{VIEW_JIRA_ISSUE.get()}</TabLink>
            </p>
            {if has_workspaces {
                view! {
                    <SelectField
                        label=WORKSPACE.get()
                        value=state.form_workspace_id
                        options=workspace_options
                        placeholder=PICK_WORKSPACE.get()
                        required=true
                    />
                }
                    .into_any()
            } else {
                view! {
                    <p class="muted">
                        {NO_WORKSPACE_PREFIX.get()}
                        <TabLink href="/workspaces" class="text-accent">{WORKSPACES.get()}</TabLink>
                        {NO_WORKSPACE_SUFFIX.get()}
                    </p>
                }
                    .into_any()
            }}
            <Transition fallback=|| view! { <p class="muted text-sm">{LOADING_BRANCHES.get()}</p> }>
                {move || Suspend::new(async move {
                    let branches: Vec<ConversionBranchDto> = state
                        .branches
                        .await
                        .unwrap_or_default();
                    if state.form_workspace_id.get_untracked().is_empty() {
                        return ().into_any();
                    }
                    let options: Vec<SelectOption> = branches
                        .iter()
                        .map(|b: &ConversionBranchDto| SelectOption {
                            value: b.name.clone(),
                            label: branch_label(b),
                        })
                        .collect();
                    view! {
                        <SelectField
                            label=BASE_BRANCH.get()
                            value=state.form_base_branch
                            options=options
                            placeholder=WORKSPACE_BRANCH.get()
                        />
                    }
                        .into_any()
                })}
            </Transition>
            <TextField label=TITLE.get() value=state.form_title required=true />
            <SelectField label=PRIORITY.get() value=state.form_priority options=priority_options() />
            <TextArea label=PROMPT.get() value=state.form_prompt required=true large=true />
            <label class="row min-h-10 items-center gap-2 text-sm text-fg">
                <input
                    type="checkbox"
                    class="h-5 w-5"
                    prop:checked=move || state.form_start_now.get()
                    on:change=move |ev| state.form_start_now.set(event_target_checked(&ev))
                />
                {START_NOW.get()}
            </label>
            <ActionError result=state.convert.value().into() />
            {move || {
                state
                    .converted()
                    .and_then(|c: ConvertedIssueDto| {
                        c.start_error.map(|message: String| (c.task_id, message))
                    })
                    .map(|(task_id, message): (String, String)| {
                        view! {
                            <div class="stack gap-2">
                                <ErrorText message=message />
                                <TabLink href=format!("/tasks/{task_id}") class="text-accent">
                                    {OPEN_CREATED.get()}
                                </TabLink>
                            </div>
                        }
                    })
            }}
            <div class="row justify-end gap-2">
                <button
                    type="submit"
                    class="btn-primary"
                    disabled=move || pending.get() || !has_workspaces
                >
                    {move || if pending.get() { CREATING.get() } else { CREATE_TASK.get() }}
                </button>
            </div>
        </form>
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn converted(start_error: Option<&str>) -> ConvertedIssueDto {
        ConvertedIssueDto {
            task_id: "t1".to_string(),
            start_error: start_error.map(str::to_string),
        }
    }

    #[test]
    fn the_task_opens_on_its_own_only_when_nothing_needs_attention() {
        assert!(opens_automatically(&converted(None)));
        assert!(!opens_automatically(&converted(Some("sin agente"))));
    }
}
