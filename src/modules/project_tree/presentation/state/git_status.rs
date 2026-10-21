use std::collections::HashMap;

use crate::modules::project_tree::infrastructure::adapters::inbound::dto::project_tree_dto::{
    GitChangeDto, GitChangeKindDto,
};

/// Severity used to pick the status a folder shows when several descendants differ (pure).
fn rank(kind: GitChangeKindDto) -> u8 {
    match kind {
        GitChangeKindDto::Conflicted => 5,
        GitChangeKindDto::Deleted => 4,
        GitChangeKindDto::Modified => 3,
        GitChangeKindDto::Renamed => 2,
        GitChangeKindDto::Added | GitChangeKindDto::Untracked => 1,
    }
}

/// Status per path, folders included: a folder takes the most severe status below it (pure).
pub fn status_map(changes: &[GitChangeDto]) -> HashMap<String, GitChangeKindDto> {
    let mut map: HashMap<String, GitChangeKindDto> = HashMap::new();
    let mut put = |path: &str, kind: GitChangeKindDto| {
        let stronger: bool = map
            .get(path)
            .is_none_or(|current: &GitChangeKindDto| rank(kind) > rank(*current));
        if stronger {
            map.insert(path.to_string(), kind);
        }
    };
    for change in changes {
        put(&change.path, change.kind);
        for (index, _) in change.path.match_indices('/') {
            put(&change.path[..index], change.kind);
        }
    }
    map
}

/// CSS modifier class for a status, empty when the path is clean (pure).
pub fn status_class(kind: Option<GitChangeKindDto>) -> &'static str {
    match kind {
        None => "",
        Some(GitChangeKindDto::Modified | GitChangeKindDto::Renamed) => "git-modified",
        Some(GitChangeKindDto::Added | GitChangeKindDto::Untracked) => "git-added",
        Some(GitChangeKindDto::Deleted) => "git-deleted",
        Some(GitChangeKindDto::Conflicted) => "git-conflicted",
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn change(path: &str, kind: GitChangeKindDto) -> GitChangeDto {
        GitChangeDto {
            path: path.to_string(),
            kind,
            staged: false,
            unstaged: true,
        }
    }

    #[test]
    fn folders_inherit_the_most_severe_descendant() {
        let map: HashMap<String, GitChangeKindDto> = status_map(&[
            change("src/a/new.rs", GitChangeKindDto::Untracked),
            change("src/a/edit.rs", GitChangeKindDto::Modified),
            change("docs/x.md", GitChangeKindDto::Added),
        ]);
        assert_eq!(map.get("src"), Some(&GitChangeKindDto::Modified));
        assert_eq!(map.get("src/a"), Some(&GitChangeKindDto::Modified));
        assert_eq!(map.get("src/a/new.rs"), Some(&GitChangeKindDto::Untracked));
        assert_eq!(map.get("docs"), Some(&GitChangeKindDto::Added));
        assert_eq!(map.get("tests"), None);
    }

    #[test]
    fn conflicts_win_over_everything() {
        let map: HashMap<String, GitChangeKindDto> = status_map(&[
            change("a/m.rs", GitChangeKindDto::Modified),
            change("a/c.rs", GitChangeKindDto::Conflicted),
        ]);
        assert_eq!(map.get("a"), Some(&GitChangeKindDto::Conflicted));
    }

    #[test]
    fn classes_group_statuses_by_color() {
        assert_eq!(status_class(None), "");
        assert_eq!(status_class(Some(GitChangeKindDto::Untracked)), "git-added");
        assert_eq!(
            status_class(Some(GitChangeKindDto::Renamed)),
            "git-modified"
        );
        assert_eq!(status_class(Some(GitChangeKindDto::Deleted)), "git-deleted");
    }
}
