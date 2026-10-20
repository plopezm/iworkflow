use leptos::prelude::*;
use leptos::server_fn::codec::Json;

use crate::modules::tasks::infrastructure::adapters::inbound::dto::image_attachment_dto::{
    ImageAttachmentDto, TaskAttachmentDto,
};
use crate::modules::tasks::infrastructure::adapters::inbound::dto::subagent_task_dto::SubagentTaskDto;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_diff_dto::TaskDiffDto;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
    AgentModeDto, BranchOptionDto, CreateTaskDto, CreatedTaskDto, MergeOutcomeDto, MessageModeDto,
    TaskDto, UpdateTaskDto, WorkspaceOptionDto,
};
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_event_dto::DialogAnswerDto;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_file_dto::{
    SaveTaskFileDto, TaskFileDto,
};
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_graph_dto::{
    TaskDependencyDto, TaskGraphDto,
};

#[cfg(feature = "ssr")]
mod support {
    use leptos::prelude::*;

    use crate::modules::tasks::domain::errors::task_error::TaskError;
    use crate::modules::tasks::domain::models::task::TaskId;
    use crate::modules::tasks::infrastructure::di::tasks_context::TasksContext;

    pub fn context() -> Result<TasksContext, ServerFnError> {
        use_context::<TasksContext>()
            .ok_or_else(|| ServerFnError::new("TasksContext no disponible"))
    }

    pub fn to_server_error(error: TaskError) -> ServerFnError {
        ServerFnError::new(error.to_string())
    }

    pub fn task_id(id: &str) -> Result<TaskId, ServerFnError> {
        TaskId::parse(id).map_err(to_server_error)
    }

    #[cfg(test)]
    mod tests {
        use super::*;

        #[test]
        fn domain_error_message_is_preserved() {
            assert_eq!(
                to_server_error(TaskError::AgentNotRunning),
                ServerFnError::ServerError(
                    "el agente de la tarea no está en ejecución".to_string()
                )
            );
        }

        #[test]
        fn invalid_attachment_message_is_preserved() {
            let error: TaskError =
                TaskError::InvalidAttachment("a.png: la imagen está vacía".to_string());
            let expected: String = error.to_string();

            assert_eq!(
                to_server_error(error),
                ServerFnError::ServerError(expected.clone())
            );
            assert!(expected.contains("a.png: la imagen está vacía"));
        }

        #[test]
        fn file_errors_keep_their_messages() {
            for error in [
                TaskError::InvalidFilePath("../x".to_string()),
                TaskError::FileNotEditable("a.bin: es binario".to_string()),
            ] {
                let expected: String = error.to_string();
                assert_eq!(to_server_error(error), ServerFnError::ServerError(expected));
            }
        }

        #[test]
        fn missing_task_message_is_preserved() {
            let error: TaskError = TaskError::NotFound("abc".to_string());
            let expected: String = error.to_string();

            assert_eq!(
                to_server_error(error),
                ServerFnError::ServerError(expected.clone())
            );
            assert!(expected.contains("abc"));
        }

        #[test]
        fn graph_errors_keep_their_messages() {
            for error in [
                TaskError::DependencyCycle,
                TaskError::SelfDependency,
                TaskError::DuplicateDependency,
                TaskError::CrossWorkspaceDependency,
            ] {
                let expected: String = error.to_string();
                assert_eq!(to_server_error(error), ServerFnError::ServerError(expected));
            }
        }

        #[test]
        fn invalid_id_is_rejected() {
            assert!(task_id("nope").is_err());
        }
    }
}

#[server]
pub async fn list_tasks(workspace_id: Option<String>) -> Result<Vec<TaskDto>, ServerFnError> {
    use crate::modules::tasks::domain::models::task::Task;

    let tasks: Vec<Task> = support::context()?
        .list_tasks
        .execute(workspace_id)
        .await
        .map_err(support::to_server_error)?;
    Ok(tasks.into_iter().map(TaskDto::from).collect())
}

#[server]
pub async fn list_done_tasks(
    workspace_id: Option<String>,
    limit: i64,
    offset: i64,
) -> Result<Vec<TaskDto>, ServerFnError> {
    use crate::modules::tasks::domain::models::task::Task;

    let tasks: Vec<Task> = support::context()?
        .list_done_tasks
        .execute(workspace_id, limit, offset)
        .await
        .map_err(support::to_server_error)?;
    Ok(tasks.into_iter().map(TaskDto::from).collect())
}

