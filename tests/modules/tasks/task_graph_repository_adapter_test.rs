#[path = "../../common/mod.rs"]
mod common;

use std::path::PathBuf;

use chrono::{DateTime, SubsecRound, Utc};
use iworkflow::modules::tasks::domain::errors::task_error::TaskError;
use iworkflow::modules::tasks::domain::models::resource_limits::ResourceLimits;
use iworkflow::modules::tasks::domain::models::subagent_activity::SubagentActivity;
use iworkflow::modules::tasks::domain::models::task::{Task, TaskId, TaskStatus};
use iworkflow::modules::tasks::domain::models::task_graph::{
    DependencyTrigger, NodePosition, TaskDependency,
};
use iworkflow::modules::tasks::domain::models::task_priority::TaskPriority;
use iworkflow::modules::tasks::domain::ports::outbound::task_graph_repository_port::TaskGraphRepositoryPort;
use iworkflow::modules::tasks::domain::ports::outbound::task_repository_port::TaskRepositoryPort;
use iworkflow::modules::tasks::infrastructure::adapters::outbound::persistence::task_graph_repository_adapter::TaskGraphRepositoryAdapter;
use iworkflow::modules::tasks::infrastructure::adapters::outbound::persistence::task_repository_adapter::TaskRepositoryAdapter;
use iworkflow::modules::workspaces::domain::models::workspace::{Workspace, WorkspaceId};
use iworkflow::modules::workspaces::domain::ports::outbound::workspace_repository_port::WorkspaceRepositoryPort;
use iworkflow::modules::workspaces::infrastructure::adapters::outbound::persistence::workspace_repository_adapter::WorkspaceRepositoryAdapter;

async fn workspace(db: &common::TestDb, name: &str) -> String {
    let workspace: Workspace = Workspace {
        id: WorkspaceId::new(),
        name: name.to_string(),
        path: PathBuf::from(format!("/repos/{name}")),
        base_branch: "main".to_string(),
        created_at: Utc::now(),
    };
    WorkspaceRepositoryAdapter::new(db.pool.clone())
        .save(&workspace)
        .await
        .expect("workspace");
    workspace.id.to_string()
}

async fn backlog_task(db: &common::TestDb, workspace_id: &str, title: &str) -> TaskId {
    let now: DateTime<Utc> = Utc::now().trunc_subsecs(6);
    let task: Task = Task {
        id: TaskId::new(),
        workspace_id: workspace_id.to_string(),
        title: title.to_string(),
        prompt: "haz algo".to_string(),
        branch: String::new(),
        base_branch: "main".to_string(),
        worktree_path: PathBuf::new(),
        status: TaskStatus::Backlog,
        priority: TaskPriority::default(),
        limits: ResourceLimits::default(),
        usage_limit: Default::default(),
        error: None,
        pending_dialogs: Vec::new(),
        subagents: SubagentActivity::default(),
        diff_range: None,
        created_at: now,
        updated_at: now,
    };
    TaskRepositoryAdapter::new(db.pool.clone())
        .insert(&task)
        .await
        .expect("task");
    task.id
}

fn edge(workspace_id: &str, predecessor: TaskId, successor: TaskId) -> TaskDependency {
    TaskDependency {
        predecessor,
        successor,
        workspace_id: workspace_id.to_string(),
        trigger: DependencyTrigger::Merged,
    }
}

#[tokio::test(flavor = "multi_thread")]
async fn stores_lists_and_removes_dependencies() {
    let db: common::TestDb = common::test_db().await;
    let ws: String = workspace(&db, "demo").await;
    let other_ws: String = workspace(&db, "other").await;
    let (a, b, c): (TaskId, TaskId, TaskId) = (
        backlog_task(&db, &ws, "a").await,
        backlog_task(&db, &ws, "b").await,
        backlog_task(&db, &ws, "c").await,
    );
    let foreign: TaskId = backlog_task(&db, &other_ws, "x").await;
    let graph: TaskGraphRepositoryAdapter = TaskGraphRepositoryAdapter::new(db.pool.clone());

    graph
        .insert_dependency(&edge(&ws, a, b))
        .await
        .expect("a->b");
    graph
        .insert_dependency(&edge(&ws, a, c))
        .await
        .expect("a->c");
    graph
        .insert_dependency(&edge(&other_ws, foreign, foreign))
        .await
        .expect_err("self edge");

    let listed: Vec<TaskDependency> = graph
        .list_dependencies(Some(ws.clone()))
        .await
        .expect("list");
    assert_eq!(listed.len(), 2);
    assert!(listed.contains(&edge(&ws, a, b)));
    assert!(
        graph
            .list_dependencies(Some(other_ws))
            .await
            .expect("other")
            .is_empty()
    );

    assert!(graph.remove_dependency(a, b).await.expect("remove"));
    assert!(!graph.remove_dependency(a, b).await.expect("remove twice"));
    assert_eq!(graph.list_dependencies(None).await.expect("all").len(), 1);
}

