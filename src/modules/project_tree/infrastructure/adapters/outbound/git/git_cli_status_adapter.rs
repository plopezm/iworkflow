use std::path::Path;

use crate::core::git::{GitOutput, run_git_raw};
use crate::modules::project_tree::domain::errors::project_tree_error::ProjectTreeError;
use crate::modules::project_tree::domain::models::git_change::{GitChange, GitChangeKind};
use crate::modules::project_tree::domain::ports::outbound::git_status_port::GitStatusPort;
use crate::modules::project_tree::infrastructure::adapters::outbound::fs::fs_directory_reader_adapter::validate_relative;

/// Reads git state with the `git` CLI.
#[derive(Default)]
pub struct GitCliStatusAdapter;

impl GitCliStatusAdapter {
    pub fn new() -> Self {
        Self
    }
}

fn git_error(error: crate::core::git::GitCommandError) -> ProjectTreeError {
    ProjectTreeError::Io(error.to_string())
}

/// Maps the two status columns of `git status --porcelain` to a change kind (pure).
fn kind_of(index: char, worktree: char) -> Option<GitChangeKind> {
    let pair: (char, char) = (index, worktree);
    match pair {
        ('?', '?') => Some(GitChangeKind::Untracked),
        ('!', '!') | (' ', ' ') => None,
        ('D', 'D') | ('A', 'A') | ('U', _) | (_, 'U') => Some(GitChangeKind::Conflicted),
        ('A', _) => Some(GitChangeKind::Added),
        ('R' | 'C', _) | (_, 'R' | 'C') => Some(GitChangeKind::Renamed),
        ('D', _) | (_, 'D') => Some(GitChangeKind::Deleted),
        _ => Some(GitChangeKind::Modified),
    }
}

/// Parses `git status --porcelain=v1 -z` output into changes under `prefix` (pure).
///
/// `prefix` is the path of the project root inside the repository (`""` or `"sub/dir/"`); paths
/// outside it are dropped and the prefix is stripped from the rest.
pub fn parse_status(output: &str, prefix: &str) -> Vec<GitChange> {
    let mut changes: Vec<GitChange> = Vec::new();
    let mut fields = output.split('\0').filter(|f: &&str| !f.is_empty());
    while let Some(field) = fields.next() {
        let mut chars = field.chars();
        let (Some(index), Some(worktree)) = (chars.next(), chars.next()) else {
            continue;
        };
        let path: &str = field.get(3..).unwrap_or_default();
        // A rename or copy carries its original path as an extra field.
        if matches!(index, 'R' | 'C') || matches!(worktree, 'R' | 'C') {
            fields.next();
        }
        let Some(kind) = kind_of(index, worktree) else {
            continue;
        };
        if let Some(relative) = path.strip_prefix(prefix) {
            changes.push(GitChange {
                path: relative.trim_end_matches('/').to_string(),
                kind,
                staged: !matches!(index, ' ' | '?' | '!'),
                unstaged: worktree != ' ',
            });
        }
    }
    changes
}

#[async_trait::async_trait]
impl GitStatusPort for GitCliStatusAdapter {
    async fn changes(&self, root: &Path) -> Result<Vec<GitChange>, ProjectTreeError> {
        let prefix: GitOutput = run_git_raw(root, &["rev-parse", "--show-prefix"])
            .await
            .map_err(git_error)?;
        if !prefix.success {
            return Ok(Vec::new());
        }
        let status: GitOutput = run_git_raw(
            root,
            &["status", "--porcelain=v1", "-z", "--untracked-files=all"],
        )
        .await
        .map_err(git_error)?;
        if !status.success {
            return Ok(Vec::new());
        }
        Ok(parse_status(&status.stdout, prefix.stdout.trim()))
    }

    async fn base_content(
        &self,
        root: &Path,
        relative: &str,
    ) -> Result<Option<String>, ProjectTreeError> {
        validate_relative(relative)?;
        let spec: String = format!("HEAD:./{relative}");
        let shown: GitOutput = run_git_raw(root, &["show", &spec])
            .await
            .map_err(git_error)?;
        Ok(shown.success.then_some(shown.stdout))
    }

    async fn stage(&self, root: &Path, paths: &[String]) -> Result<(), ProjectTreeError> {
        let args: Vec<&str> = pathspec_command(&["add", "-A"], paths)?;
        run_index_command(root, &args).await
    }

    async fn unstage(&self, root: &Path, paths: &[String]) -> Result<(), ProjectTreeError> {
        let args: Vec<&str> = pathspec_command(&["reset", "-q"], paths)?;
        run_index_command(root, &args).await
    }

