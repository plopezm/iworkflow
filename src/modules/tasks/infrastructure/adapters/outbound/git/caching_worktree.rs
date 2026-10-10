use std::collections::VecDeque;
use std::path::{Path, PathBuf};
use std::sync::{Arc, Mutex, MutexGuard};

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::worktree::{DiffRange, MergeOutcome, TaskDiff};
use crate::modules::tasks::domain::ports::outbound::worktree_port::WorktreePort;

/// Número máximo de diffs conservados en memoria.
pub const DIFF_CACHE_CAPACITY: usize = 32;

/// Cache LRU pequeña con búsqueda lineal (la capacidad es mínima).
struct Lru {
    capacity: usize,
    /// Más reciente al final.
    entries: VecDeque<(String, TaskDiff)>,
}

impl Lru {
    fn get(&mut self, key: &str) -> Option<TaskDiff> {
        let index: usize = self.entries.iter().position(|(k, _)| k == key)?;
        let entry: (String, TaskDiff) = self.entries.remove(index)?;
        let value: TaskDiff = entry.1.clone();
        self.entries.push_back(entry);
        Some(value)
    }

    fn put(&mut self, key: String, value: TaskDiff) {
        self.entries.retain(|(k, _)| *k != key);
        if self.entries.len() >= self.capacity {
            self.entries.pop_front();
        }
        self.entries.push_back((key, value));
    }
}

/// Decorador de `WorktreePort` que evita recalcular diffs sin cambios.
///
/// - `diff_range`: el rango (base, head) es inmutable, la clave es el propio rango.
/// - `diff`: la clave incluye la huella `diff_fingerprint` (puntas de ambas ramas), calculada
///   en cada petición; si no cambia, el diff tampoco.
pub struct CachingWorktree {
    inner: Arc<dyn WorktreePort>,
    cache: Mutex<Lru>,
}

impl CachingWorktree {
    pub fn new(inner: Arc<dyn WorktreePort>) -> Self {
        Self::with_capacity(inner, DIFF_CACHE_CAPACITY)
    }

    pub fn with_capacity(inner: Arc<dyn WorktreePort>, capacity: usize) -> Self {
        Self {
            inner,
            cache: Mutex::new(Lru {
                capacity: capacity.max(1),
                entries: VecDeque::new(),
            }),
        }
    }

    fn lock(&self) -> MutexGuard<'_, Lru> {
        self.cache
            .lock()
            .unwrap_or_else(|poisoned| poisoned.into_inner())
    }

    fn lookup(&self, key: &str) -> Option<TaskDiff> {
        self.lock().get(key)
    }

    fn store(&self, key: String, diff: &TaskDiff) {
        self.lock().put(key, diff.clone());
    }
}

#[async_trait::async_trait]
impl WorktreePort for CachingWorktree {
    fn worktree_path(&self, workspace_name: &str, dir_name: &str) -> PathBuf {
        self.inner.worktree_path(workspace_name, dir_name)
    }

    async fn create(
        &self,
        repo: &Path,
        base_branch: &str,
        branch: &str,
        path: &Path,
    ) -> Result<(), TaskError> {
        self.inner.create(repo, base_branch, branch, path).await
    }

    async fn commit_all(&self, path: &Path, message: &str) -> Result<bool, TaskError> {
        self.inner.commit_all(path, message).await
    }

    async fn diff(
        &self,
        repo: &Path,
        base_branch: &str,
        branch: &str,
    ) -> Result<TaskDiff, TaskError> {
        let fingerprint: String = self
            .inner
            .diff_fingerprint(repo, base_branch, branch)
            .await?;
        let key: String = format!("diff|{}|{fingerprint}", repo.display());
        if let Some(hit) = self.lookup(&key) {
            return Ok(hit);
        }
        let diff: TaskDiff = self.inner.diff(repo, base_branch, branch).await?;
        self.store(key, &diff);
        Ok(diff)
    }

