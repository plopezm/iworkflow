use leptos::prelude::*;

use crate::core::components::form::{ActionError, SelectField, SelectOption, TextArea, TextField};
use crate::core::components::layout::ErrorText;
use crate::core::dock::context::{use_tab_nav, use_tab_params};
use crate::core::dock::link::TabLink;
use crate::modules::jira::infrastructure::adapters::inbound::dto::jira_dto::{
    ConversionBranchDto, ConversionPreparationDto, ConversionWorkspaceDto, ConvertedIssueDto,
};
use crate::modules::jira::presentation::state::jira_convert_state::JiraConvertState;

const PRIORITIES: [(&str, &str); 4] = [
    ("low", "Baja"),
    ("normal", "Normal"),
    ("high", "Alta"),
    ("urgent", "Urgente"),
];

fn priority_options() -> Vec<SelectOption> {
    PRIORITIES
        .into_iter()
        .map(|(value, label): (&str, &str)| SelectOption {
            value: value.to_string(),
            label: label.to_string(),
        })
        .collect()
}

fn branch_label(branch: &ConversionBranchDto) -> String {
    if branch.remote {
        format!("{} (remota)", branch.name)
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
        if let Some(ConvertedIssueDto { task_id, .. }) = state.converted() {
            nav.replace(&format!("/tasks/{task_id}"));
        }
    });

    view! {
        <div class="h-full min-h-0 overflow-y-auto">
            <div class="mx-auto w-full max-w-5xl p-4 pb-10 md:p-6 md:pb-12">
                <Transition fallback=|| view! { <p class="muted">"Preparando conversión…"</p> }>
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
    let heading: String = format!("Convertir {} en tarea de iworkflow", prepared.issue.key);
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
                "Revisa cómo se mapea la tarea de Jira y edita el prompt antes de crear la tarea. "
                <TabLink href=detail_path class="text-accent">"Ver tarea de Jira"</TabLink>
            </p>
            {if has_workspaces {
                view! {
                    <SelectField
                        label="Workspace"
                        value=state.form_workspace_id
                        options=workspace_options
                        placeholder="Selecciona un workspace…"
                        required=true
                    />
                }
                    .into_any()
            } else {
                view! {
                    <p class="muted">
                        "Registra primero un workspace en "
                        <TabLink href="/workspaces" class="text-accent">"Workspaces"</TabLink>
                        "."
                    </p>
                }
                    .into_any()
            }}
            <Transition fallback=|| view! { <p class="muted text-sm">"Cargando ramas…"</p> }>
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
                            label="Rama base"
                            value=state.form_base_branch
                            options=options
                            placeholder="Rama del workspace"
                        />
                    }
                        .into_any()
                })}
            </Transition>
            <TextField label="Título" value=state.form_title required=true />
            <SelectField label="Prioridad" value=state.form_priority options=priority_options() />
            <TextArea label="Prompt" value=state.form_prompt required=true large=true />
            <label class="row min-h-10 items-center gap-2 text-sm text-fg">
                <input
                    type="checkbox"
                    class="h-5 w-5"
                    prop:checked=move || state.form_start_now.get()
                    on:change=move |ev| state.form_start_now.set(event_target_checked(&ev))
                />
                "Iniciar ahora"
            </label>
            <ActionError result=state.convert.value().into() />
            <div class="row justify-end gap-2">
                <button
                    type="submit"
                    class="btn-primary"
                    disabled=move || pending.get() || !has_workspaces
                >
                    {move || if pending.get() { "Creando…" } else { "Crear tarea" }}
                </button>
            </div>
        </form>
    }
}