#[tokio::test(flavor = "multi_thread")]
async fn rejects_duplicates_and_cycles_without_changing_stored_edges() {
    let db: common::TestDb = common::test_db().await;
    let ws: String = workspace(&db, "demo").await;
    let (a, b, c): (TaskId, TaskId, TaskId) = (
        backlog_task(&db, &ws, "a").await,
        backlog_task(&db, &ws, "b").await,
        backlog_task(&db, &ws, "c").await,
    );
    let graph: TaskGraphRepositoryAdapter = TaskGraphRepositoryAdapter::new(db.pool.clone());
    graph
        .insert_dependency(&edge(&ws, a, b))
        .await
        .expect("a->b");
    graph
        .insert_dependency(&edge(&ws, b, c))
        .await
        .expect("b->c");

    assert_eq!(
        graph.insert_dependency(&edge(&ws, a, b)).await,
        Err(TaskError::DuplicateDependency)
    );
    assert_eq!(
        graph.insert_dependency(&edge(&ws, c, a)).await,
        Err(TaskError::DependencyCycle)
    );
    assert_eq!(
        graph.insert_dependency(&edge(&ws, a, a)).await,
        Err(TaskError::SelfDependency)
    );
    assert_eq!(graph.list_dependencies(None).await.expect("all").len(), 2);
}

#[tokio::test(flavor = "multi_thread")]
async fn deleting_a_task_removes_its_edges_and_position() {
    let db: common::TestDb = common::test_db().await;
    let ws: String = workspace(&db, "demo").await;
    let (a, b): (TaskId, TaskId) = (
        backlog_task(&db, &ws, "a").await,
        backlog_task(&db, &ws, "b").await,
    );
    let graph: TaskGraphRepositoryAdapter = TaskGraphRepositoryAdapter::new(db.pool.clone());
    graph
        .insert_dependency(&edge(&ws, a, b))
        .await
        .expect("edge");
    graph
        .set_position(b, NodePosition::new(1.0, 2.0).expect("pos"))
        .await
        .expect("position");

    assert!(
        TaskRepositoryAdapter::new(db.pool.clone())
            .delete_if_backlog(b)
            .await
            .expect("delete")
    );

    assert!(
        graph
            .list_dependencies(None)
            .await
            .expect("edges")
            .is_empty()
    );
    assert!(
        graph
            .list_positions(ws)
            .await
            .expect("positions")
            .is_empty()
    );
}

#[tokio::test(flavor = "multi_thread")]
async fn positions_are_upserted_per_task_and_scoped_to_the_workspace() {
    let db: common::TestDb = common::test_db().await;
    let ws: String = workspace(&db, "demo").await;
    let other_ws: String = workspace(&db, "other").await;
    let a: TaskId = backlog_task(&db, &ws, "a").await;
    let foreign: TaskId = backlog_task(&db, &other_ws, "x").await;
    let graph: TaskGraphRepositoryAdapter = TaskGraphRepositoryAdapter::new(db.pool.clone());

    graph
        .set_position(a, NodePosition::new(10.0, 20.0).expect("pos"))
        .await
        .expect("first");
    graph
        .set_position(a, NodePosition::new(-5.5, 7.25).expect("pos"))
        .await
        .expect("second");
    graph
        .set_position(foreign, NodePosition::new(1.0, 1.0).expect("pos"))
        .await
        .expect("foreign");
    // Una tarea inexistente se ignora sin error.
    graph
        .set_position(TaskId::new(), NodePosition::new(0.0, 0.0).expect("pos"))
        .await
        .expect("missing task");

    let positions: Vec<(TaskId, NodePosition)> = graph.list_positions(ws).await.expect("positions");
    assert_eq!(positions, vec![(a, NodePosition { x: -5.5, y: 7.25 })]);
}
