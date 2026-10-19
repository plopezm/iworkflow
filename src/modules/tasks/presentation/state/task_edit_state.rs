use leptos::prelude::*;

use crate::core::components::form::parse_positive_number;

use crate::modules::tasks::infrastructure::adapters::inbound::dto::image_attachment_dto::{
    ImageAttachmentDto, TaskAttachmentDto,
};
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
    BranchOptionDto, TaskDto, TaskPriorityDto, UpdateTaskDto, WorkspaceOptionDto,
};
use crate::modules::tasks::infrastructure::adapters::inbound::server_fns::{
    UpdateTask, get_task, list_task_attachments, list_workspace_branches, list_workspace_options,
};
use crate::modules::tasks::presentation::state::board_state::effective_limits;
use crate::modules::tasks::presentation::state::image_attachments::{ImageAttachments, MAX_IMAGES};

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
    pub form_memory_limit: RwSignal<String>,
    pub form_cpu_limit: RwSignal<String>,
    /// Rama de la que parte el worktree; vacío = base del workspace.
    pub form_base_branch: RwSignal<String>,
    /// Ramas locales y remotas del repositorio de la tarea.
    pub branches: Resource<Result<Vec<BranchOptionDto>, ServerFnError>>,
    /// Rama base del workspace de la tarea (la que se usa si no se elige otra).
    pub default_branch: Memo<String>,
    /// Muestra las opciones avanzadas (límites de RAM y CPU).
    pub form_show_advanced: RwSignal<bool>,
    /// Imágenes nuevas a añadir a las ya adjuntas.
    pub images: ImageAttachments,
    /// Imágenes ya guardadas en la tarea.
    pub attachments: Resource<Result<Vec<TaskAttachmentDto>, ServerFnError>>,
    /// Ids de imágenes guardadas marcadas para quitar al guardar.
    pub removed_ids: RwSignal<Vec<String>>,
}

/// Marca o desmarca `id` para quitar (función pura).
pub fn toggle_removed(removed: &mut Vec<String>, id: &str) {
    match removed.iter().position(|r: &String| r == id) {
        Some(index) => {
            removed.remove(index);
        }
        None => removed.push(id.to_string()),
    }
}

/// Imágenes guardadas que se conservan: las existentes menos las marcadas (función pura).
pub fn visible_attachments(
    existing: &[TaskAttachmentDto],
    removed: &[String],
) -> Vec<TaskAttachmentDto> {
    existing
        .iter()
        .filter(|a: &&TaskAttachmentDto| !removed.contains(&a.id))
        .cloned()
        .collect()
}

/// Huecos libres para imágenes nuevas dado el total conservado (función pura).
pub fn remaining_slots(kept: usize) -> usize {
    MAX_IMAGES.saturating_sub(kept)
}

/// Construye el DTO de edición con las imágenes nuevas (función pura).
// Un argumento por campo del formulario; agruparlos solo movería el problema.
#[allow(clippy::too_many_arguments)]
pub fn build_update_task_dto(
    title: String,
    prompt: String,
    priority: &str,
    memory_limit: &str,
    cpu_limit: &str,
    base_branch: &str,
    images: Vec<ImageAttachmentDto>,
    remove_attachment_ids: Vec<String>,
) -> UpdateTaskDto {
    UpdateTaskDto {
        title,
        prompt,
        priority: TaskPriorityDto::from_value(priority).unwrap_or_default(),
        memory_limit_mb: parse_positive_number(memory_limit),
        cpu_limit_percent: parse_positive_number(cpu_limit),
        base_branch: Some(base_branch.to_string()).filter(|b: &String| !b.trim().is_empty()),
        images,
        remove_attachment_ids,
    }
}

