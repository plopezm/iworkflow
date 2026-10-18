//! Screen state of the task diff: parsed files, tree, view mode, collapse set and inline editor.

use std::collections::BTreeSet;

use leptos::prelude::*;

use crate::core::i18n::Msg;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_file_dto::{
    SaveTaskFileDto, TaskFileDto,
};
use crate::modules::tasks::infrastructure::adapters::inbound::server_fns::{
    get_task_file, save_task_file,
};
use crate::modules::tasks::presentation::state::diff_files::{
    FileDiff, FileStatus, TreeNode, build_tree, parse_patch,
};

const SPLIT: Msg = Msg {
    en: "Split",
    es: "Dividido",
    zh: "分栏",
    de: "Geteilt",
    fr: "Côte à côte",
};

const UNIFIED: Msg = Msg {
    en: "Unified",
    es: "Unificado",
    zh: "统一",
    de: "Vereint",
    fr: "Unifié",
};

/// Prefix of the DOM id of every file card (target of the tree anchors).
const ANCHOR_PREFIX: &str = "diff-file-";

/// How the rows of a file are laid out.
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq)]
pub enum ViewMode {
    #[default]
    Split,
    Unified,
}

impl ViewMode {
    pub fn toggled(self) -> Self {
        match self {
            ViewMode::Split => ViewMode::Unified,
            ViewMode::Unified => ViewMode::Split,
        }
    }

    pub fn label(self) -> &'static str {
        match self {
            ViewMode::Split => SPLIT.get(),
            ViewMode::Unified => UNIFIED.get(),
        }
    }
}

/// What to do once the server answered a save.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SaveOutcome {
    /// A commit was created: close the editor and reload the diff.
    Saved,
    /// The content was identical to the stored one: nothing to commit.
    Unchanged,
}

pub fn save_outcome(committed: bool) -> SaveOutcome {
    if committed {
        SaveOutcome::Saved
    } else {
        SaveOutcome::Unchanged
    }
}

/// DOM id of the card of `path`; every non alphanumeric byte is escaped so ids stay unique.
pub fn anchor_id(path: &str) -> String {
    let mut id: String = String::from(ANCHOR_PREFIX);
    for byte in path.bytes() {
        if byte.is_ascii_alphanumeric() {
            id.push(char::from(byte));
        } else {
            id.push_str(&format!("_{byte:02x}"));
        }
    }
    id
}

/// Adds `key` to `set`, or removes it when already present.
pub fn toggle_in(set: &mut BTreeSet<String>, key: &str) {
    if !set.remove(key) {
        set.insert(key.to_string());
    }
}

/// Deleted and binary files cannot be edited, and nothing can when the task has no worktree.
pub fn can_edit_file(file: &FileDiff, can_edit_files: bool) -> bool {
    can_edit_files && !file.binary && file.status != FileStatus::Deleted
}

/// Text the editor holds: the user's draft, or the loaded content while untouched.
pub fn effective_content(draft: Option<&str>, loaded: &str) -> String {
    draft.unwrap_or(loaded).to_string()
}

/// True when the draft differs from the loaded content.
pub fn is_dirty(draft: Option<&str>, loaded: &str) -> bool {
    draft.is_some_and(|d: &str| d != loaded)
}

pub fn file_paths(files: &[FileDiff]) -> BTreeSet<String> {
    files.iter().map(|f: &FileDiff| f.path.clone()).collect()
}

type SaveInput = (String, String);

/// Signals, resources and actions of one diff screen. Created once per task page so the
/// view mode, collapsed cards and selection survive diff reloads.
#[derive(Clone, Copy)]
pub struct DiffViewState {
    pub files: Memo<Vec<FileDiff>>,
    pub tree: Memo<Vec<TreeNode>>,
    pub view_mode: RwSignal<ViewMode>,
    pub selected_file: RwSignal<Option<String>>,
    pub collapsed_files: RwSignal<BTreeSet<String>>,
    pub collapsed_dirs: RwSignal<BTreeSet<String>>,
    pub tree_open: RwSignal<bool>,
    /// Fraction (0..1) of the layout taken by the tree sidebar; `None` keeps the default width.
    pub sidebar_ratio: RwSignal<Option<f64>>,
    pub summary_open: RwSignal<bool>,
    /// Path of the file being edited, if any.
    pub editing: RwSignal<Option<String>>,
    /// User's text while editing; `None` until the first keystroke.
    pub draft: RwSignal<Option<String>>,
    pub file: Resource<Option<Result<TaskFileDto, ServerFnError>>>,
    pub save: Action<SaveInput, Result<SaveTaskFileDto, ServerFnError>>,
}