#[server]
pub async fn get_task(id: String) -> Result<TaskDto, ServerFnError> {
    use crate::modules::tasks::domain::models::task::Task;

    let task: Task = support::context()?
        .get_task
        .execute(support::task_id(&id)?)
        .await
        .map_err(support::to_server_error)?;
    Ok(TaskDto::from(task))
}

#[server(input = Json)]
pub async fn create_task(input: CreateTaskDto) -> Result<CreatedTaskDto, ServerFnError> {
    use crate::modules::tasks::domain::models::task::CreatedTask;

    let created: CreatedTask = support::context()?
        .create_task
        .execute(input.into())
        .await
        .map_err(support::to_server_error)?;
    Ok(CreatedTaskDto::from(created))
}

#[server(input = Json)]
pub async fn send_task_message(
    id: String,
    message: String,
    images: Vec<ImageAttachmentDto>,
    mode: MessageModeDto,
) -> Result<(), ServerFnError> {
    support::context()?
        .send_message
        .execute(
            support::task_id(&id)?,
            message,
            images.into_iter().map(Into::into).collect(),
            mode.into(),
        )
        .await
        .map_err(support::to_server_error)
}

#[server(input = Json)]
pub async fn answer_task_dialog(
    id: String,
    dialog_id: String,
    answer: DialogAnswerDto,
) -> Result<(), ServerFnError> {
    support::context()?
        .answer_dialog
        .execute(support::task_id(&id)?, dialog_id, answer.into())
        .await
        .map_err(support::to_server_error)
}

#[server]
pub async fn start_task(id: String) -> Result<TaskDto, ServerFnError> {
    use crate::modules::tasks::domain::models::task::Task;

    let task: Task = support::context()?
        .start_task
        .execute(support::task_id(&id)?)
        .await
        .map_err(support::to_server_error)?;
    Ok(TaskDto::from(task))
}

#[server(input = Json)]
pub async fn update_task(id: String, input: UpdateTaskDto) -> Result<TaskDto, ServerFnError> {
    use crate::modules::tasks::domain::models::resource_limits::ResourceLimits;
    use crate::modules::tasks::domain::models::task::Task;

    let task: Task = support::context()?
        .update_task
        .execute(
            support::task_id(&id)?,
            input.title,
            input.prompt,
            input.priority.into(),
            ResourceLimits {
                memory_mb: input.memory_limit_mb,
                cpu_percent: input.cpu_limit_percent,
            },
            input.base_branch,
            input.images.into_iter().map(Into::into).collect(),
            input.remove_attachment_ids,
        )
        .await
        .map_err(support::to_server_error)?;
    Ok(TaskDto::from(task))
}

#[server]
pub async fn list_task_attachments(id: String) -> Result<Vec<TaskAttachmentDto>, ServerFnError> {
    use crate::modules::tasks::domain::models::stored_attachment::StoredAttachment;
    use crate::modules::tasks::domain::models::task::TaskId;

    let task_id: TaskId = support::task_id(&id)?;
    let attachments: Vec<StoredAttachment> = support::context()?
        .list_task_attachments
        .execute(task_id)
        .await
        .map_err(support::to_server_error)?;
    let task_id_text: String = task_id.to_string();
    Ok(attachments
        .into_iter()
        .map(|a: StoredAttachment| TaskAttachmentDto::from_stored(&task_id_text, a))
        .collect())
}

#[server]
pub async fn delete_task(id: String) -> Result<(), ServerFnError> {
    support::context()?
        .delete_task
        .execute(support::task_id(&id)?)
        .await
        .map_err(support::to_server_error)
}

#[server]
pub async fn abort_task(id: String) -> Result<(), ServerFnError> {
    support::context()?
        .abort_task
        .execute(support::task_id(&id)?)
        .await
        .map_err(support::to_server_error)
}

#[server]
pub async fn set_task_mode(id: String, mode: AgentModeDto) -> Result<(), ServerFnError> {
    support::context()?
        .set_agent_mode
        .execute(support::task_id(&id)?, mode.into())
        .await
        .map_err(support::to_server_error)
}

#[server]
pub async fn finish_task(id: String) -> Result<(), ServerFnError> {
    support::context()?
        .finish_task
        .execute(support::task_id(&id)?)
        .await
        .map_err(support::to_server_error)
}

#[server]
pub async fn resume_task(id: String) -> Result<(), ServerFnError> {
    support::context()?
        .resume_task
        .execute(support::task_id(&id)?)
        .await
        .map_err(support::to_server_error)
}

