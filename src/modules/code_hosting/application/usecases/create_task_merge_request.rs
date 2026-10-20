use std::sync::Arc;

use crate::modules::code_hosting::application::services::task_project_resolver::{
    ResolvedTask, TaskProjectResolver,
};
use crate::modules::code_hosting::domain::errors::code_host_error::CodeHostError;
use crate::modules::code_hosting::domain::models::merge_request::{
    MergeRequest, MergeRequestStatus, NewMergeRequest,
};
use crate::modules::code_hosting::domain::ports::outbound::git_remote_port::GitRemotePort;

#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct CreateTaskMergeRequestInput {
    pub task_id: String,
    /// Vacío o `None` usa el título de la tarea.
    pub title: Option<String>,
    /// Vacío o `None` usa el prompt de la tarea.
    pub description: Option<String>,
}

/// Abre un MR de la rama de la tarea hacia su rama base (o reutiliza el abierto).
/// Si la rama aún no existe en `origin`, la publica antes de abrir el MR.
pub struct CreateTaskMergeRequest {
    resolver: Arc<TaskProjectResolver>,
    remote: Arc<dyn GitRemotePort>,
}

impl CreateTaskMergeRequest {
    pub fn new(resolver: Arc<TaskProjectResolver>, remote: Arc<dyn GitRemotePort>) -> Self {
        Self { resolver, remote }
    }

    pub async fn execute(
        &self,
        input: CreateTaskMergeRequestInput,
    ) -> Result<MergeRequestStatus, CodeHostError> {
        let resolved: ResolvedTask = self.resolver.resolve(&input.task_id).await?;
        let source = &resolved.source;
        let existing: Option<MergeRequest> = resolved
            .client
            .find_latest_merge_request(&resolved.settings, &resolved.project, &source.branch)
            .await?;
        let merge_request: MergeRequest = match existing {
            Some(open) if open.is_open() => open,
            _ => {
                // La API del proveedor espera el nombre de la rama en el servidor, no la
                // referencia remota local (`origin/develop`) con la que se creó la tarea.
                let target: String = remote_branch_name(&source.base_branch).to_string();
                if target == source.branch {
                    return Err(CodeHostError::SameSourceAndTarget(target));
                }
                let base_published: bool = resolved
                    .client
                    .branch_exists(&resolved.settings, &resolved.project, &target)
                    .await?;
                if !base_published {
                    return Err(CodeHostError::BaseBranchNotPublished(target));
                }
                let published: bool = resolved
                    .client
                    .branch_exists(&resolved.settings, &resolved.project, &source.branch)
                    .await?;
                if !published {
                    self.remote
                        .publish_branch(
                            &source.repo_path,
                            &source.branch,
                            &resolved.settings,
                            &resolved.project,
                        )
                        .await?;
                }
                let new: NewMergeRequest = NewMergeRequest {
                    source_branch: source.branch.clone(),
                    target_branch: target,
                    title: non_blank(input.title).unwrap_or_else(|| source.title.clone()),
                    description: non_blank(input.description)
                        .unwrap_or_else(|| source.description.clone()),
                };
                resolved
                    .client
                    .create_merge_request(&resolved.settings, &resolved.project, &new)
                    .await?
            }
        };
        self.resolver.status_of(&resolved, merge_request).await
    }
}

/// Prefijo de la referencia remota con la que las tareas pueden elegir su rama de partida.
const ORIGIN_PREFIX: &str = "origin/";

/// `origin/develop` → `develop`; una rama sin prefijo se devuelve tal cual.
fn remote_branch_name(branch: &str) -> &str {
    let short: &str = branch.strip_prefix("refs/remotes/").unwrap_or(branch);
    short.strip_prefix(ORIGIN_PREFIX).unwrap_or(short)
}

fn non_blank(value: Option<String>) -> Option<String> {
    value
        .map(|v: String| v.trim().to_string())
        .filter(|v: &String| !v.is_empty())
}

#[cfg(test)]
mod tests {
    use std::path::Path;

    use super::*;
    use crate::modules::code_hosting::application::services::task_project_resolver::TaskProjectResolver;
    use crate::modules::code_hosting::application::test_support::{self, TASK_ID};
    use crate::modules::code_hosting::domain::models::code_host_settings::CodeHostSettings;
    use crate::modules::code_hosting::domain::ports::outbound::task_source_port::TaskSource;
    use crate::modules::code_hosting::mocks::{MockCodeHostClient, MockGitRemote};

