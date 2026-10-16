use std::sync::Arc;

use crate::core::db::pool::DbPool;
use crate::modules::jira::application::usecases::add_issue_comment::AddIssueComment;
use crate::modules::jira::application::usecases::convert_issue::ConvertIssue;
use crate::modules::jira::application::usecases::create_lane::CreateLane;
use crate::modules::jira::application::usecases::delete_connection::DeleteConnection;
use crate::modules::jira::application::usecases::delete_lane::DeleteLane;
use crate::modules::jira::application::usecases::get_connection::GetConnection;
use crate::modules::jira::application::usecases::get_issue::GetIssue;
use crate::modules::jira::application::usecases::list_conversion_branches::ListConversionBranches;
use crate::modules::jira::application::usecases::list_issue_comments::ListIssueComments;
use crate::modules::jira::application::usecases::list_issue_transitions::ListIssueTransitions;
use crate::modules::jira::application::usecases::list_issues::ListIssues;
use crate::modules::jira::application::usecases::prepare_conversion::PrepareConversion;
use crate::modules::jira::application::usecases::save_connection::SaveConnection;
use crate::modules::jira::application::usecases::set_issue_ignored::SetIssueIgnored;
use crate::modules::jira::application::usecases::set_lane_collapsed::SetLaneCollapsed;
use crate::modules::jira::application::usecases::sync_issues::SyncIssues;
use crate::modules::jira::application::usecases::transition_issue::TransitionIssue;
use crate::modules::jira::application::usecases::update_lane::UpdateLane;
use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::ports::outbound::jira_client_port::JiraClientPort;
use crate::modules::jira::domain::ports::outbound::jira_connection_repository_port::JiraConnectionRepositoryPort;
use crate::modules::jira::domain::ports::outbound::jira_issue_repository_port::JiraIssueRepositoryPort;
use crate::modules::jira::domain::ports::outbound::jira_lane_repository_port::JiraLaneRepositoryPort;
use crate::modules::jira::domain::ports::outbound::task_creation_port::TaskCreationPort;
use crate::modules::jira::domain::ports::outbound::workspace_catalog_port::WorkspaceCatalogPort;
use crate::modules::jira::infrastructure::adapters::outbound::api::reqwest_jira_client_adapter::ReqwestJiraClientAdapter;
use crate::modules::jira::infrastructure::adapters::outbound::persistence::jira_connection_repository_adapter::JiraConnectionRepositoryAdapter;
use crate::modules::jira::infrastructure::adapters::outbound::persistence::jira_issue_repository_adapter::JiraIssueRepositoryAdapter;
use crate::modules::jira::infrastructure::adapters::outbound::persistence::jira_lane_repository_adapter::JiraLaneRepositoryAdapter;

#[derive(Clone)]
pub struct JiraContext {
    pub get_connection: Arc<GetConnection>,
    pub save_connection: Arc<SaveConnection>,
    pub delete_connection: Arc<DeleteConnection>,
    pub sync_issues: Arc<SyncIssues>,
    pub list_issues: Arc<ListIssues>,
    pub create_lane: Arc<CreateLane>,
    pub update_lane: Arc<UpdateLane>,
    pub delete_lane: Arc<DeleteLane>,
    pub set_lane_collapsed: Arc<SetLaneCollapsed>,
    pub get_issue: Arc<GetIssue>,
    pub list_comments: Arc<ListIssueComments>,
    pub list_transitions: Arc<ListIssueTransitions>,
    pub add_comment: Arc<AddIssueComment>,
    pub transition_issue: Arc<TransitionIssue>,
    pub set_ignored: Arc<SetIssueIgnored>,
    pub prepare_conversion: Arc<PrepareConversion>,
    pub list_branches: Arc<ListConversionBranches>,
    pub convert_issue: Arc<ConvertIssue>,
}

impl JiraContext {
    /// `tasks` and `workspaces` are the other features' capabilities, bridged by the
    /// composition root.
    pub fn new(
        pool: DbPool,
        tasks: Arc<dyn TaskCreationPort>,
        workspaces: Arc<dyn WorkspaceCatalogPort>,
    ) -> Result<Self, JiraError> {
        let connections: Arc<dyn JiraConnectionRepositoryPort> =
            Arc::new(JiraConnectionRepositoryAdapter::new(pool.clone()));
        let issues: Arc<dyn JiraIssueRepositoryPort> =
            Arc::new(JiraIssueRepositoryAdapter::new(pool.clone()));
        let lanes: Arc<dyn JiraLaneRepositoryPort> = Arc::new(JiraLaneRepositoryAdapter::new(pool));
        let client: Arc<dyn JiraClientPort> = Arc::new(ReqwestJiraClientAdapter::new()?);
        let sync_issues: Arc<SyncIssues> = Arc::new(SyncIssues::new(
            connections.clone(),
            issues.clone(),
            lanes.clone(),
            client.clone(),
        ));
        Ok(Self {
            get_connection: Arc::new(GetConnection::new(connections.clone())),
            save_connection: Arc::new(SaveConnection::new(
                connections.clone(),
                issues.clone(),
                client.clone(),
            )),
            delete_connection: Arc::new(DeleteConnection::new(connections.clone(), issues.clone())),
            create_lane: Arc::new(CreateLane::new(lanes.clone(), sync_issues.clone())),
            update_lane: Arc::new(UpdateLane::new(lanes.clone(), sync_issues.clone())),
            delete_lane: Arc::new(DeleteLane::new(lanes.clone())),
            set_lane_collapsed: Arc::new(SetLaneCollapsed::new(lanes.clone())),
            sync_issues,
            list_issues: Arc::new(ListIssues::new(issues.clone(), lanes)),
            get_issue: Arc::new(GetIssue::new(issues.clone())),
            list_comments: Arc::new(ListIssueComments::new(connections.clone(), client.clone())),
            list_transitions: Arc::new(ListIssueTransitions::new(
                connections.clone(),
                client.clone(),
            )),
            add_comment: Arc::new(AddIssueComment::new(connections.clone(), client.clone())),
            transition_issue: Arc::new(TransitionIssue::new(connections, issues.clone(), client)),
            set_ignored: Arc::new(SetIssueIgnored::new(issues.clone())),
            prepare_conversion: Arc::new(PrepareConversion::new(
                issues.clone(),
                workspaces.clone(),
            )),
            list_branches: Arc::new(ListConversionBranches::new(workspaces)),
            convert_issue: Arc::new(ConvertIssue::new(issues, tasks)),
        })
    }
}