    async fn diff_fingerprint(
        &self,
        repo: &Path,
        base_branch: &str,
        branch: &str,
    ) -> Result<String, TaskError> {
        self.inner.diff_fingerprint(repo, base_branch, branch).await
    }

    async fn pin_range(
        &self,
        repo: &Path,
        base_branch: &str,
        branch: &str,
        pin_ref: &str,
    ) -> Result<DiffRange, TaskError> {
        self.inner
            .pin_range(repo, base_branch, branch, pin_ref)
            .await
    }

    async fn diff_range(&self, repo: &Path, range: &DiffRange) -> Result<TaskDiff, TaskError> {
        let key: String = format!(
            "range|{}|{}..{}",
            repo.display(),
            range.base_sha,
            range.head_sha
        );
        if let Some(hit) = self.lookup(&key) {
            return Ok(hit);
        }
        let diff: TaskDiff = self.inner.diff_range(repo, range).await?;
        self.store(key, &diff);
        Ok(diff)
    }

    async fn merge(
        &self,
        repo: &Path,
        base_branch: &str,
        branch: &str,
        message: &str,
    ) -> Result<MergeOutcome, TaskError> {
        self.inner.merge(repo, base_branch, branch, message).await
    }

    async fn exists(&self, path: &Path) -> bool {
        self.inner.exists(path).await
    }

