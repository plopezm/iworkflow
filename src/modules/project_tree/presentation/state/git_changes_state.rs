use leptos::prelude::*;

use crate::modules::project_tree::infrastructure::adapters::inbound::dto::project_tree_dto::{
    GitChangeDto, GitChangeKindDto,
};
use crate::modules::project_tree::infrastructure::adapters::inbound::server_fns::{
    list_project_git_changes, stage_project_paths, unstage_project_paths,
};

#[cfg(feature = "hydrate")]
/// How often the changes are refetched while the panel is open, in seconds.
const GIT_POLL_SECONDS: u64 = 5;

/// Changes grouped by what a commit would do with them.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct ChangeGroups {
    /// Part of the next commit.
    pub staged: Vec<GitChangeDto>,
    /// Tracked files modified in the working tree and not (fully) staged.
    pub unstaged: Vec<GitChangeDto>,
    /// New files git does not know yet.
    pub untracked: Vec<GitChangeDto>,
}

impl ChangeGroups {
    pub fn is_empty(&self) -> bool {
        self.staged.is_empty() && self.unstaged.is_empty() && self.untracked.is_empty()
    }
}

/// Splits changes into the three groups, each sorted by path; a partially staged file is in
/// both `staged` and `unstaged` (pure).
pub fn group_changes(changes: &[GitChangeDto]) -> ChangeGroups {
    let mut groups: ChangeGroups = ChangeGroups::default();
    for change in changes {
        if change.staged {
            groups.staged.push(change.clone());
        }
        if change.unstaged {
            if change.kind == GitChangeKindDto::Untracked {
                groups.untracked.push(change.clone());
            } else {
                groups.unstaged.push(change.clone());
            }
        }
    }
    for list in [
        &mut groups.staged,
        &mut groups.unstaged,
        &mut groups.untracked,
    ] {
        list.sort_by(|a: &GitChangeDto, b: &GitChangeDto| a.path.cmp(&b.path));
    }
    groups
}

/// File name of a `/`-separated path (pure).
pub fn file_name(path: &str) -> &str {
    path.rsplit('/').next().unwrap_or(path)
}

/// Directory of a `/`-separated path, empty for a file at the root (pure).
pub fn parent_dir(path: &str) -> &str {
    path.rsplit_once('/').map_or("", |(dir, _)| dir)
}

/// One-letter status shown at the end of a row (pure).
pub fn kind_letter(kind: GitChangeKindDto) -> &'static str {
    match kind {
        GitChangeKindDto::Modified => "M",
        GitChangeKindDto::Added => "A",
        GitChangeKindDto::Untracked => "U",
        GitChangeKindDto::Deleted => "D",
        GitChangeKindDto::Renamed => "R",
        GitChangeKindDto::Conflicted => "!",
    }
}

/// Paths of a group, for bulk actions (pure).
pub fn paths_of(changes: &[GitChangeDto]) -> Vec<String> {
    changes
        .iter()
        .map(|c: &GitChangeDto| c.path.clone())
        .collect()
}

/// Screen state of the changes panel: the grouped changes and the stage/unstage actions.
#[derive(Clone, Copy)]
pub struct GitChangesState {
    pub changes: LocalResource<Result<Vec<GitChangeDto>, ServerFnError>>,
    pub stage: Action<Vec<String>, Result<(), ServerFnError>>,
    pub unstage: Action<Vec<String>, Result<(), ServerFnError>>,
    /// Attach to the panel root: the poll only runs while it is displayed.
    pub root: NodeRef<leptos::html::Div>,
    /// Outcome of the latest stage/unstage request; `None` once one succeeds.
    error: RwSignal<Option<String>>,
    reload: RwSignal<u32>,
}