    async fn commit(&self, root: &Path, message: &str) -> Result<(), ProjectTreeError> {
        let output: GitOutput = run_git_raw(root, &["commit", "-q", "-m", message])
            .await
            .map_err(git_error)?;
        if output.success {
            return Ok(());
        }
        // `git commit` reports "nothing to commit" on stdout, not stderr.
        let reason: String = if output.stderr.is_empty() {
            output.stdout.trim().to_string()
        } else {
            output.stderr
        };
        Err(ProjectTreeError::Io(reason))
    }
}

/// Builds `git --literal-pathspecs <command> -- <paths>`; no paths means the whole project (`.`).
///
/// Literal pathspecs stop names such as `:(exclude)x` or `*.rs` from being read as git magic.
fn pathspec_command<'a>(
    command: &[&'a str],
    paths: &'a [String],
) -> Result<Vec<&'a str>, ProjectTreeError> {
    let mut args: Vec<&str> = vec!["--literal-pathspecs"];
    args.extend_from_slice(command);
    args.push("--");
    if paths.is_empty() {
        args.push(".");
    }
    for path in paths {
        if path.is_empty() {
            return Err(ProjectTreeError::InvalidPath(path.clone()));
        }
        validate_relative(path)?;
        args.push(path);
    }
    Ok(args)
}

async fn run_index_command(root: &Path, args: &[&str]) -> Result<(), ProjectTreeError> {
    let output: GitOutput = run_git_raw(root, args).await.map_err(git_error)?;
    if output.success {
        Ok(())
    } else {
        Err(ProjectTreeError::Io(output.stderr))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn classifies_status_columns() {
        let out: String = [
            " M src/a.rs",
            "A  src/b.rs",
            "?? src/c.rs",
            " D src/d.rs",
            "UU src/e.rs",
            "AM src/f.rs",
        ]
        .join("\0");
        let kinds: Vec<(String, GitChangeKind)> = parse_status(&out, "")
            .into_iter()
            .map(|c: GitChange| (c.path, c.kind))
            .collect();
        assert_eq!(
            kinds,
            vec![
                ("src/a.rs".to_string(), GitChangeKind::Modified),
                ("src/b.rs".to_string(), GitChangeKind::Added),
                ("src/c.rs".to_string(), GitChangeKind::Untracked),
                ("src/d.rs".to_string(), GitChangeKind::Deleted),
                ("src/e.rs".to_string(), GitChangeKind::Conflicted),
                ("src/f.rs".to_string(), GitChangeKind::Added),
            ]
        );
    }

    #[test]
    fn index_and_worktree_columns_set_the_staged_flags() {
        let out: String = ["M  a", " M b", "MM c", "?? d", "A  e", " D f"].join("\0");
        let flags: Vec<(String, bool, bool)> = parse_status(&out, "")
            .into_iter()
            .map(|c: GitChange| (c.path, c.staged, c.unstaged))
            .collect();
        let expected: Vec<(&str, bool, bool)> = vec![
            ("a", true, false),
            ("b", false, true),
            ("c", true, true),
            ("d", false, true),
            ("e", true, false),
            ("f", false, true),
        ];
        let expected: Vec<(String, bool, bool)> = expected
            .into_iter()
            .map(|(p, s, u)| (p.to_string(), s, u))
            .collect();
        assert_eq!(flags, expected);
    }

    #[test]
    fn pathspec_command_defaults_to_the_whole_project() {
        let args: Vec<&str> = pathspec_command(&["add", "-A"], &[]).unwrap();
        assert_eq!(args, vec!["--literal-pathspecs", "add", "-A", "--", "."]);
    }

    #[test]
    fn pathspec_command_rejects_escaping_paths() {
        let paths: Vec<String> = vec!["../x".to_string()];
        assert!(pathspec_command(&["add", "-A"], &paths).is_err());
    }

    #[test]
    fn rename_consumes_the_original_path_field() {
        let out: String = ["R  new.rs", "old.rs", " M other.rs"].join("\0");
        let changes: Vec<GitChange> = parse_status(&out, "");
        assert_eq!(changes.len(), 2);
        assert_eq!(changes[0].kind, GitChangeKind::Renamed);
        assert_eq!(changes[0].path, "new.rs");
        assert_eq!(changes[1].path, "other.rs");
    }

    #[test]
    fn prefix_is_stripped_and_outside_paths_dropped() {
        let out: String = [" M app/src/a.rs", " M other/b.rs"].join("\0");
        let changes: Vec<GitChange> = parse_status(&out, "app/");
        assert_eq!(
            changes,
            vec![GitChange {
                path: "src/a.rs".to_string(),
                kind: GitChangeKind::Modified,
                staged: false,
                unstaged: true,
            }]
        );
    }

    #[test]
    fn ignored_and_empty_output_yield_nothing() {
        assert!(parse_status("", "").is_empty());
        assert!(parse_status("!! target/", "").is_empty());
    }
}