impl DiffViewState {
    /// `patch` is the unified diff of the task; `refresh` is bumped after a successful save so
    /// the owner reloads the diff.
    pub fn new(
        task_id: StoredValue<String>,
        patch: Signal<String>,
        refresh: RwSignal<u32>,
    ) -> Self {
        let files: Memo<Vec<FileDiff>> =
            Memo::new(move |_| patch.with(|p: &String| parse_patch(p)));
        let tree: Memo<Vec<TreeNode>> =
            Memo::new(move |_| files.with(|f: &Vec<FileDiff>| build_tree(f)));
        let editing: RwSignal<Option<String>> = RwSignal::new(None);
        let draft: RwSignal<Option<String>> = RwSignal::new(None);
        let file: Resource<Option<Result<TaskFileDto, ServerFnError>>> = Resource::new(
            move || editing.get(),
            move |path: Option<String>| async move {
                match path {
                    Some(path) => Some(get_task_file(task_id.get_value(), path).await),
                    None => None,
                }
            },
        );
        let save: Action<SaveInput, Result<SaveTaskFileDto, ServerFnError>> =
            Action::new(move |input: &SaveInput| {
                let (path, content): SaveInput = input.clone();
                async move {
                    let result: Result<SaveTaskFileDto, ServerFnError> =
                        save_task_file(task_id.get_value(), path, content).await;
                    if let Ok(saved) = &result
                        && save_outcome(saved.committed) == SaveOutcome::Saved
                    {
                        editing.set(None);
                        draft.set(None);
                        refresh.update(|n: &mut u32| *n = n.wrapping_add(1));
                    }
                    result
                }
            });
        Self {
            files,
            tree,
            view_mode: RwSignal::new(ViewMode::default()),
            selected_file: RwSignal::new(None),
            collapsed_files: RwSignal::new(BTreeSet::new()),
            collapsed_dirs: RwSignal::new(BTreeSet::new()),
            tree_open: RwSignal::new(true),
            sidebar_ratio: RwSignal::new(None),
            summary_open: RwSignal::new(false),
            editing,
            draft,
            file,
            save,
        }
    }

    pub fn toggle_view_mode(&self) {
        self.view_mode.update(|m: &mut ViewMode| *m = m.toggled());
    }

    pub fn select(&self, path: &str) {
        self.selected_file.set(Some(path.to_string()));
        // A selected card must be visible to be scrolled to.
        self.collapsed_files.update(|set: &mut BTreeSet<String>| {
            set.remove(path);
        });
    }

    pub fn toggle_file(&self, path: &str) {
        self.collapsed_files
            .update(|set: &mut BTreeSet<String>| toggle_in(set, path));
    }

    pub fn toggle_dir(&self, path: &str) {
        self.collapsed_dirs
            .update(|set: &mut BTreeSet<String>| toggle_in(set, path));
    }

    pub fn is_file_collapsed(&self, path: &str) -> bool {
        self.collapsed_files
            .with(|set: &BTreeSet<String>| set.contains(path))
    }

    pub fn is_dir_collapsed(&self, path: &str) -> bool {
        self.collapsed_dirs
            .with(|set: &BTreeSet<String>| set.contains(path))
    }

    pub fn collapse_all(&self) {
        let paths: BTreeSet<String> = self.files.with(|f: &Vec<FileDiff>| file_paths(f));
        self.collapsed_files.set(paths);
    }

    pub fn expand_all(&self) {
        self.collapsed_files.set(BTreeSet::new());
    }

    pub fn start_edit(&self, path: &str) {
        self.draft.set(None);
        self.save.value().set(None);
        self.editing.set(Some(path.to_string()));
    }

    pub fn cancel_edit(&self) {
        self.editing.set(None);
        self.draft.set(None);
        self.save.value().set(None);
    }

    pub fn is_editing(&self, path: &str) -> bool {
        self.editing
            .with(|current: &Option<String>| current.as_deref() == Some(path))
    }

