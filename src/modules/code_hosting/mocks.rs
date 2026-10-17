use std::path::Path;

use mockall::mock;

use crate::modules::code_hosting::domain::errors::code_host_error::CodeHostError;
use crate::modules::code_hosting::domain::models::code_host_settings::CodeHostSettings;
use crate::modules::code_hosting::domain::models::merge_request::{
    MergeRequest, NewMergeRequest, Pipeline,
};
use crate::modules::code_hosting::domain::ports::outbound::code_host_client_port::CodeHostClientPort;
use crate::modules::code_hosting::domain::ports::outbound::code_host_settings_repository_port::CodeHostSettingsRepositoryPort;
use crate::modules::code_hosting::domain::ports::outbound::git_remote_port::GitRemotePort;
use crate::modules::code_hosting::domain::ports::outbound::task_source_port::{
    TaskSource, TaskSourcePort,
};

mock! {
    pub CodeHostSettingsRepository {}

    #[async_trait::async_trait]
    impl CodeHostSettingsRepositoryPort for CodeHostSettingsRepository {
        async fn get(&self) -> Result<Option<CodeHostSettings>, CodeHostError>;
        async fn save(&self, settings: &CodeHostSettings) -> Result<(), CodeHostError>;
    }
}

mock! {
    pub GitRemote {}

    #[async_trait::async_trait]
    impl GitRemotePort for GitRemote {
        async fn origin_url(&self, repo: &Path) -> Result<String, CodeHostError>;
    }
}

mock! {
    pub CodeHostClient {}

    #[async_trait::async_trait]
    impl CodeHostClientPort for CodeHostClient {
        async fn current_user(&self, settings: &CodeHostSettings) -> Result<String, CodeHostError>;
        async fn branch_exists(
            &self,
            settings: &CodeHostSettings,
            project: &str,
            branch: &str,
        ) -> Result<bool, CodeHostError>;
        async fn find_latest_merge_request(
            &self,
            settings: &CodeHostSettings,
            project: &str,
            source_branch: &str,
        ) -> Result<Option<MergeRequest>, CodeHostError>;
        async fn create_merge_request(
            &self,
            settings: &CodeHostSettings,
            project: &str,
            new: &NewMergeRequest,
        ) -> Result<MergeRequest, CodeHostError>;
        async fn get_merge_request(
            &self,
            settings: &CodeHostSettings,
            project: &str,
            iid: u64,
        ) -> Result<MergeRequest, CodeHostError>;
        async fn latest_pipeline(
            &self,
            settings: &CodeHostSettings,
            project: &str,
            iid: u64,
        ) -> Result<Option<Pipeline>, CodeHostError>;
        async fn merge_merge_request(
            &self,
            settings: &CodeHostSettings,
            project: &str,
            iid: u64,
        ) -> Result<MergeRequest, CodeHostError>;
    }
}

mock! {
    pub TaskSourceLookup {}

    #[async_trait::async_trait]
    impl TaskSourcePort for TaskSourceLookup {
        async fn find(&self, task_id: &str) -> Result<Option<TaskSource>, CodeHostError>;
    }
}
