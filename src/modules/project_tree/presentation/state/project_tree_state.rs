use std::collections::{HashMap, HashSet};

use leptos::prelude::*;

use crate::modules::project_tree::infrastructure::adapters::inbound::dto::project_tree_dto::{
    GitChangeDto, GitChangeKindDto, ProjectDirectoryDto, TreeEntryDto,
};
use crate::modules::project_tree::infrastructure::adapters::inbound::server_fns::{
    list_project_directory, list_project_git_changes,
};
use crate::modules::project_tree::presentation::state::git_status::{status_class, status_map};

#[cfg(feature = "hydrate")]
/// How often the git status is refetched while the tree is open, in seconds.
const GIT_POLL_SECONDS: u64 = 5;

/// Path of the project root in the server function contract.
const ROOT_PATH: &str = "";

/// Lazy-loading status of the children of one directory.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ChildrenState {
    Loading,
    Loaded(Vec<TreeEntryDto>),
    Failed(String),
}

/// Screen state of the project tree: root listing, expanded folders and lazily loaded children.
#[derive(Clone, Copy)]
pub struct ProjectTreeState {
    pub workspace_id: StoredValue<String>,
    pub root: Resource<Result<ProjectDirectoryDto, ServerFnError>>,
    pub expanded: RwSignal<HashSet<String>>,
    pub children: RwSignal<HashMap<String, ChildrenState>>,
    /// Git status per path (folders included); empty while loading or when it cannot be read.
    pub git: Memo<HashMap<String, GitChangeKindDto>>,
    reload: RwSignal<u32>,
    git_reload: RwSignal<u32>,
    load_children: Action<String, ()>,
}

/// Flips `path` in the expanded set and returns whether it is expanded afterwards (pure).
pub fn toggle_path(expanded: &mut HashSet<String>, path: &str) -> bool {
    if expanded.remove(path) {
        false
    } else {
        expanded.insert(path.to_string());
        true
    }
}

/// Expanded directories in a stable order, to refetch them after a refresh (pure).
pub fn paths_to_reload(expanded: &HashSet<String>) -> Vec<String> {
    let mut paths: Vec<String> = expanded.iter().cloned().collect();
    paths.sort();
    paths
}

/// A directory needs fetching unless it is already loading or loaded (pure).
pub fn needs_load(children: &HashMap<String, ChildrenState>, path: &str) -> bool {
    matches!(children.get(path), None | Some(ChildrenState::Failed(_)))
}

/// Maps a listing result into a children state (pure).
pub fn children_from_result(result: Result<ProjectDirectoryDto, ServerFnError>) -> ChildrenState {
    match result {
        Ok(directory) => ChildrenState::Loaded(directory.entries),
        Err(e) => ChildrenState::Failed(e.to_string()),
    }
}

impl ProjectTreeState {
    pub fn new(workspace_id: String) -> Self {
        let id: StoredValue<String> = StoredValue::new(workspace_id);
        let children: RwSignal<HashMap<String, ChildrenState>> = RwSignal::new(HashMap::new());
        let load_children: Action<String, ()> = Action::new(move |path: &String| {
            let path: String = path.clone();
            async move {
                children.update(|map: &mut HashMap<String, ChildrenState>| {
                    map.insert(path.clone(), ChildrenState::Loading);
                });
                let result: Result<ProjectDirectoryDto, ServerFnError> =
                    list_project_directory(id.get_value(), path.clone()).await;
                children.update(|map: &mut HashMap<String, ChildrenState>| {
                    map.insert(path, children_from_result(result));
                });
            }
        });
        let reload: RwSignal<u32> = RwSignal::new(0);
        let git_reload: RwSignal<u32> = RwSignal::new(0);
        let git_changes: LocalResource<Result<Vec<GitChangeDto>, ServerFnError>> =
            LocalResource::new(move || {
                git_reload.track();
                list_project_git_changes(id.get_value())
            });
        let git: Memo<HashMap<String, GitChangeKindDto>> = Memo::new(move |_| {
            git_changes
                .get()
                .and_then(|result: Result<Vec<GitChangeDto>, ServerFnError>| result.ok())
                .map(|changes: Vec<GitChangeDto>| status_map(&changes))
                .unwrap_or_default()
        });
        #[cfg(feature = "hydrate")]
        Effect::new(move |_| {
            let period: std::time::Duration = std::time::Duration::from_secs(GIT_POLL_SECONDS);
            let handle = leptos::leptos_dom::helpers::set_interval_with_handle(
                move || git_reload.update(|n: &mut u32| *n = n.wrapping_add(1)),
                period,
            );
            if let Ok(handle) = handle {
                on_cleanup(move || handle.clear());
            }
        });
        Self {
            workspace_id: id,
            root: Resource::new(
                move || reload.get(),
                move |_| list_project_directory(id.get_value(), ROOT_PATH.to_string()),
            ),
            expanded: RwSignal::new(HashSet::new()),
            children,
            git,
            reload,
            git_reload,
            load_children,
        }
    }

