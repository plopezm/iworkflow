use std::sync::Arc;

use leptos::prelude::provide_context;

use crate::core::config::AppConfig;
use crate::core::db::pool::DbPool;
use crate::modules::agent_settings::infrastructure::di::agent_settings_context::{
    AgentSettingsContext, AgentSettingsSettings,
};
use crate::modules::agent_settings::application::usecases::prepare_launch::PrepareLaunch;
use crate::modules::agent_settings::domain::errors::agent_settings_error::AgentSettingsError;
use crate::modules::agent_settings::domain::models::launch_preferences::LaunchPreferences;
use crate::modules::agent_settings::domain::models::model_ref::ModelRef;
use crate::modules::agent_settings::domain::models::thinking_level::ThinkingLevel;
use crate::modules::tasks::application::usecases::delete_workspace_tasks::DeleteWorkspaceTasks;
use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::launch_options::{LaunchModel, LaunchOptions};
use crate::modules::tasks::domain::models::workspace_ref::WorkspaceRef;
use crate::modules::tasks::domain::ports::outbound::launch_settings_port::LaunchSettingsPort;
use crate::modules::tasks::domain::ports::outbound::workspace_lookup_port::WorkspaceLookupPort;
use crate::modules::tasks::infrastructure::di::tasks_context::{TasksContext, TasksSettings};
use crate::modules::workspaces::application::usecases::get_workspace::GetWorkspace;
use crate::modules::workspaces::application::usecases::list_workspaces::ListWorkspaces;
use crate::modules::workspaces::domain::errors::workspace_error::WorkspaceError;
use crate::modules::workspaces::domain::models::workspace::{Workspace, WorkspaceId};
use crate::modules::workspaces::domain::ports::outbound::workspace_repository_port::WorkspaceRepositoryPort;
use crate::modules::workspaces::domain::ports::outbound::workspace_tasks_port::WorkspaceTasksPort;
use crate::modules::workspaces::infrastructure::adapters::outbound::persistence::workspace_repository_adapter::WorkspaceRepositoryAdapter;
use crate::modules::workspaces::infrastructure::di::workspaces_context::WorkspacesContext;

/// Composition root: agrupa los contextos de cada feature y los provee a Leptos.
#[derive(Clone)]
pub struct AppState {
    pub workspaces: WorkspacesContext,
    pub tasks: TasksContext,
    pub agent_settings: AgentSettingsContext,
}

impl AppState {
    pub fn new(pool: DbPool, config: &AppConfig) -> Self {
        let workspace_repository: Arc<dyn WorkspaceRepositoryPort> =
            Arc::new(WorkspaceRepositoryAdapter::new(pool.clone()));
        let get_workspace: Arc<GetWorkspace> =
            Arc::new(GetWorkspace::new(workspace_repository.clone()));
        let list_workspaces: Arc<ListWorkspaces> =
            Arc::new(ListWorkspaces::new(workspace_repository));
        let lookup: Arc<dyn WorkspaceLookupPort> = Arc::new(WorkspaceLookupBridge {
            get_workspace: get_workspace.clone(),
            list_workspaces: list_workspaces.clone(),
        });
        let agent_settings: AgentSettingsContext = AgentSettingsContext::new(
            pool.clone(),
            AgentSettingsSettings {
                gentle_shell_bin: config.gentle_shell_bin.clone(),
                gentle_pi_config_home: config.gentle_pi_config_home.clone(),
            },
        );
        let launch_settings: Arc<dyn LaunchSettingsPort> = Arc::new(LaunchSettingsBridge {
            prepare_launch: agent_settings.prepare_launch.clone(),
        });
        let tasks: TasksContext = TasksContext::new(
            pool.clone(),
            TasksSettings {
                gentle_shell_bin: config.gentle_shell_bin.clone(),
                worktrees_dir: config.worktrees_dir.clone(),
            },
            lookup,
            launch_settings,
        );
        let workspace_tasks: Arc<dyn WorkspaceTasksPort> = Arc::new(WorkspaceTasksBridge {
            delete_workspace_tasks: tasks.delete_workspace_tasks.clone(),
        });
        let workspaces: WorkspacesContext =
            WorkspacesContext::new(pool.clone(), config.home_dir.clone(), workspace_tasks);
        Self {
            workspaces,
            tasks,
            agent_settings,
        }
    }

    /// Registra cada contexto de feature para las server functions y el SSR.
    pub fn provide(&self) {
        provide_context(self.workspaces.clone());
        provide_context(self.tasks.clone());
        provide_context(self.agent_settings.clone());
    }
}

/// Expone los workspaces al feature de tareas a través de su propio puerto.
pub struct WorkspaceLookupBridge {
    pub get_workspace: Arc<GetWorkspace>,
    pub list_workspaces: Arc<ListWorkspaces>,
}

#[async_trait::async_trait]
impl WorkspaceLookupPort for WorkspaceLookupBridge {
    async fn find(&self, workspace_id: &str) -> Result<Option<WorkspaceRef>, TaskError> {
        let Ok(id) = WorkspaceId::parse(workspace_id) else {
            return Ok(None);
        };
        match self.get_workspace.execute(id).await {
            Ok(workspace) => Ok(Some(to_ref(workspace))),
            Err(WorkspaceError::NotFound(_)) => Ok(None),
            Err(error) => Err(TaskError::Repository(error.to_string())),
        }
    }

    async fn list(&self) -> Result<Vec<WorkspaceRef>, TaskError> {
        self.list_workspaces
            .execute()
            .await
            .map(|all: Vec<Workspace>| all.into_iter().map(to_ref).collect())
            .map_err(|e: WorkspaceError| TaskError::Repository(e.to_string()))
    }
}

fn to_ref(workspace: Workspace) -> WorkspaceRef {
    WorkspaceRef {
        id: workspace.id.to_string(),
        name: workspace.name,
        path: workspace.path,
        base_branch: workspace.base_branch,
    }
}

/// Expone la resolución del modelo de lanzamiento al feature de tareas a través de
/// su propio puerto.
pub struct LaunchSettingsBridge {
    pub prepare_launch: Arc<PrepareLaunch>,
}

#[async_trait::async_trait]
impl LaunchSettingsPort for LaunchSettingsBridge {
    async fn prepare(&self, cwd: &std::path::Path) -> Result<LaunchOptions, TaskError> {
        let preferences: LaunchPreferences = self
            .prepare_launch
            .execute(cwd)
            .await
            .map_err(|error: AgentSettingsError| TaskError::Runtime(error.to_string()))?;
        Ok(LaunchOptions {
            model: preferences.model.map(|m: ModelRef| LaunchModel {
                provider: m.provider,
                model: m.model,
            }),
            thinking: preferences
                .thinking
                .map(|t: ThinkingLevel| t.as_str().to_string()),
        })
    }
}

/// Expone el borrado en cascada de tareas al feature de workspaces a través de su
/// propio puerto.
pub struct WorkspaceTasksBridge {
    pub delete_workspace_tasks: Arc<DeleteWorkspaceTasks>,
}

#[async_trait::async_trait]
impl WorkspaceTasksPort for WorkspaceTasksBridge {
    async fn delete_all(&self, id: WorkspaceId) -> Result<(), WorkspaceError> {
        self.delete_workspace_tasks
            .execute(id.to_string())
            .await
            .map_err(|error: TaskError| match error {
                TaskError::WorkspaceHasActiveTasks => WorkspaceError::InUse,
                other => WorkspaceError::Repository(other.to_string()),
            })
    }
}