#[server]
pub async fn merge_task(id: String) -> Result<MergeOutcomeDto, ServerFnError> {
    use crate::modules::tasks::domain::models::worktree::MergeOutcome;

    let outcome: MergeOutcome = support::context()?
        .merge_task
        .execute(support::task_id(&id)?)
        .await
        .map_err(support::to_server_error)?;
    Ok(MergeOutcomeDto::from(outcome))
}

#[server]
pub async fn get_task_graph(workspace_id: String) -> Result<TaskGraphDto, ServerFnError> {
    support::context()?
        .get_task_graph
        .execute(workspace_id)
        .await
        .map(TaskGraphDto::from)
        .map_err(support::to_server_error)
}

#[server]
pub async fn add_task_dependency(
    predecessor_id: String,
    successor_id: String,
) -> Result<TaskDependencyDto, ServerFnError> {
    support::context()?
        .add_task_dependency
        .execute(
            support::task_id(&predecessor_id)?,
            support::task_id(&successor_id)?,
        )
        .await
        .map(TaskDependencyDto::from)
        .map_err(support::to_server_error)
}

#[server]
pub async fn remove_task_dependency(
    predecessor_id: String,
    successor_id: String,
) -> Result<(), ServerFnError> {
    support::context()?
        .remove_task_dependency
        .execute(
            support::task_id(&predecessor_id)?,
            support::task_id(&successor_id)?,
        )
        .await
        .map_err(support::to_server_error)
}

#[server]
pub async fn detach_task_from_graph(id: String) -> Result<(), ServerFnError> {
    support::context()?
        .detach_task_from_graph
        .execute(support::task_id(&id)?)
        .await
        .map_err(support::to_server_error)
}

#[server]
pub async fn set_task_position(id: String, x: f64, y: f64) -> Result<(), ServerFnError> {
    support::context()?
        .set_task_position
        .execute(support::task_id(&id)?, x, y)
        .await
        .map_err(support::to_server_error)
}

#[server]
pub async fn discard_task(id: String) -> Result<(), ServerFnError> {
    support::context()?
        .discard_task
        .execute(support::task_id(&id)?)
        .await
        .map_err(support::to_server_error)
}

#[server]
pub async fn reset_task(id: String) -> Result<(), ServerFnError> {
    support::context()?
        .reset_task
        .execute(support::task_id(&id)?)
        .await
        .map_err(support::to_server_error)
}

#[server]
pub async fn get_task_diff(id: String) -> Result<TaskDiffDto, ServerFnError> {
    support::context()?
        .get_task_diff
        .execute(support::task_id(&id)?)
        .await
        .map(TaskDiffDto::from)
        .map_err(support::to_server_error)
}

#[server]
pub async fn get_task_file(id: String, path: String) -> Result<TaskFileDto, ServerFnError> {
    support::context()?
        .get_task_file
        .execute(support::task_id(&id)?, &path)
        .await
        .map(TaskFileDto::from)
        .map_err(support::to_server_error)
}

#[server]
pub async fn save_task_file(
    id: String,
    path: String,
    content: String,
) -> Result<SaveTaskFileDto, ServerFnError> {
    support::context()?
        .save_task_file
        .execute(support::task_id(&id)?, &path, &content)
        .await
        .map(SaveTaskFileDto::from)
        .map_err(support::to_server_error)
}

#[server]
pub async fn get_task_subagents(id: String) -> Result<Vec<SubagentTaskDto>, ServerFnError> {
    use crate::modules::tasks::domain::models::subagent_task::SubagentTask;

    let subagents: Vec<SubagentTask> = support::context()?
        .get_task_subagents
        .execute(support::task_id(&id)?)
        .await
        .map_err(support::to_server_error)?;
    Ok(subagents.into_iter().map(SubagentTaskDto::from).collect())
}

#[server]
pub async fn list_workspace_branches(
    workspace_id: String,
) -> Result<Vec<BranchOptionDto>, ServerFnError> {
    let branches: Vec<crate::modules::tasks::domain::models::branch_option::BranchOption> =
        support::context()?
            .list_workspace_branches
            .execute(&workspace_id)
            .await
            .map_err(support::to_server_error)?;
    Ok(branches.into_iter().map(BranchOptionDto::from).collect())
}

#[server]
pub async fn list_workspace_options() -> Result<Vec<WorkspaceOptionDto>, ServerFnError> {
    use crate::modules::tasks::domain::models::workspace_ref::WorkspaceRef;

    let workspaces: Vec<WorkspaceRef> = support::context()?
        .list_workspace_options
        .execute()
        .await
        .map_err(support::to_server_error)?;
    Ok(workspaces
        .into_iter()
        .map(WorkspaceOptionDto::from)
        .collect())
}