impl TaskEditState {
    pub fn new(task_id: String) -> Self {
        let id: StoredValue<String> = StoredValue::new(task_id);
        let task: Resource<Result<TaskDto, ServerFnError>> =
            Resource::new(|| (), move |_| get_task(id.get_value()));
        let attachments: Resource<Result<Vec<TaskAttachmentDto>, ServerFnError>> =
            Resource::new(|| (), move |_| list_task_attachments(id.get_value()));
        let branches: Resource<Result<Vec<BranchOptionDto>, ServerFnError>> = Resource::new(
            || (),
            move |_| async move {
                let task: TaskDto = get_task(id.get_value()).await?;
                list_workspace_branches(task.workspace_id).await
            },
        );
        let workspaces: Resource<Result<Vec<WorkspaceOptionDto>, ServerFnError>> =
            Resource::new(|| (), |_| list_workspace_options());
        let default_branch: Memo<String> = Memo::new(move |_| {
            let workspace_id: String = task
                .get()
                .and_then(Result::ok)
                .map(|t: TaskDto| t.workspace_id)
                .unwrap_or_default();
            workspaces
                .get()
                .and_then(Result::ok)
                .and_then(|all: Vec<WorkspaceOptionDto>| {
                    all.into_iter()
                        .find(|w: &WorkspaceOptionDto| w.id == workspace_id)
                })
                .map(|w: WorkspaceOptionDto| w.base_branch)
                .unwrap_or_default()
        });
        let removed_ids: RwSignal<Vec<String>> = RwSignal::new(Vec::new());
        let images: ImageAttachments = ImageAttachments::new();
        Effect::new(move |_| {
            let kept: usize = visible_attachments(
                &attachments.get().and_then(Result::ok).unwrap_or_default(),
                &removed_ids.get(),
            )
            .len();
            images.reserved.set(kept);
        });
        Self {
            task_id: id,
            task,
            update: ServerAction::new(),
            form_title: RwSignal::new(String::new()),
            form_prompt: RwSignal::new(String::new()),
            form_priority: RwSignal::new(TaskPriorityDto::default().as_str().to_string()),
            form_memory_limit: RwSignal::new(String::new()),
            form_cpu_limit: RwSignal::new(String::new()),
            form_base_branch: RwSignal::new(String::new()),
            branches,
            default_branch,
            form_show_advanced: RwSignal::new(false),
            images,
            attachments,
            removed_ids,
        }
    }

    /// Imágenes guardadas que siguen visibles (sin las marcadas para quitar).
    pub fn visible_attachments(&self) -> Vec<TaskAttachmentDto> {
        let existing: Vec<TaskAttachmentDto> = self
            .attachments
            .get()
            .and_then(Result::ok)
            .unwrap_or_default();
        self.removed_ids
            .with(|removed: &Vec<String>| visible_attachments(&existing, removed))
    }

    /// Marca una imagen guardada para quitarla al guardar.
    pub fn mark_removed(&self, id: &str) {
        self.removed_ids
            .update(|removed: &mut Vec<String>| toggle_removed(removed, id));
    }

    /// Precarga el formulario con los datos actuales de la tarea.
    pub fn prefill(&self, task: &TaskDto) {
        self.form_title.set(task.title.clone());
        self.form_prompt.set(task.prompt.clone());
        self.form_priority.set(task.priority.as_str().to_string());
        self.form_memory_limit.set(
            task.memory_limit_mb
                .map(|v: u32| v.to_string())
                .unwrap_or_default(),
        );
        self.form_cpu_limit.set(
            task.cpu_limit_percent
                .map(|v: u32| v.to_string())
                .unwrap_or_default(),
        );
        self.form_base_branch.set(task.base_branch.clone());
        // Si la tarea ya tiene límites propios, la sección se abre para que se vean.
        self.form_show_advanced
            .set(task.memory_limit_mb.is_some() || task.cpu_limit_percent.is_some());
    }