    pub fn submit(&self, path: &str, loaded: &str) {
        let content: String = self
            .draft
            .with(|d: &Option<String>| effective_content(d.as_deref(), loaded));
        self.save.dispatch((path.to_string(), content));
    }

    /// Error message of the last save, if it failed.
    pub fn save_error(&self) -> Option<String> {
        self.save.value().with(
            |v: &Option<Result<SaveTaskFileDto, ServerFnError>>| match v {
                Some(Err(e)) => Some(e.to_string()),
                _ => None,
            },
        )
    }

    /// True when the last save found nothing to commit.
    pub fn save_unchanged(&self) -> bool {
        self.save
            .value()
            .with(|v: &Option<Result<SaveTaskFileDto, ServerFnError>>| {
                matches!(v, Some(Ok(saved)) if save_outcome(saved.committed) == SaveOutcome::Unchanged)
            })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::presentation::state::diff_files::parse_patch;

    fn file(status_header: &str, path: &str, binary: bool) -> FileDiff {
        let body: String = if binary {
            "Binary files a/x and b/x differ\n".to_string()
        } else {
            format!("--- a/{path}\n+++ b/{path}\n@@ -1 +1 @@\n-a\n+b\n")
        };
        let patch: String = format!("diff --git a/{path} b/{path}\n{status_header}{body}");
        parse_patch(&patch).remove(0)
    }

    #[test]
    fn view_mode_toggles_and_labels() {
        assert_eq!(ViewMode::default(), ViewMode::Split);
        assert_eq!(ViewMode::Split.toggled(), ViewMode::Unified);
        assert_eq!(ViewMode::Unified.toggled(), ViewMode::Split);
        assert_eq!(ViewMode::Split.label(), "Split");
        assert_eq!(ViewMode::Unified.label(), "Unified");
    }

    #[test]
    fn save_outcome_closes_only_when_committed() {
        assert_eq!(save_outcome(true), SaveOutcome::Saved);
        assert_eq!(save_outcome(false), SaveOutcome::Unchanged);
    }

    #[test]
    fn anchor_id_is_dom_safe_and_injective() {
        let a: String = anchor_id("src/a-b.rs");
        let b: String = anchor_id("src/a/b.rs");
        assert_ne!(a, b);
        assert!(a.starts_with("diff-file-"));
        assert!(
            a.chars()
                .all(|c| c.is_ascii_alphanumeric() || c == '-' || c == '_')
        );
        assert_eq!(anchor_id("ñ.rs"), anchor_id("ñ.rs"));
    }

    #[test]
    fn toggle_in_adds_then_removes() {
        let mut set: BTreeSet<String> = BTreeSet::new();
        toggle_in(&mut set, "a");
        assert!(set.contains("a"));
        toggle_in(&mut set, "a");
        assert!(set.is_empty());
    }

    #[test]
    fn deleted_binary_and_readonly_files_are_not_editable() {
        let modified: FileDiff = file("", "a.rs", false);
        let added: FileDiff = file("new file mode 100644\n", "b.rs", false);
        let deleted: FileDiff = file("deleted file mode 100644\n", "c.rs", false);
        let binary: FileDiff = file("", "d.png", true);
        assert!(can_edit_file(&modified, true));
        assert!(can_edit_file(&added, true));
        assert!(!can_edit_file(&deleted, true));
        assert!(!can_edit_file(&binary, true));
        assert!(!can_edit_file(&modified, false));
    }

    #[test]
    fn draft_overrides_loaded_content() {
        assert_eq!(effective_content(None, "old"), "old");
        assert_eq!(effective_content(Some("new"), "old"), "new");
        assert_eq!(effective_content(Some(""), "old"), "");
    }

    #[test]
    fn dirty_only_when_draft_differs() {
        assert!(!is_dirty(None, "x"));
        assert!(!is_dirty(Some("x"), "x"));
        assert!(is_dirty(Some("y"), "x"));
    }

    #[test]
    fn file_paths_collects_every_file() {
        let files: Vec<FileDiff> = vec![file("", "a.rs", false), file("", "b/c.rs", false)];
        let paths: BTreeSet<String> = file_paths(&files);
        assert_eq!(paths.len(), 2);
        assert!(paths.contains("b/c.rs"));
    }
}
