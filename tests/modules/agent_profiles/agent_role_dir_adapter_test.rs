use iworkflow::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;
use iworkflow::modules::agent_profiles::domain::models::agent_role::{
    AgentRole, NewAgentRole, RoleName,
};
use iworkflow::modules::agent_profiles::domain::ports::outbound::agent_role_port::AgentRolePort;
use iworkflow::modules::agent_profiles::infrastructure::adapters::outbound::roles::agent_role_dir_adapter::AgentRoleDirAdapter;

fn new_role(name: &str, description: &str, prompt: &str) -> NewAgentRole {
    NewAgentRole {
        name: RoleName::parse(name).expect("name"),
        description: description.to_string(),
        prompt: prompt.to_string(),
        model: None,
        thinking: None,
    }
}

#[tokio::test]
async fn missing_dir_lists_empty() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let adapter: AgentRoleDirAdapter = AgentRoleDirAdapter::new(dir.path().join("nope"));
    assert!(adapter.list().await.expect("list").is_empty());
}

#[tokio::test]
async fn create_then_list_sorted() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let adapter: AgentRoleDirAdapter = AgentRoleDirAdapter::new(dir.path().join("agents"));
    adapter
        .create(&new_role("zeta", "Last one", "Do z.\n"))
        .await
        .expect("zeta");
    adapter
        .create(&new_role("alpha", "First", "Do a."))
        .await
        .expect("alpha");

    let roles: Vec<AgentRole> = adapter.list().await.expect("list");
    let names: Vec<&str> = roles.iter().map(|r: &AgentRole| r.name.as_str()).collect();
    assert_eq!(names, vec!["alpha", "zeta"]);
    assert_eq!(roles[1].description, "Last one");
    assert_eq!(roles[0].model, None);

    let raw: String = std::fs::read_to_string(dir.path().join("agents/alpha.md")).expect("read");
    assert!(raw.starts_with("---\nname: alpha\ndescription: First\n---\n"));
    assert!(raw.contains("Do a."));
}

#[tokio::test]
async fn create_never_overwrites() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let adapter: AgentRoleDirAdapter = AgentRoleDirAdapter::new(dir.path().to_path_buf());
    adapter
        .create(&new_role("dup", "one", "first"))
        .await
        .expect("first");
    let second: Result<(), AgentProfilesError> =
        adapter.create(&new_role("dup", "two", "second")).await;
    assert_eq!(
        second,
        Err(AgentProfilesError::RoleAlreadyExists("dup".to_string()))
    );
    let raw: String = std::fs::read_to_string(dir.path().join("dup.md")).expect("read");
    assert!(raw.contains("first") && !raw.contains("second"));
}

#[tokio::test]
async fn create_rejects_newline_in_description() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let adapter: AgentRoleDirAdapter = AgentRoleDirAdapter::new(dir.path().to_path_buf());
    let result: Result<(), AgentProfilesError> = adapter
        .create(&new_role("bad", "line1\nmodel: evil", "p"))
        .await;
    assert!(result.is_err());
    assert!(!dir.path().join("bad.md").exists());
}

#[tokio::test]
async fn list_parses_existing_files_and_skips_non_md() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    std::fs::write(
        dir.path().join("worker.md"),
        "---\nname: gentle-worker\ndescription: Does work\nmodel: p/m\nthinking: low\ntools:\n  - read\n---\n\nBody\n",
    )
    .expect("write");
    std::fs::write(
        dir.path().join("stem-only.md"),
        "---\ndescription: \"Quoted\"\n---\nx",
    )
    .expect("write");
    std::fs::write(dir.path().join("notes.txt"), "ignored").expect("write");
    let adapter: AgentRoleDirAdapter = AgentRoleDirAdapter::new(dir.path().to_path_buf());
    let roles: Vec<AgentRole> = adapter.list().await.expect("list");
    assert_eq!(roles.len(), 2);
    assert_eq!(roles[0].name.as_str(), "gentle-worker");
    assert_eq!(roles[0].model.as_deref(), Some("p/m"));
    assert_eq!(roles[0].thinking.as_deref(), Some("low"));
    assert_eq!(roles[1].name.as_str(), "stem-only");
    assert_eq!(roles[1].description, "Quoted");
}

#[tokio::test]
async fn list_skips_unreadable_entries() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    std::fs::write(dir.path().join("good.md"), "---\nname: good\n---\nbody\n").expect("good");
    std::fs::write(dir.path().join("binary.md"), [0xff_u8, 0xfe, 0x00, 0xc3]).expect("binary");
    std::fs::write(dir.path().join("bad name.md"), "no frontmatter").expect("bad");
    std::os::unix::fs::symlink(dir.path().join("gone"), dir.path().join("dangling.md"))
        .expect("symlink");
    let adapter: AgentRoleDirAdapter = AgentRoleDirAdapter::new(dir.path().to_path_buf());
    let roles: Vec<AgentRole> = adapter.list().await.expect("list");
    let names: Vec<&str> = roles.iter().map(|r: &AgentRole| r.name.as_str()).collect();
    assert_eq!(names, vec!["good"]);
}

#[tokio::test]
async fn create_with_model_and_thinking_persists_and_lists_them() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let agents: std::path::PathBuf = dir.path().join("agents");
    let adapter: AgentRoleDirAdapter = AgentRoleDirAdapter::new(agents.clone());
    let mut role: NewAgentRole = new_role("tuned", "Tuned role", "Body.");
    role.model = Some("anthropic/claude".to_string());
    role.thinking = Some("high".to_string());
    adapter.create(&role).await.expect("create");

    let content: String = std::fs::read_to_string(agents.join("tuned.md")).expect("read");
    assert!(content.contains("\nmodel: anthropic/claude\n"));
    assert!(content.contains("\nthinking: high\n"));
    let roles: Vec<AgentRole> = adapter.list().await.expect("list");
    assert_eq!(roles[0].model.as_deref(), Some("anthropic/claude"));
    assert_eq!(roles[0].thinking.as_deref(), Some("high"));
}

#[tokio::test]
async fn create_without_model_and_thinking_writes_no_such_lines() {
    let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
    let agents: std::path::PathBuf = dir.path().join("agents");
    let adapter: AgentRoleDirAdapter = AgentRoleDirAdapter::new(agents.clone());
    adapter
        .create(&new_role("plain", "Plain", "Body."))
        .await
        .expect("create");

    let content: String = std::fs::read_to_string(agents.join("plain.md")).expect("read");
    assert!(!content.contains("model:"));
    assert!(!content.contains("thinking:"));
}