    pub fn submit(&self) {
        // Con la sección avanzada oculta los límites no aplican: se usan los de ajustes.
        let (memory_limit, cpu_limit): (String, String) = effective_limits(
            self.form_show_advanced.get_untracked(),
            &self.form_memory_limit.get_untracked(),
            &self.form_cpu_limit.get_untracked(),
        );
        self.update.dispatch(UpdateTask {
            id: self.task_id.get_value(),
            input: build_update_task_dto(
                self.form_title.get_untracked(),
                self.form_prompt.get_untracked(),
                &self.form_priority.get_untracked(),
                &memory_limit,
                &cpu_limit,
                &self.form_base_branch.get_untracked(),
                self.images.images.get_untracked(),
                self.removed_ids.get_untracked(),
            ),
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
            memory_limit_mb: None,
            cpu_limit_percent: None,
            error: None,
            resume_at: None,
            actions: Default::default(),
            stage: TaskStageDto::Backlog,
            needs_attention: false,
            pending_dialogs: 0,
            created_at: "2026-01-01T00:00:00Z".to_string(),
            updated_at: "2026-01-01T00:00:00Z".to_string(),
        }
    }

    #[test]
    fn update_dto_carries_images_and_priority() {
        let image: ImageAttachmentDto = ImageAttachmentDto {
            name: "a.png".to_string(),
            mime_type: "image/png".to_string(),
            data: "aGVsbG8=".to_string(),
        };
        let dto: UpdateTaskDto = build_update_task_dto(
            "t".to_string(),
            "p".to_string(),
            "urgent",
            "512",
            "150",
            " origin/dev ",
            vec![image.clone()],
            vec!["a1".to_string()],
        );
        assert_eq!(dto.images, vec![image]);
        assert_eq!(dto.remove_attachment_ids, vec!["a1".to_string()]);
        assert_eq!(dto.priority, TaskPriorityDto::Urgent);
        assert_eq!(dto.memory_limit_mb, Some(512));
        assert_eq!(dto.cpu_limit_percent, Some(150));
        assert_eq!(dto.base_branch.as_deref(), Some(" origin/dev "));
    }

    #[test]
    fn blank_base_branch_is_sent_as_none() {
        let dto: UpdateTaskDto = build_update_task_dto(
            "t".to_string(),
            "p".to_string(),
            "normal",
            "",
            "",
            "  ",
            Vec::new(),
            Vec::new(),
        );
        assert_eq!(dto.base_branch, None);
    }

    fn stored(id: &str) -> TaskAttachmentDto {
        TaskAttachmentDto {
            id: id.to_string(),
            name: format!("{id}.png"),
            mime_type: "image/png".to_string(),
            size_bytes: 2,
            url: format!("/api/tasks/t/attachments/{id}"),
        }
    }

    #[test]
    fn toggle_removed_marks_and_unmarks() {
        let mut removed: Vec<String> = Vec::new();
        toggle_removed(&mut removed, "a");
        toggle_removed(&mut removed, "b");
        assert_eq!(removed, vec!["a".to_string(), "b".to_string()]);
        toggle_removed(&mut removed, "a");
        assert_eq!(removed, vec!["b".to_string()]);
    }

    #[test]
    fn visible_attachments_excludes_removed_ids() {
        let existing: Vec<TaskAttachmentDto> = vec![stored("a"), stored("b"), stored("c")];
        let visible: Vec<TaskAttachmentDto> =
            visible_attachments(&existing, &["b".to_string(), "zz".to_string()]);
        assert_eq!(visible, vec![stored("a"), stored("c")]);
        assert_eq!(visible_attachments(&existing, &[]).len(), 3);
    }

    #[test]
    fn remaining_slots_respects_total_maximum() {
        assert_eq!(remaining_slots(0), MAX_IMAGES);
        assert_eq!(remaining_slots(2), MAX_IMAGES - 2);
        assert_eq!(remaining_slots(MAX_IMAGES + 3), 0);
    }

    #[tokio::test]
    async fn marking_removed_updates_state_and_submit_payload() {
        let _init: Result<(), _> = Executor::init_tokio();
        let owner: Owner = Owner::new();
        owner.with(|| {
            let state: TaskEditState = TaskEditState::new("t1".to_string());
            state.mark_removed("a");
            assert_eq!(state.removed_ids.get_untracked(), vec!["a".to_string()]);
            state.mark_removed("a");
            assert!(state.removed_ids.get_untracked().is_empty());
        });
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
    async fn prefill_opens_advanced_section_only_when_task_has_limits() {
        let _init: Result<(), _> = Executor::init_tokio();
        let owner: Owner = Owner::new();
        owner.with(|| {
            let state: TaskEditState = TaskEditState::new("t1".to_string());
            state.prefill(&task());
            assert!(!state.form_show_advanced.get_untracked());

            let mut limited: TaskDto = task();
            limited.cpu_limit_percent = Some(150);
            state.prefill(&limited);
            assert!(state.form_show_advanced.get_untracked());
            assert_eq!(state.form_cpu_limit.get_untracked(), "150");
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
