use leptos::prelude::*;

use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
    CreateTaskDto, TaskDto, WorkspaceOptionDto,
};
use crate::modules::tasks::infrastructure::adapters::inbound::server_fns::{
    CreateTask, list_tasks, list_workspace_options,
};

/// Intervalo de refresco del listado de tareas (sus estados cambian en segundo plano).
#[cfg(feature = "hydrate")]
const REFRESH_INTERVAL: std::time::Duration = std::time::Duration::from_secs(5);

/// Estado del dashboard: listado de tareas y formulario de nueva tarea.
#[derive(Clone, Copy)]
pub struct DashboardState {
    pub tasks: Resource<Result<Vec<TaskDto>, ServerFnError>>,
    pub workspaces: Resource<Result<Vec<WorkspaceOptionDto>, ServerFnError>>,
    pub create: ServerAction<CreateTask>,
    pub filter_workspace: RwSignal<String>,
    pub workspace_id: RwSignal<String>,
    pub title: RwSignal<String>,
    pub prompt: RwSignal<String>,
    tick: RwSignal<u32>,
}

impl DashboardState {
    pub fn new() -> Self {
        let create: ServerAction<CreateTask> = ServerAction::<CreateTask>::new();
        let filter_workspace: RwSignal<String> = RwSignal::new(String::new());
        let tick: RwSignal<u32> = RwSignal::new(0);
        let tasks: Resource<Result<Vec<TaskDto>, ServerFnError>> = Resource::new(
            move || (filter_workspace.get(), create.version().get(), tick.get()),
            |(filter, _, _)| list_tasks(Some(filter).filter(|f: &String| !f.is_empty())),
        );
        let workspaces: Resource<Result<Vec<WorkspaceOptionDto>, ServerFnError>> =
            Resource::new(|| (), |_| list_workspace_options());
        let state: Self = Self {
            tasks,
            workspaces,
            create,
            filter_workspace,
            workspace_id: RwSignal::new(String::new()),
            title: RwSignal::new(String::new()),
            prompt: RwSignal::new(String::new()),
            tick,
        };
        state.start_polling();
        state
    }

    pub fn submit(&self) {
        self.create.dispatch(CreateTask {
            input: CreateTaskDto {
                workspace_id: self.workspace_id.get_untracked(),
                title: self.title.get_untracked(),
                prompt: self.prompt.get_untracked(),
            },
        });
    }

    #[cfg(feature = "hydrate")]
    fn start_polling(&self) {
        let tick: RwSignal<u32> = self.tick;
        if let Ok(handle) =
            set_interval_with_handle(move || tick.update(|n: &mut u32| *n += 1), REFRESH_INTERVAL)
        {
            on_cleanup(move || handle.clear());
        }
    }

    #[cfg(not(feature = "hydrate"))]
    fn start_polling(&self) {
        let _tick: RwSignal<u32> = self.tick;
    }
}

impl Default for DashboardState {
    fn default() -> Self {
        Self::new()
    }
}