    fn input(title: Option<&str>) -> CreateTaskMergeRequestInput {
        CreateTaskMergeRequestInput {
            task_id: TASK_ID.to_string(),
            title: title.map(str::to_string),
            description: None,
        }
    }

    fn remote() -> Arc<dyn GitRemotePort> {
        let mut remote: MockGitRemote = MockGitRemote::new();
        remote.expect_publish_branch().never();
        Arc::new(remote)
    }

    fn client_without_mr() -> MockCodeHostClient {
        let mut client: MockCodeHostClient = MockCodeHostClient::new();
        client
            .expect_find_latest_merge_request()
            .returning(|_, _, _| Ok(None));
        client
            .expect_latest_pipeline()
            .returning(|_, _, _| Ok(None));
        client
    }

    #[tokio::test]
    async fn creates_mr_from_task_branch_to_base_with_task_defaults() {
        let mut client: MockCodeHostClient = client_without_mr();
        client.expect_branch_exists().returning(|_, _, _| Ok(true));
        client
            .expect_create_merge_request()
            .withf(|_, _, new: &NewMergeRequest| {
                new.source_branch == "pi/feature"
                    && new.target_branch == "main"
                    && new.title == "Mi tarea"
                    && new.description == "Prompt de la tarea"
            })
            .times(1)
            .returning(|_, _, _| Ok(test_support::merge_request(9, "opened")));

        let status: MergeRequestStatus =
            CreateTaskMergeRequest::new(test_support::resolver(client), remote())
                .execute(input(Some("  ")))
                .await
                .expect("create");

        assert_eq!(status.merge_request.iid, 9);
    }

    #[tokio::test]
    async fn uses_custom_title_when_given() {
        let mut client: MockCodeHostClient = client_without_mr();
        client.expect_branch_exists().returning(|_, _, _| Ok(true));
        client
            .expect_create_merge_request()
            .withf(|_, _, new: &NewMergeRequest| new.title == "Título propio")
            .returning(|_, _, _| Ok(test_support::merge_request(9, "opened")));

        CreateTaskMergeRequest::new(test_support::resolver(client), remote())
            .execute(input(Some("Título propio")))
            .await
            .expect("create");
    }

    #[tokio::test]
    async fn reuses_open_merge_request_without_creating_another() {
        let mut client: MockCodeHostClient = MockCodeHostClient::new();
        client
            .expect_find_latest_merge_request()
            .returning(|_, _, _| Ok(Some(test_support::merge_request(4, "opened"))));
        client
            .expect_latest_pipeline()
            .returning(|_, _, _| Ok(None));
        client.expect_create_merge_request().never();

        let status: MergeRequestStatus =
            CreateTaskMergeRequest::new(test_support::resolver(client), remote())
                .execute(input(None))
                .await
                .expect("reuse");

        assert_eq!(status.merge_request.iid, 4);
    }

    /// Solo la rama de la tarea falta en el servidor; la base sí está publicada.
    fn only_base_is_published(client: &mut MockCodeHostClient) {
        client
            .expect_branch_exists()
            .returning(|_, _, branch: &str| Ok(branch == "main"));
    }

    #[tokio::test]
    async fn unpublished_branch_is_pushed_before_creating_the_mr() {
        let mut client: MockCodeHostClient = client_without_mr();
        only_base_is_published(&mut client);
        client
            .expect_create_merge_request()
            .times(1)
            .returning(|_, _, _| Ok(test_support::merge_request(5, "opened")));
        let mut remote: MockGitRemote = MockGitRemote::new();
        remote
            .expect_publish_branch()
            .withf(
                |repo: &Path, branch: &str, settings: &CodeHostSettings, project: &str| {
                    repo == Path::new("/repo")
                        && branch == "pi/feature"
                        && settings.token() == "tok"
                        && project == "grupo/proyecto"
                },
            )
            .times(1)
            .returning(|_, _, _, _| Ok(()));

        let status: MergeRequestStatus =
            CreateTaskMergeRequest::new(test_support::resolver(client), Arc::new(remote))
                .execute(input(None))
                .await
                .expect("create");

        assert_eq!(status.merge_request.iid, 5);
    }

