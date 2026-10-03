use leptos::prelude::*;

use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
    TaskDto, TaskPriorityDto, UpdateTaskDto,
};
use crate::modules::tasks::infrastructure::adapters::inbound::server_fns::{UpdateTask, get_task};

/// Estado de la pantalla de edición de una tarea.
#[derive(Clone, Copy)]
pub struct TaskEditState {
    pub task_id: StoredValue<String>,
    pub task: Resource<Result<TaskDto, ServerFnError>>,
    pub update: ServerAction<UpdateTask>,
    pub form_title: RwSignal<String>,
    pub form_prompt: RwSignal<String>,
    /// Valor serializado de `TaskPriorityDto` (alimenta un `SelectField`).
    pub form_priority: RwSignal<String>,
}

impl TaskEditState {
    pub fn new(task_id: String) -> Self {
        let id: StoredValue<String> = StoredValue::new(task_id);
        let task: Resource<Result<TaskDto, ServerFnError>> =
            Resource::new(|| (), move |_| get_task(id.get_value()));
        Self {
            task_id: id,
            task,
            update: ServerAction::new(),
            form_title: RwSignal::new(String::new()),
            form_prompt: RwSignal::new(String::new()),
            form_priority: RwSignal::new(TaskPriorityDto::default().as_str().to_string()),
        }
    }

    /// Precarga el formulario con los datos actuales de la tarea.
    pub fn prefill(&self, task: &TaskDto) {
        self.form_title.set(task.title.clone());
        self.form_prompt.set(task.prompt.clone());
        self.form_priority.set(task.priority.as_str().to_string());
    }

    pub fn submit(&self) {
        self.update.dispatch(UpdateTask {
            id: self.task_id.get_value(),
            input: UpdateTaskDto {
                title: self.form_title.get_untracked(),
                prompt: self.form_prompt.get_untracked(),
                priority: TaskPriorityDto::from_value(&self.form_priority.get_untracked())
                    .unwrap_or_default(),
            },
        });
    }

    /// `true` cuando la última actualización terminó con éxito.
    pub fn saved(&self) -> bool {
        self.update
            .value()
            .with(|r: &Option<Result<TaskDto, ServerFnError>>| matches!(r, Some(Ok(_))))
    }
}

#[cfg(all(test, feature = "ssr"))]
mod tests {
    use super::*;
    use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
        TaskStageDto, TaskStatusDto,
    };
    use any_spawner::Executor;

    fn task() -> TaskDto {
        TaskDto {
            id: "t1".to_string(),
            workspace_id: "w".to_string(),
            title: "Título".to_string(),
            prompt: "Prompt".to_string(),
            branch: "pi/x".to_string(),
            base_branch: "main".to_string(),
            worktree_path: "/wt".to_string(),
            status: TaskStatusDto::Idle,
            priority: Default::default(),
            error: None,
            actions: Default::default(),
            stage: TaskStageDto::Backlog,
            needs_attention: false,
            pending_dialogs: 0,
            created_at: "2026-01-01T00:00:00Z".to_string(),
            updated_at: "2026-01-01T00:00:00Z".to_string(),
        }
    }

    #[tokio::test]
    async fn prefill_copies_title_prompt_and_priority_into_the_form() {
        let _init: Result<(), _> = Executor::init_tokio();
        let owner: Owner = Owner::new();
        owner.with(|| {
            let state: TaskEditState = TaskEditState::new("t1".to_string());
            state.prefill(&task());
            assert_eq!(state.form_title.get_untracked(), "Título");
            assert_eq!(state.form_prompt.get_untracked(), "Prompt");
            assert_eq!(state.form_priority.get_untracked(), "normal");

            let mut urgent: TaskDto = task();
            urgent.priority = TaskPriorityDto::Urgent;
            state.prefill(&urgent);
            assert_eq!(state.form_priority.get_untracked(), "urgent");
        });
    }

    #[tokio::test]
    async fn saved_is_false_until_an_update_succeeds() {
        let _init: Result<(), _> = Executor::init_tokio();
        let owner: Owner = Owner::new();
        owner.with(|| {
            let state: TaskEditState = TaskEditState::new("t1".to_string());
            assert!(!state.saved());
            state.update.value().set(Some(Ok(task())));
            assert!(state.saved());
            state
                .update
                .value()
                .set(Some(Err(ServerFnError::new("boom"))));
            assert!(!state.saved());
        });
    }
}
