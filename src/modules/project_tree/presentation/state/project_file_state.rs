use leptos::prelude::*;

use crate::core::url_path::encode_path;
use crate::modules::project_tree::infrastructure::adapters::inbound::dto::project_tree_dto::ProjectFileDto;
use crate::modules::project_tree::infrastructure::adapters::inbound::server_fns::{
    read_project_file, write_project_file,
};

/// Tab route that opens `path` (relative to the project root) of a workspace (pure).
pub fn file_href(workspace_id: &str, path: &str) -> String {
    format!("/workspaces/{workspace_id}/files/{}", encode_path(path))
}

/// Whether the editor holds changes that are not on disk yet (pure).
pub fn is_dirty(draft: &str, loaded: &str) -> bool {
    draft != loaded
}

/// Screen state of one project file: read-only view, edit mode with a draft, and saving.
#[derive(Clone, Copy)]
pub struct ProjectFileState {
    pub file: Resource<Result<ProjectFileDto, ServerFnError>>,
    pub editing: RwSignal<bool>,
    pub draft: RwSignal<String>,
    reload: RwSignal<u32>,
    save: Action<String, Result<(), ServerFnError>>,
}

impl ProjectFileState {
    pub fn new(workspace_id: String, path: String) -> Self {
        let reload: RwSignal<u32> = RwSignal::new(0);
        let editing: RwSignal<bool> = RwSignal::new(false);
        let draft: RwSignal<String> = RwSignal::new(String::new());
        let file: Resource<Result<ProjectFileDto, ServerFnError>> = {
            let (workspace_id, path): (String, String) = (workspace_id.clone(), path.clone());
            Resource::new(
                move || reload.get(),
                move |_| read_project_file(workspace_id.clone(), path.clone()),
            )
        };
        let save: Action<String, Result<(), ServerFnError>> =
            Action::new(move |content: &String| {
                write_project_file(workspace_id.clone(), path.clone(), content.clone())
            });
        Effect::new(move |_| {
            if matches!(save.value().get(), Some(Ok(()))) {
                editing.set(false);
                reload.update(|n: &mut u32| *n += 1);
            }
        });
        Self {
            file,
            editing,
            draft,
            reload,
            save,
        }
    }

    pub fn start_edit(&self, loaded: &str) {
        self.draft.set(loaded.to_string());
        self.editing.set(true);
    }

    pub fn discard(&self) {
        self.editing.set(false);
    }

    pub fn submit(&self) {
        self.save.dispatch(self.draft.get_untracked());
    }

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
    fn dirty_only_when_text_differs() {
        assert!(!is_dirty("a", "a"));
        assert!(is_dirty("a ", "a"));
    }
}