    #[tokio::test]
    async fn failed_push_is_reported_and_nothing_is_created() {
        let mut client: MockCodeHostClient = client_without_mr();
        only_base_is_published(&mut client);
        client.expect_create_merge_request().never();
        let mut remote: MockGitRemote = MockGitRemote::new();
        remote
            .expect_publish_branch()
            .returning(|_, branch: &str, _, _| {
                Err(CodeHostError::BranchPublishFailed {
                    branch: branch.to_string(),
                    reason: "denied".to_string(),
                })
            });

        let result: Result<MergeRequestStatus, CodeHostError> =
            CreateTaskMergeRequest::new(test_support::resolver(client), Arc::new(remote))
                .execute(input(None))
                .await;

        assert_eq!(
            result.unwrap_err(),
            CodeHostError::BranchPublishFailed {
                branch: "pi/feature".to_string(),
                reason: "denied".to_string()
            }
        );
    }

    #[tokio::test]
    async fn merged_previous_mr_does_not_block_a_new_one() {
        let mut client: MockCodeHostClient = MockCodeHostClient::new();
        client
            .expect_find_latest_merge_request()
            .returning(|_, _, _| Ok(Some(test_support::merge_request(2, "merged"))));
        client
            .expect_latest_pipeline()
            .returning(|_, _, _| Ok(None));
        client.expect_branch_exists().returning(|_, _, _| Ok(true));
        client
            .expect_create_merge_request()
            .times(1)
            .returning(|_, _, _| Ok(test_support::merge_request(3, "opened")));

        let status: MergeRequestStatus =
            CreateTaskMergeRequest::new(test_support::resolver(client), remote())
                .execute(input(None))
                .await
                .expect("create");

        assert_eq!(status.merge_request.iid, 3);
    }

    /// Fuente con la base indicada; el resto, igual que la tarea de prueba.
    fn resolver_with_base(
        base: &'static str,
        client: MockCodeHostClient,
    ) -> Arc<TaskProjectResolver> {
        test_support::resolver_for_source(
            TaskSource {
                base_branch: base.to_string(),
                ..test_support::source()
            },
            client,
        )
    }

    #[tokio::test]
    async fn remote_tracking_base_is_sent_without_the_remote_prefix() {
        let mut client: MockCodeHostClient = client_without_mr();
        client.expect_branch_exists().returning(|_, _, _| Ok(true));
        client
            .expect_create_merge_request()
            .withf(|_, _, new: &NewMergeRequest| new.target_branch == "develop")
            .times(1)
            .returning(|_, _, _| Ok(test_support::merge_request(6, "opened")));

        CreateTaskMergeRequest::new(resolver_with_base("origin/develop", client), remote())
            .execute(input(None))
            .await
            .expect("create");
    }

    #[tokio::test]
    async fn base_with_nested_name_keeps_everything_after_the_remote() {
        let mut client: MockCodeHostClient = client_without_mr();
        client.expect_branch_exists().returning(|_, _, _| Ok(true));
        client
            .expect_create_merge_request()
            .withf(|_, _, new: &NewMergeRequest| new.target_branch == "release/1.0")
            .times(1)
            .returning(|_, _, _| Ok(test_support::merge_request(6, "opened")));

        CreateTaskMergeRequest::new(resolver_with_base("origin/release/1.0", client), remote())
            .execute(input(None))
            .await
            .expect("create");
    }

    #[tokio::test]
    async fn unpublished_base_is_reported_and_nothing_is_pushed_or_created() {
        let mut client: MockCodeHostClient = client_without_mr();
        client
            .expect_branch_exists()
            .withf(|_, _, branch: &str| branch == "develop")
            .returning(|_, _, _| Ok(false));
        client.expect_create_merge_request().never();

        let result: Result<MergeRequestStatus, CodeHostError> =
            CreateTaskMergeRequest::new(resolver_with_base("origin/develop", client), remote())
                .execute(input(None))
                .await;

        assert_eq!(
            result.unwrap_err(),
            CodeHostError::BaseBranchNotPublished("develop".to_string())
        );
    }

    #[tokio::test]
    async fn task_branch_equal_to_the_base_is_rejected() {
        let mut client: MockCodeHostClient = client_without_mr();
        client.expect_create_merge_request().never();

        let result: Result<MergeRequestStatus, CodeHostError> =
            CreateTaskMergeRequest::new(resolver_with_base("origin/pi/feature", client), remote())
                .execute(input(None))
                .await;

        assert_eq!(
            result.unwrap_err(),
            CodeHostError::SameSourceAndTarget("pi/feature".to_string())
        );
    }
}
