use leptos::prelude::*;
use leptos_router::NavigateOptions;
use leptos_router::hooks::use_navigate;

use crate::core::components::form::{ActionError, SelectField, SelectOption};
use crate::core::components::layout::{Card, ErrorText};
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
    TaskDto, WorkspaceOptionDto,
};
use crate::modules::tasks::presentation::components::task_form::NewTaskForm;
use crate::modules::tasks::presentation::components::task_table::TaskTable;
use crate::modules::tasks::presentation::state::dashboard_state::DashboardState;

#[component]
pub fn DashboardPage() -> impl IntoView {
    let state: DashboardState = DashboardState::new();
    let navigate = use_navigate();

    // Tras lanzar una tarea se abre su detalle (navegación de UI).
    Effect::new(move |_| {
        if let Some(Ok(task)) = state.create.value().get() {
            navigate(&format!("/tasks/{}", task.id), NavigateOptions::default());
        }
    });
    let on_submit: Callback<()> = Callback::new(move |_| state.submit());

    view! {
        <h1>"Tareas"</h1>
        <Card title="Nueva tarea">
            <Suspense fallback=|| view! { <p class="muted">"Cargando workspaces…"</p> }>
                {move || Suspend::new(async move {
                    match state.workspaces.await {
                        Ok(workspaces) => {
                            let workspaces: Vec<WorkspaceOptionDto> = workspaces;
                            view! {
                                <NewTaskForm
                                    workspaces
                                    workspace_id=state.workspace_id
                                    title=state.title
                                    prompt=state.prompt
                                    pending=state.create.pending()
                                    on_submit
                                />
                            }
                                .into_any()
                        }
                        Err(e) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                    }
                })}
            </Suspense>
            <ActionError result=state.create.value().into() />
        </Card>
        <Card title="Tareas">
            <Transition fallback=|| view! { <p class="muted">"Cargando…"</p> }>
                {move || Suspend::new(async move {
                    let workspaces: Vec<WorkspaceOptionDto> = state
                        .workspaces
                        .await
                        .unwrap_or_default();
                    let filter_options: Vec<SelectOption> = workspaces
                        .iter()
                        .map(|w: &WorkspaceOptionDto| SelectOption {
                            value: w.id.clone(),
                            label: w.name.clone(),
                        })
                        .collect();
                    match state.tasks.await {
                        Ok(tasks) => {
                            let tasks: Vec<TaskDto> = tasks;
                            view! {
                                <SelectField
                                    label="Filtrar por workspace"
                                    value=state.filter_workspace
                                    options=filter_options
                                    placeholder="Todos"
                                />
                                <TaskTable tasks workspaces />
                            }
                                .into_any()
                        }
                        Err(e) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                    }
                })}
            </Transition>
        </Card>
    }
}