impl GitChangesState {
    pub fn new(workspace_id: String) -> Self {
        let id: StoredValue<String> = StoredValue::new(workspace_id);
        let reload: RwSignal<u32> = RwSignal::new(0);
        let error: RwSignal<Option<String>> = RwSignal::new(None);
        let root: NodeRef<leptos::html::Div> = NodeRef::new();
        let changes: LocalResource<Result<Vec<GitChangeDto>, ServerFnError>> =
            LocalResource::new(move || {
                reload.track();
                list_project_git_changes(id.get_value())
            });
        // Refetch right after the index changed, even when git rejected the request.
        let stage: Action<Vec<String>, Result<(), ServerFnError>> =
            Action::new(move |paths: &Vec<String>| {
                let paths: Vec<String> = paths.clone();
                async move {
                    let result: Result<(), ServerFnError> =
                        stage_project_paths(id.get_value(), paths).await;
                    error.set(error_of(&result));
                    reload.update(|n: &mut u32| *n = n.wrapping_add(1));
                    result
                }
            });
        let unstage: Action<Vec<String>, Result<(), ServerFnError>> =
            Action::new(move |paths: &Vec<String>| {
                let paths: Vec<String> = paths.clone();
                async move {
                    let result: Result<(), ServerFnError> =
                        unstage_project_paths(id.get_value(), paths).await;
                    error.set(error_of(&result));
                    reload.update(|n: &mut u32| *n = n.wrapping_add(1));
                    result
                }
            });
        #[cfg(feature = "hydrate")]
        Effect::new(move |_| {
            let period: std::time::Duration = std::time::Duration::from_secs(GIT_POLL_SECONDS);
            let handle = leptos::leptos_dom::helpers::set_interval_with_handle(
                move || {
                    if is_displayed(root) {
                        reload.update(|n: &mut u32| *n = n.wrapping_add(1));
                    }
                },
                period,
            );
            if let Ok(handle) = handle {
                on_cleanup(move || handle.clear());
            }
        });
        Self {
            changes,
            stage,
            unstage,
            root,
            error,
            reload,
        }
    }

    /// Refetches the changes now.
    pub fn refresh(&self) {
        self.reload.update(|n: &mut u32| *n = n.wrapping_add(1));
    }

    /// The message of the latest stage/unstage request when it failed.
    pub fn error(&self) -> Option<String> {
        self.error.get()
    }
}

/// Message to show for a stage/unstage outcome: a success clears a previous failure (pure).
pub fn error_of(result: &Result<(), ServerFnError>) -> Option<String> {
    result.as_ref().err().map(|e: &ServerFnError| e.to_string())
}

/// Whether the panel is on screen: a tab hidden by `KeepAlive` has no layout parent.
#[cfg(feature = "hydrate")]
fn is_displayed(root: NodeRef<leptos::html::Div>) -> bool {
    root.get_untracked()
        .is_some_and(|el: leptos::web_sys::HtmlDivElement| el.offset_parent().is_some())
}

#[cfg(test)]
mod tests {
    use super::*;

    fn change(path: &str, kind: GitChangeKindDto, staged: bool, unstaged: bool) -> GitChangeDto {
        GitChangeDto {
            path: path.to_string(),
            kind,
            staged,
            unstaged,
        }
    }

    fn paths(list: &[GitChangeDto]) -> Vec<&str> {
        list.iter()
            .map(|c: &GitChangeDto| c.path.as_str())
            .collect()
    }

    #[test]
    fn groups_changes_by_what_a_commit_would_do() {
        let groups: ChangeGroups = group_changes(&[
            change("z.rs", GitChangeKindDto::Modified, false, true),
            change("a.rs", GitChangeKindDto::Added, true, false),
            change("n.txt", GitChangeKindDto::Untracked, false, true),
            change("both.rs", GitChangeKindDto::Modified, true, true),
        ]);
        assert_eq!(paths(&groups.staged), vec!["a.rs", "both.rs"]);
        assert_eq!(paths(&groups.unstaged), vec!["both.rs", "z.rs"]);
        assert_eq!(paths(&groups.untracked), vec!["n.txt"]);
    }

    #[test]
    fn no_changes_is_an_empty_group_set() {
        assert!(group_changes(&[]).is_empty());
        assert!(!group_changes(&[change("a", GitChangeKindDto::Modified, true, false)]).is_empty());
    }

    #[test]
    fn splits_name_and_directory() {
        assert_eq!(file_name("src/a/b.rs"), "b.rs");
        assert_eq!(parent_dir("src/a/b.rs"), "src/a");
        assert_eq!(file_name("b.rs"), "b.rs");
        assert_eq!(parent_dir("b.rs"), "");
    }

    #[test]
    fn a_success_clears_the_previous_failure() {
        let failed: Result<(), ServerFnError> = Err(ServerFnError::new("boom"));
        assert_eq!(
            error_of(&failed),
            Some(ServerFnError::new("boom").to_string())
        );
        assert_eq!(error_of(&Ok(())), None);
    }

    #[test]
    fn letters_follow_the_status() {
        assert_eq!(kind_letter(GitChangeKindDto::Modified), "M");
        assert_eq!(kind_letter(GitChangeKindDto::Untracked), "U");
        assert_eq!(kind_letter(GitChangeKindDto::Conflicted), "!");
    }

    #[test]
    fn bulk_actions_take_every_path_of_the_group() {
        let list: Vec<GitChangeDto> = vec![
            change("a", GitChangeKindDto::Modified, false, true),
            change("b", GitChangeKindDto::Modified, false, true),
        ];
        assert_eq!(paths_of(&list), vec!["a".to_string(), "b".to_string()]);
    }
}
