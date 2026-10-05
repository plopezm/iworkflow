#[path = "../../common/mod.rs"]
mod common;

use std::path::{Path, PathBuf};

use iworkflow::modules::agent_settings::domain::models::profile_name::ProfileName;
use iworkflow::modules::agent_settings::domain::ports::outbound::profile_pin_port::ProfilePinPort;
use iworkflow::modules::agent_settings::infrastructure::adapters::outbound::git::git_profile_pin_adapter::GitProfilePinAdapter;

fn pin_path(common_dir: &Path) -> PathBuf {
    common_dir.join("gentle-ai/profile-pin.json")
}

#[tokio::test]
async fn writes_pin_file_with_expected_content() {
    let (_dir, root) = common::git_repo();
    let profile: ProfileName = ProfileName::parse("Claude").expect("profile");
    let adapter: GitProfilePinAdapter = GitProfilePinAdapter::new();

    adapter.apply(&root, Some(&profile)).await.expect("apply");

    let content: String = std::fs::read_to_string(pin_path(&root.join(".git"))).expect("pin file");
    assert_eq!(
        content,
        "{\n  \"kind\": \"gentle-pi.agent_model_profile_pin\",\n  \"version\": 1,\n  \"profile\": \"Claude\"\n}\n"
    );
}

#[tokio::test]
async fn none_removes_existing_pin_file() {
    let (_dir, root) = common::git_repo();
    let profile: ProfileName = ProfileName::parse("Claude").expect("profile");
    let adapter: GitProfilePinAdapter = GitProfilePinAdapter::new();
    adapter.apply(&root, Some(&profile)).await.expect("apply");

    adapter.apply(&root, None).await.expect("apply none");

    assert!(!pin_path(&root.join(".git")).exists());
}

#[tokio::test]
async fn none_on_missing_pin_file_is_not_an_error() {
    let (_dir, root) = common::git_repo();
    let adapter: GitProfilePinAdapter = GitProfilePinAdapter::new();

    let result: Result<(), _> = adapter.apply(&root, None).await;

    assert!(result.is_ok());
}

#[tokio::test]
async fn resolves_common_dir_from_a_subfolder() {
    let (_dir, root) = common::git_repo();
    let sub: PathBuf = root.join("sub");
    std::fs::create_dir(&sub).expect("mkdir");
    let profile: ProfileName = ProfileName::parse("Claude").expect("profile");
    let adapter: GitProfilePinAdapter = GitProfilePinAdapter::new();

    adapter.apply(&sub, Some(&profile)).await.expect("apply");

    assert!(pin_path(&root.join(".git")).exists());
}