    async fn remove(&self, repo: &Path, path: &Path, branch: &str) -> Result<(), TaskError> {
        self.inner.remove(repo, path, branch).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::mocks::MockWorktree;

    fn sample(patch: &str) -> TaskDiff {
        TaskDiff {
            stat: String::new(),
            patch: patch.to_string(),
            truncated: false,
            commits: Vec::new(),
        }
    }

    fn range(head: &str) -> DiffRange {
        DiffRange {
            base_sha: "base".to_string(),
            head_sha: head.to_string(),
        }
    }

    fn caching(inner: MockWorktree, capacity: usize) -> CachingWorktree {
        CachingWorktree::with_capacity(Arc::new(inner), capacity)
    }

    #[tokio::test]
    async fn open_diff_hits_when_fingerprint_is_unchanged() {
        let mut inner: MockWorktree = MockWorktree::new();
        inner
            .expect_diff_fingerprint()
            .times(2)
            .returning(|_, _, _| Ok("a:b".to_string()));
        inner
            .expect_diff()
            .times(1)
            .returning(|_, _, _| Ok(sample("one")));
        let cache: CachingWorktree = caching(inner, 4);

        let first: TaskDiff = cache
            .diff(Path::new("/r"), "main", "pi/x")
            .await
            .expect("1");
        let second: TaskDiff = cache
            .diff(Path::new("/r"), "main", "pi/x")
            .await
            .expect("2");

        assert_eq!(first, sample("one"));
        assert_eq!(second, first);
    }

    #[tokio::test]
    async fn open_diff_misses_when_fingerprint_changes() {
        let mut inner: MockWorktree = MockWorktree::new();
        let mut seq: mockall::Sequence = mockall::Sequence::new();
        inner
            .expect_diff_fingerprint()
            .times(1)
            .in_sequence(&mut seq)
            .returning(|_, _, _| Ok("a:b".to_string()));
        inner
            .expect_diff()
            .times(1)
            .in_sequence(&mut seq)
            .returning(|_, _, _| Ok(sample("old")));
        inner
            .expect_diff_fingerprint()
            .times(1)
            .in_sequence(&mut seq)
            .returning(|_, _, _| Ok("a:c".to_string()));
        inner
            .expect_diff()
            .times(1)
            .in_sequence(&mut seq)
            .returning(|_, _, _| Ok(sample("new")));
        let cache: CachingWorktree = caching(inner, 4);

        let old: TaskDiff = cache
            .diff(Path::new("/r"), "main", "pi/x")
            .await
            .expect("1");
        let new: TaskDiff = cache
            .diff(Path::new("/r"), "main", "pi/x")
            .await
            .expect("2");

        assert_eq!(old, sample("old"));
        assert_eq!(new, sample("new"));
    }

    #[tokio::test]
    async fn fingerprint_error_is_propagated_without_diffing() {
        let mut inner: MockWorktree = MockWorktree::new();
        inner
            .expect_diff_fingerprint()
            .returning(|_, _, _| Err(TaskError::Git("boom".to_string())));
        inner.expect_diff().never();
        let cache: CachingWorktree = caching(inner, 4);

        let result: Result<TaskDiff, TaskError> = cache.diff(Path::new("/r"), "main", "pi/x").await;

        assert!(matches!(result, Err(TaskError::Git(_))));
    }

    #[tokio::test]
    async fn range_diff_is_cached_by_range_without_fingerprint() {
        let mut inner: MockWorktree = MockWorktree::new();
        inner.expect_diff_fingerprint().never();
        inner
            .expect_diff_range()
            .times(1)
            .returning(|_, _| Ok(sample("merged")));
        let cache: CachingWorktree = caching(inner, 4);

        let first: TaskDiff = cache
            .diff_range(Path::new("/r"), &range("h1"))
            .await
            .expect("1");
        let second: TaskDiff = cache
            .diff_range(Path::new("/r"), &range("h1"))
            .await
            .expect("2");

        assert_eq!(first, second);
    }

    #[tokio::test]
    async fn different_ranges_do_not_collide() {
        let mut inner: MockWorktree = MockWorktree::new();
        inner
            .expect_diff_range()
            .times(2)
            .returning(|_, r: &DiffRange| Ok(sample(&r.head_sha)));
        let cache: CachingWorktree = caching(inner, 4);

        let a: TaskDiff = cache
            .diff_range(Path::new("/r"), &range("h1"))
            .await
            .expect("a");
        let b: TaskDiff = cache
            .diff_range(Path::new("/r"), &range("h2"))
            .await
            .expect("b");

        assert_eq!(a, sample("h1"));
        assert_eq!(b, sample("h2"));
    }

    #[tokio::test]
    async fn evicts_least_recently_used_entry() {
        let mut inner: MockWorktree = MockWorktree::new();
        // h1 se calcula 2 veces (expulsada), h2 y h3 una sola vez.
        inner
            .expect_diff_range()
            .times(4)
            .returning(|_, r: &DiffRange| Ok(sample(&r.head_sha)));
        let cache: CachingWorktree = caching(inner, 2);
        let repo: &Path = Path::new("/r");

        cache.diff_range(repo, &range("h1")).await.expect("h1");
        cache.diff_range(repo, &range("h2")).await.expect("h2");
        cache.diff_range(repo, &range("h3")).await.expect("h3"); // expulsa h1
        cache.diff_range(repo, &range("h2")).await.expect("h2 hit");
        cache.diff_range(repo, &range("h1")).await.expect("h1 miss");
    }

    #[tokio::test]
    async fn hit_refreshes_recency() {
        let mut inner: MockWorktree = MockWorktree::new();
        // h1, h2, h3 + h2 recalculada tras ser expulsada = 4; h1 sigue viva por el hit.
        inner
            .expect_diff_range()
            .times(4)
            .returning(|_, r: &DiffRange| Ok(sample(&r.head_sha)));
        let cache: CachingWorktree = caching(inner, 2);
        let repo: &Path = Path::new("/r");

        cache.diff_range(repo, &range("h1")).await.expect("h1");
        cache.diff_range(repo, &range("h2")).await.expect("h2");
        cache.diff_range(repo, &range("h1")).await.expect("h1 hit");
        cache.diff_range(repo, &range("h3")).await.expect("h3"); // expulsa h2
        cache.diff_range(repo, &range("h1")).await.expect("h1 hit");
        cache.diff_range(repo, &range("h2")).await.expect("h2 miss");
    }
}
