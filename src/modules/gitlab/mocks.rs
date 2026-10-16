use std::path::Path;

use mockall::mock;

use crate::modules::gitlab::domain::errors::gitlab_error::GitLabError;
use crate::modules::gitlab::domain::models::gitlab_settings::GitLabSettings;
use crate::modules::gitlab::domain::models::merge_request::{
    MergeRequest, NewMergeRequest, Pipeline,
};
use crate::modules::gitlab::domain::ports::outbound::git_remote_port::GitRemotePort;
use crate::modules::gitlab::domain::ports::outbound::gitlab_client_port::GitLabClientPort;
use crate::modules::gitlab::domain::ports::outbound::gitlab_settings_repository_port::GitLabSettingsRepositoryPort;
use crate::modules::gitlab::domain::ports::outbound::task_source_port::{
    TaskSource, TaskSourcePort,
};

mock! {
    pub GitLabSettingsRepository {}

    #[async_trait::async_trait]
    impl GitLabSettingsRepositoryPort for GitLabSettingsRepository {
        async fn get(&self) -> Result<Option<GitLabSettings>, GitLabError>;
        async fn save(&self, settings: &GitLabSettings) -> Result<(), GitLabError>;
    }
}

mock! {
    pub GitRemote {}

    #[async_trait::async_trait]
    impl GitRemotePort for GitRemote {
        async fn origin_url(&self, repo: &Path) -> Result<String, GitLabError>;
    }
}

mock! {
    pub GitLabClient {}

    #[async_trait::async_trait]
    impl GitLabClientPort for GitLabClient {
        async fn current_user(&self, settings: &GitLabSettings) -> Result<String, GitLabError>;
        async fn branch_exists(
            &self,
            settings: &GitLabSettings,
            project: &str,
            branch: &str,
        ) -> Result<bool, GitLabError>;
        async fn find_latest_merge_request(
            &self,
            settings: &GitLabSettings,
            project: &str,
            source_branch: &str,
        ) -> Result<Option<MergeRequest>, GitLabError>;
        async fn create_merge_request(
            &self,
            settings: &GitLabSettings,
            project: &str,
            new: &NewMergeRequest,
        ) -> Result<MergeRequest, GitLabError>;
        async fn get_merge_request(
            &self,
            settings: &GitLabSettings,
            project: &str,
            iid: u64,
        ) -> Result<MergeRequest, GitLabError>;
        async fn latest_pipeline(
            &self,
            settings: &GitLabSettings,
            project: &str,
            iid: u64,
        ) -> Result<Option<Pipeline>, GitLabError>;
        async fn merge_merge_request(
            &self,
            settings: &GitLabSettings,
            project: &str,
            iid: u64,
        ) -> Result<MergeRequest, GitLabError>;
    }
}

mock! {
    pub TaskSourceLookup {}

    #[async_trait::async_trait]
    impl TaskSourcePort for TaskSourceLookup {
        async fn find(&self, task_id: &str) -> Result<Option<TaskSource>, GitLabError>;
    }
}
