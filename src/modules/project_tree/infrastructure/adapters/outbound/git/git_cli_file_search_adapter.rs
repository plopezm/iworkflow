use std::collections::HashMap;
use std::path::{Path, PathBuf};
use std::sync::Mutex;
use std::time::{Duration, Instant};

use crate::core::git::{GitCommandError, GitOutput, run_git_raw};
use crate::modules::project_tree::domain::errors::project_tree_error::ProjectTreeError;
use crate::modules::project_tree::domain::ports::outbound::file_search_port::FileSearchPort;

/// How long a listing is reused. Autocomplete asks once per keystroke; a few seconds absorb the
/// burst while new files still show up almost immediately.
const LISTING_TTL: Duration = Duration::from_secs(3);

/// Short-lived memory of the last listing per project root (pure: time is passed in).
struct ListingCache {
    ttl: Duration,
    entries: HashMap<PathBuf, (Instant, Vec<String>)>,
}

impl ListingCache {
    fn new(ttl: Duration) -> Self {
        Self {
            ttl,
            entries: HashMap::new(),
        }
    }

    /// The cached listing of `root` if it is younger than the TTL.
    fn get(&self, root: &Path, now: Instant) -> Option<Vec<String>> {
        let (stored_at, files) = self.entries.get(root)?;
        (now.saturating_duration_since(*stored_at) < self.ttl).then(|| files.clone())
    }

    /// Stores a listing and drops the expired ones so the cache cannot grow without bound.
    fn put(&mut self, root: &Path, files: Vec<String>, now: Instant) {
        let ttl: Duration = self.ttl;
        self.entries
            .retain(|_: &PathBuf, (stored_at, _): &mut (Instant, Vec<String>)| {
                now.saturating_duration_since(*stored_at) < ttl
            });
        self.entries.insert(root.to_path_buf(), (now, files));
    }
}

/// Lists project files with `git ls-files`, so ignored files and `.git` never show up.
pub struct GitCliFileSearchAdapter {
    cache: Mutex<ListingCache>,
}

impl Default for GitCliFileSearchAdapter {
    fn default() -> Self {
        Self::new()
    }
}

impl GitCliFileSearchAdapter {
    pub fn new() -> Self {
        Self {
            cache: Mutex::new(ListingCache::new(LISTING_TTL)),
        }
    }

    /// A poisoned lock only costs a cache miss.
    fn cached(&self, root: &Path) -> Option<Vec<String>> {
        self.cache.lock().ok()?.get(root, Instant::now())
    }

    fn remember(&self, root: &Path, files: &[String]) {
        if let Ok(mut cache) = self.cache.lock() {
            cache.put(root, files.to_vec(), Instant::now());
        }
    }
}

fn git_error(error: GitCommandError) -> ProjectTreeError {
    ProjectTreeError::Io(error.to_string())
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

/// Runs git to list the files of `root`; empty when it is not a git repository.
async fn list_files(root: &Path) -> Result<Vec<String>, ProjectTreeError> {
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
    .map_err(git_error)?;
    if !listed.success {
        return Ok(Vec::new());
    }
    // `--cached` also lists tracked files that were deleted from the worktree.
    let deleted: GitOutput = run_git_raw(root, &["ls-files", "-z", "--deleted"])
        .await
        .map_err(git_error)?;
    let gone: Vec<String> = if deleted.success {
        parse_files(&deleted.stdout)
    } else {
        Vec::new()
    };
    Ok(without_deleted(parse_files(&listed.stdout), &gone))
}

#[async_trait::async_trait]
impl FileSearchPort for GitCliFileSearchAdapter {
    async fn files(&self, root: &Path) -> Result<Vec<String>, ProjectTreeError> {
        if let Some(files) = self.cached(root) {
            return Ok(files);
        }
        let files: Vec<String> = list_files(root).await?;
        self.remember(root, &files);
        Ok(files)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn listing(items: &[&str]) -> Vec<String> {
        items.iter().map(|s: &&str| s.to_string()).collect()
    }

    #[test]
    fn a_fresh_listing_is_reused_and_an_old_one_is_not() {
        let root: &Path = Path::new("/work/a");
        let start: Instant = Instant::now();
        let mut cache: ListingCache = ListingCache::new(Duration::from_secs(3));
        assert_eq!(cache.get(root, start), None);

        cache.put(root, listing(&["a.rs"]), start);

        assert_eq!(
            cache.get(root, start + Duration::from_secs(2)),
            Some(listing(&["a.rs"]))
        );
        assert_eq!(cache.get(root, start + Duration::from_secs(3)), None);
    }

    #[test]
    fn roots_do_not_share_listings_and_expired_ones_are_dropped_on_put() {
        let start: Instant = Instant::now();
        let mut cache: ListingCache = ListingCache::new(Duration::from_secs(3));
        cache.put(Path::new("/work/a"), listing(&["a.rs"]), start);

        let later: Instant = start + Duration::from_secs(10);
        cache.put(Path::new("/work/b"), listing(&["b.rs"]), later);

        assert_eq!(cache.get(Path::new("/work/a"), later), None);
        assert_eq!(cache.entries.len(), 1);
        assert_eq!(
            cache.get(Path::new("/work/b"), later),
            Some(listing(&["b.rs"]))
        );
    }

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
