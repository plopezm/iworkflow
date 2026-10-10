use std::sync::Arc;

use leptos::prelude::provide_context;

use crate::core::config::AppConfig;
use crate::core::db::pool::DbPool;
use crate::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;
use crate::modules::agent_profiles::domain::ports::outbound::model_options_port::{
    ModelOption, ModelOptionsPort,
};
use crate::modules::agent_profiles::infrastructure::di::agent_profiles_context::{
    AgentProfilesContext, AgentProfilesSettings,
};
use crate::modules::agent_settings::application::usecases::list_models::ListModels;
use crate::modules::agents_files::domain::errors::agents_file_error::AgentsFileError;
use crate::modules::agents_files::application::usecases::install_agents_file::InstallAgentsFile;
use crate::modules::agents_files::application::usecases::list_agents_files::ListAgentsFiles;
use crate::modules::agents_files::domain::models::agents_file::{AgentsFile, AgentsFileId};
use crate::modules::agents_files::domain::models::project_ref::ProjectRef;
use crate::modules::agents_files::domain::ports::outbound::project_lookup_port::ProjectLookupPort;
use crate::modules::agents_files::infrastructure::di::agents_files_context::AgentsFilesContext;
use crate::modules::agent_settings::infrastructure::di::agent_settings_context::{
    AgentSettingsContext, AgentSettingsSettings,
};
use crate::modules::agent_settings::application::usecases::prepare_launch::PrepareLaunch;
use crate::modules::agent_settings::domain::errors::agent_settings_error::AgentSettingsError;
use crate::modules::agent_settings::domain::models::launch_preferences::LaunchPreferences;
use crate::modules::agent_settings::domain::models::agent_runtime::AgentRuntime;
use crate::modules::agent_settings::domain::models::model_ref::ModelRef;
use crate::modules::agent_settings::domain::models::thinking_level::ThinkingLevel;
use crate::modules::project_tree::domain::models::workspace_root::WorkspaceRoot;
use crate::modules::project_tree::infrastructure::adapters::outbound::workspace_root::resolver_workspace_root_adapter::{
    RootLookupFuture, RootResolver,
};
use crate::modules::project_tree::infrastructure::di::project_tree_context::ProjectTreeContext;
use crate::modules::terminal::domain::models::workspace_target::WorkspaceTarget;
use crate::modules::terminal::infrastructure::adapters::outbound::workspace_path::resolver_workspace_path_adapter::{
    PathLookupFuture, PathResolver,
};
use crate::modules::terminal::infrastructure::di::terminal_context::TerminalContext;
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
use crate::modules::workspaces::domain::models::agents_file_option::AgentsFileOption;
use crate::modules::workspaces::domain::models::workspace::{Workspace, WorkspaceId};
use crate::modules::workspaces::domain::ports::outbound::agents_files_port::AgentsFilesPort;
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
    pub agent_profiles: AgentProfilesContext,
    pub agents_files: AgentsFilesContext,
    pub terminal: TerminalContext,
    pub project_tree: ProjectTreeContext,
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
        let projects: Arc<dyn ProjectLookupPort> = Arc::new(ProjectLookupBridge {
            get_workspace: get_workspace.clone(),
            list_workspaces: list_workspaces.clone(),
        });
        let terminal: TerminalContext = TerminalContext::new(
            workspace_path_resolver(get_workspace.clone()),
            config.home_dir.clone(),
        );
        let project_tree: ProjectTreeContext =
            ProjectTreeContext::new(workspace_root_resolver(get_workspace.clone()));
        let agents_files: AgentsFilesContext =
            AgentsFilesContext::new(pool.clone(), config.agents_files_dir.clone(), projects);
        let agent_settings: AgentSettingsContext = AgentSettingsContext::new(
            pool.clone(),
            AgentSettingsSettings {
                gentle_shell_bin: config.gentle_shell_bin.clone(),
                pi_bin: config.pi_bin.clone(),
                gentle_pi_config_home: config.gentle_pi_config_home.clone(),
            },
        );
        let model_options: Arc<dyn ModelOptionsPort> = Arc::new(ModelOptionsBridge {
            list_models: agent_settings.list_models.clone(),
        });
        let agent_profiles: AgentProfilesContext =
            AgentProfilesContext::new(AgentProfilesSettings {
                profiles_dir: config.gentle_pi_config_home.clone(),
                agents_dir: config.pi_agents_dir.clone(),
                model_options,
            });
        let launch_settings: Arc<dyn LaunchSettingsPort> = Arc::new(LaunchSettingsBridge {
            prepare_launch: agent_settings.prepare_launch.clone(),
        });
        let tasks: TasksContext = TasksContext::new(
            pool.clone(),
            TasksSettings {
                gentle_shell_bin: config.gentle_shell_bin.clone(),
                pi_bin: config.pi_bin.clone(),
                worktrees_dir: config.worktrees_dir.clone(),
                attachments_dir: config.attachments_dir.clone(),
            },
            lookup,
            launch_settings,
        );
        let workspace_tasks: Arc<dyn WorkspaceTasksPort> = Arc::new(WorkspaceTasksBridge {
            delete_workspace_tasks: tasks.delete_workspace_tasks.clone(),
        });
        let workspace_agents_files: Arc<dyn AgentsFilesPort> = Arc::new(AgentsFilesBridge {
            list: agents_files.list.clone(),
            install: agents_files.install.clone(),
        });
        let workspaces: WorkspacesContext = WorkspacesContext::new(
            pool.clone(),
            config.home_dir.clone(),
            workspace_tasks,
            workspace_agents_files,
        );
        Self {
            workspaces,
            tasks,
            agent_settings,
            agent_profiles,
            agents_files,
            terminal,
            project_tree,
        }
    }

    /// Registra cada contexto de feature para las server functions y el SSR.
    pub fn provide(&self) {
        provide_context(self.workspaces.clone());
        provide_context(self.tasks.clone());
        provide_context(self.agent_settings.clone());
        provide_context(self.agent_profiles.clone());
        provide_context(self.agents_files.clone());
        provide_context(self.terminal.clone());
        provide_context(self.project_tree.clone());
    }
}

