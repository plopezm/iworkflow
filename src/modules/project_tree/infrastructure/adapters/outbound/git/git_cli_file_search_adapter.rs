use std::path::Path;

use crate::core::git::{GitOutput, run_git_raw};
use crate::modules::project_tree::domain::errors::project_tree_error::ProjectTreeError;
use crate::modules::project_tree::domain::ports::outbound::file_search_port::FileSearchPort;

/// Lists project files with `git ls-files`, so ignored files and `.git` never show up.
#[derive(Default)]
pub struct GitCliFileSearchAdapter;

impl GitCliFileSearchAdapter {
    pub fn new() -> Self {
        Self
    }
}

/// Parses `git ls-files -z` output into paths (pure).
pub fn parse_files(output: &str) -> Vec<String> {
    output
        .split('\0')
        .filter(|path: &&str| !path.is_empty())
        .map(str::to_string)
        .collect()
}

/// Listed paths minus the ones deleted from the worktree (pure).
pub fn without_deleted(listed: Vec<String>, deleted: &[String]) -> Vec<String> {
    listed
        .into_iter()
        .filter(|path: &String| !deleted.contains(path))
        .collect()
}

#[async_trait::async_trait]
impl FileSearchPort for GitCliFileSearchAdapter {
    async fn files(&self, root: &Path) -> Result<Vec<String>, ProjectTreeError> {
        let listed: GitOutput = run_git_raw(
            root,
            &[
                "ls-files",
                "-z",
                "--cached",
                "--others",
                "--exclude-standard",
            ],
        )
        .await
        .map_err(|error: crate::core::git::GitCommandError| {
            ProjectTreeError::Io(error.to_string())
        })?;
        if !listed.success {
            return Ok(Vec::new());
        }
        // `--cached` also lists tracked files that were deleted from the worktree.
        let deleted: GitOutput = run_git_raw(root, &["ls-files", "-z", "--deleted"])
            .await
            .map_err(|error: crate::core::git::GitCommandError| {
                ProjectTreeError::Io(error.to_string())
            })?;
        let gone: Vec<String> = if deleted.success {
            parse_files(&deleted.stdout)
        } else {
            Vec::new()
        };
        Ok(without_deleted(parse_files(&listed.stdout), &gone))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn splits_nul_separated_paths() {
        assert_eq!(
            parse_files("src/a.rs\0README.md\0"),
            vec!["src/a.rs".to_string(), "README.md".to_string()]
        );
        assert!(parse_files("").is_empty());
    }

    #[test]
    fn deleted_paths_are_dropped_from_the_listing() {
        let listed: Vec<String> = vec!["a.rs".to_string(), "b.rs".to_string()];
        assert_eq!(
            without_deleted(listed, &["a.rs".to_string()]),
            vec!["b.rs".to_string()]
        );
    }
}
