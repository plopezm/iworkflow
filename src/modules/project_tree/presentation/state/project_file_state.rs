use leptos::prelude::*;

use crate::core::line_diff::{LineMark, line_marks};
use crate::core::url_path::encode_path;
use crate::modules::project_tree::infrastructure::adapters::inbound::dto::project_tree_dto::ProjectFileDto;
use crate::modules::project_tree::infrastructure::adapters::inbound::server_fns::{
    read_project_file, read_project_file_base, write_project_file,
};

/// Prefix of a task worktree scope id (`task:<uuid>`), as the server functions expect it.
const TASK_SCOPE_PREFIX: &str = "task:";

/// Task id when `scope` addresses a task worktree instead of a workspace (pure).
pub fn task_of_scope(scope: &str) -> Option<&str> {
    scope.strip_prefix(TASK_SCOPE_PREFIX)
}

/// Tab route that opens `path` (relative to the project root) of a workspace or task scope (pure).
pub fn file_href(scope: &str, path: &str) -> String {
    match task_of_scope(scope) {
        Some(task_id) => format!("/tasks/{task_id}/files/{}", encode_path(path)),
        None => format!("/workspaces/{scope}/files/{}", encode_path(path)),
    }
}

/// Tab route that opens a terminal in the root of the scope (pure).
pub fn terminal_href(scope: &str) -> String {
    match task_of_scope(scope) {
        Some(task_id) => format!("/tasks/{task_id}/terminal"),
        None => format!("/workspaces/{scope}/terminal"),
    }
}

/// Tab route that edits the workspace; tasks have none (pure).
pub fn edit_href(scope: &str) -> Option<String> {
    match task_of_scope(scope) {
        Some(_) => None,
        None => Some(format!("/workspaces/{scope}/edit")),
    }
}

/// Whether the editor holds changes that are not on disk yet (pure).
pub fn is_dirty(draft: &str, loaded: &str) -> bool {
    draft != loaded
}

/// Screen state of one project file: always editable, with a draft and saving.
#[derive(Clone, Copy)]
pub struct ProjectFileState {
    pub file: Resource<Result<ProjectFileDto, ServerFnError>>,
    pub draft: RwSignal<String>,
    /// Marcas de cambio del borrador frente a git HEAD; vacío mientras no hay base.
    pub marks: Memo<Vec<Option<LineMark>>>,
    reload: RwSignal<u32>,
    save: Action<String, Result<(), ServerFnError>>,
}

impl ProjectFileState {
    pub fn new(workspace_id: String, path: String) -> Self {
        let reload: RwSignal<u32> = RwSignal::new(0);
        let draft: RwSignal<String> = RwSignal::new(String::new());
        let file: Resource<Result<ProjectFileDto, ServerFnError>> = {
            let (workspace_id, path): (String, String) = (workspace_id.clone(), path.clone());
            Resource::new(
                move || reload.get(),
                move |_| read_project_file(workspace_id.clone(), path.clone()),
            )
        };
        let base: LocalResource<Option<String>> = {
            let (workspace_id, path): (String, String) = (workspace_id.clone(), path.clone());
            LocalResource::new(move || {
                reload.track();
                let (workspace_id, path): (String, String) = (workspace_id.clone(), path.clone());
                async move {
                    read_project_file_base(workspace_id, path)
                        .await
                        .ok()
                        .flatten()
                }
            })
        };
        let marks: Memo<Vec<Option<LineMark>>> = Memo::new(move |_| match base.get().flatten() {
            Some(base_text) => draft.with(|d: &String| line_marks(&base_text, d)),
            None => Vec::new(),
        });
        let save: Action<String, Result<(), ServerFnError>> =
            Action::new(move |content: &String| {
                write_project_file(workspace_id.clone(), path.clone(), content.clone())
            });
        Effect::new(move |_| {
            if matches!(save.value().get(), Some(Ok(()))) {
                reload.update(|n: &mut u32| *n += 1);
            }
        });
        Self {
            file,
            draft,
            marks,
            reload,
            save,
        }
    }

    pub fn submit(&self) {
        self.save.dispatch(self.draft.get_untracked());
    }

    /// Reloads the file from disk, discarding the draft.
    pub fn refresh(&self) {
        self.reload.update(|n: &mut u32| *n += 1);
    }

    pub fn saving(&self) -> Memo<bool> {
        self.save.pending()
    }

    pub fn save_error(&self) -> Option<String> {
        match self.save.value().get() {
            Some(Err(e)) => Some(e.to_string()),
            _ => None,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn href_encodes_each_segment() {
        assert_eq!(
            file_href("w1", "src/a b/c#.rs"),
            "/workspaces/w1/files/src/a%20b/c%23.rs"
        );
    }

    #[test]
    fn task_scope_routes_to_task_tabs() {
        assert_eq!(
            file_href("task:t1", "src/a b.rs"),
            "/tasks/t1/files/src/a%20b.rs"
        );
        assert_eq!(terminal_href("task:t1"), "/tasks/t1/terminal");
        assert_eq!(terminal_href("w1"), "/workspaces/w1/terminal");
        assert_eq!(edit_href("task:t1"), None);
        assert_eq!(edit_href("w1"), Some("/workspaces/w1/edit".to_string()));
    }

    #[test]
    fn dirty_only_when_text_differs() {
        assert!(!is_dirty("a", "a"));
        assert!(is_dirty("a ", "a"));
    }
}