/// Expone el directorio de un workspace al feature de terminal como una closure.
fn workspace_path_resolver(get_workspace: Arc<GetWorkspace>) -> PathResolver {
    Arc::new(move |id: uuid::Uuid| -> PathLookupFuture {
        let get_workspace: Arc<GetWorkspace> = get_workspace.clone();
        Box::pin(async move {
            match get_workspace.execute(WorkspaceId::from_uuid(id)).await {
                Ok(workspace) => Ok(Some(WorkspaceTarget {
                    name: workspace.name,
                    path: workspace.path,
                })),
                Err(WorkspaceError::NotFound(_)) => Ok(None),
                Err(error) => Err(error.to_string()),
            }
        })
    })
}

/// Expone el directorio de un workspace al feature de árbol de proyecto como una closure.
fn workspace_root_resolver(get_workspace: Arc<GetWorkspace>) -> RootResolver {
    Arc::new(move |id: uuid::Uuid| -> RootLookupFuture {
        let get_workspace: Arc<GetWorkspace> = get_workspace.clone();
        Box::pin(async move {
            match get_workspace.execute(WorkspaceId::from_uuid(id)).await {
                Ok(workspace) => Ok(Some(WorkspaceRoot {
                    name: workspace.name,
                    path: workspace.path,
                })),
                Err(WorkspaceError::NotFound(_)) => Ok(None),
                Err(error) => Err(error.to_string()),
            }
        })
    })
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

/// Expone los workspaces al feature de ficheros AGENTS.md como proyectos, a través de su
/// propio puerto.
pub struct ProjectLookupBridge {
    pub get_workspace: Arc<GetWorkspace>,
    pub list_workspaces: Arc<ListWorkspaces>,
}

#[async_trait::async_trait]
impl ProjectLookupPort for ProjectLookupBridge {
    async fn list(&self) -> Result<Vec<ProjectRef>, AgentsFileError> {
        self.list_workspaces
            .execute()
            .await
            .map(|all: Vec<Workspace>| all.into_iter().map(to_project_ref).collect())
            .map_err(|e: WorkspaceError| AgentsFileError::Repository(e.to_string()))
    }

    async fn find(&self, id: &str) -> Result<Option<ProjectRef>, AgentsFileError> {
        let Ok(workspace_id) = WorkspaceId::parse(id) else {
            return Ok(None);
        };
        match self.get_workspace.execute(workspace_id).await {
            Ok(workspace) => Ok(Some(to_project_ref(workspace))),
            Err(WorkspaceError::NotFound(_)) => Ok(None),
            Err(error) => Err(AgentsFileError::Repository(error.to_string())),
        }
    }
}

fn to_project_ref(workspace: Workspace) -> ProjectRef {
    ProjectRef {
        id: workspace.id.to_string(),
        name: workspace.name,
        path: workspace.path,
    }
}

/// Expone el catálogo de modelos de `agent_settings` al feature de perfiles a través de su
/// propio puerto; el id de modelo se compone como `provider/model`.
pub struct ModelOptionsBridge {
    pub list_models: Arc<ListModels>,
}

#[async_trait::async_trait]
impl ModelOptionsPort for ModelOptionsBridge {
    async fn list(&self) -> Result<Vec<ModelOption>, AgentProfilesError> {
        self.list_models
            .execute()
            .await
            .map(|models: Vec<ModelRef>| {
                models
                    .into_iter()
                    .map(|model: ModelRef| {
                        let id: String = format!("{}/{}", model.provider, model.model);
                        ModelOption {
                            label: id.clone(),
                            id,
                        }
                    })
                    .collect()
            })
            .map_err(|e: AgentSettingsError| AgentProfilesError::Io(e.to_string()))
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
            runtime: preferences
                .runtime
                .map(|r: AgentRuntime| r.as_str().to_string()),
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

/// Expone los ficheros AGENTS.md guardados al feature de workspaces a través de su
/// propio puerto.
pub struct AgentsFilesBridge {
    pub list: Arc<ListAgentsFiles>,
    pub install: Arc<InstallAgentsFile>,
}

#[async_trait::async_trait]
impl AgentsFilesPort for AgentsFilesBridge {
    async fn list_options(&self) -> Result<Vec<AgentsFileOption>, WorkspaceError> {
        self.list
            .execute()
            .await
            .map(|all: Vec<AgentsFile>| {
                all.into_iter()
                    .map(|file: AgentsFile| AgentsFileOption {
                        id: file.id.to_string(),
                        description: file.description,
                    })
                    .collect()
            })
            .map_err(to_workspace_error)
    }

    async fn install(
        &self,
        agents_file_id: &str,
        root: &std::path::Path,
        overwrite: bool,
    ) -> Result<(), WorkspaceError> {
        let id: AgentsFileId = AgentsFileId::parse(agents_file_id).map_err(to_workspace_error)?;
        self.install
            .execute(id, root, overwrite)
            .await
            .map(|_| ())
            .map_err(to_workspace_error)
    }
}

fn to_workspace_error(error: AgentsFileError) -> WorkspaceError {
    match error {
        AgentsFileError::NotFound(id) | AgentsFileError::InvalidId(id) => {
            WorkspaceError::AgentsFileNotFound(id)
        }
        AgentsFileError::TargetExists(path) => WorkspaceError::AgentsFileTargetExists(path),
        other => WorkspaceError::AgentsFile(other.to_string()),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn not_found_and_invalid_id_map_to_agents_file_not_found() {
        assert_eq!(
            to_workspace_error(AgentsFileError::NotFound("x".to_string())),
            WorkspaceError::AgentsFileNotFound("x".to_string())
        );
        assert_eq!(
            to_workspace_error(AgentsFileError::InvalidId("y".to_string())),
            WorkspaceError::AgentsFileNotFound("y".to_string())
        );
    }

    #[test]
    fn target_exists_maps_to_target_exists() {
        assert_eq!(
            to_workspace_error(AgentsFileError::TargetExists("/a/AGENTS.md".to_string())),
            WorkspaceError::AgentsFileTargetExists("/a/AGENTS.md".to_string())
        );
    }

    #[test]
    fn other_errors_keep_their_message() {
        assert_eq!(
            to_workspace_error(AgentsFileError::Storage("boom".to_string())),
            WorkspaceError::AgentsFile("error de almacenamiento: boom".to_string())
        );
    }
}
