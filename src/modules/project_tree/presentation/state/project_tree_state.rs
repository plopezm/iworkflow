use std::collections::{HashMap, HashSet};

use leptos::prelude::*;

use crate::modules::project_tree::infrastructure::adapters::inbound::dto::project_tree_dto::{
    ProjectDirectoryDto, TreeEntryDto,
};
use crate::modules::project_tree::infrastructure::adapters::inbound::server_fns::list_project_directory;

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
        Self {
            workspace_id: id,
            root: Resource::new(
                || (),
                move |_| list_project_directory(id.get_value(), ROOT_PATH.to_string()),
            ),
            expanded: RwSignal::new(HashSet::new()),
            children,
            load_children,
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