    /// Refetches the root and every expanded directory, keeping the expansion state.
    pub fn refresh(&self) {
        let expanded: Vec<String> = self
            .expanded
            .with_untracked(|set: &HashSet<String>| paths_to_reload(set));
        self.children
            .update(|map: &mut HashMap<String, ChildrenState>| map.clear());
        self.reload.update(|n: &mut u32| *n = n.wrapping_add(1));
        self.git_reload.update(|n: &mut u32| *n = n.wrapping_add(1));
        for path in expanded {
            self.load_children.dispatch(path);
        }
    }

    /// Expands or collapses a directory, fetching its children the first time.
    pub fn toggle(&self, path: &str) {
        let now_expanded: bool = self
            .expanded
            .try_update(|set: &mut HashSet<String>| toggle_path(set, path))
            .unwrap_or(false);
        let missing: bool = self
            .children
            .with_untracked(|map: &HashMap<String, ChildrenState>| needs_load(map, path));
        if now_expanded && missing {
            self.load_children.dispatch(path.to_string());
        }
    }

    /// CSS modifier class that colors `path` by its git status.
    pub fn git_class(&self, path: &str) -> &'static str {
        status_class(
            self.git
                .with(|map: &HashMap<String, GitChangeKindDto>| map.get(path).copied()),
        )
    }

    pub fn is_expanded(&self, path: &str) -> bool {
        self.expanded
            .with(|set: &HashSet<String>| set.contains(path))
    }

    pub fn children(&self, path: &str) -> Option<ChildrenState> {
        self.children
            .with(|map: &HashMap<String, ChildrenState>| map.get(path).cloned())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn directory(names: &[&str]) -> ProjectDirectoryDto {
        ProjectDirectoryDto {
            workspace_name: "demo".to_string(),
            path: "src".to_string(),
            entries: names
                .iter()
                .map(|n: &&str| TreeEntryDto {
                    name: (*n).to_string(),
                    path: format!("src/{n}"),
                    is_dir: false,
                })
                .collect(),
        }
    }

    #[test]
    fn paths_to_reload_is_sorted() {
        let set: HashSet<String> = ["src/b", "src", "src/a"]
            .iter()
            .map(|s| s.to_string())
            .collect();
        assert_eq!(paths_to_reload(&set), vec!["src", "src/a", "src/b"]);
    }

    #[test]
    fn toggle_path_flips_membership() {
        let mut set: HashSet<String> = HashSet::new();
        assert!(toggle_path(&mut set, "src"));
        assert!(set.contains("src"));
        assert!(!toggle_path(&mut set, "src"));
        assert!(set.is_empty());
    }

    #[test]
    fn needs_load_only_when_absent_or_failed() {
        let mut map: HashMap<String, ChildrenState> = HashMap::new();
        assert!(needs_load(&map, "src"));
        map.insert("src".to_string(), ChildrenState::Loading);
        assert!(!needs_load(&map, "src"));
        map.insert("src".to_string(), ChildrenState::Loaded(Vec::new()));
        assert!(!needs_load(&map, "src"));
        map.insert("src".to_string(), ChildrenState::Failed("x".to_string()));
        assert!(needs_load(&map, "src"));
    }

    #[test]
    fn children_from_result_maps_both_outcomes() {
        match children_from_result(Ok(directory(&["a.rs"]))) {
            ChildrenState::Loaded(entries) => assert_eq!(entries.len(), 1),
            other => panic!("unexpected {other:?}"),
        }
        match children_from_result(Err(ServerFnError::new("boom"))) {
            ChildrenState::Failed(message) => assert!(message.contains("boom"), "{message}"),
            other => panic!("unexpected {other:?}"),
        }
    }
}
