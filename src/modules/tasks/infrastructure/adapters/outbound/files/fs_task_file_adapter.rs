use std::path::{Component, Path, PathBuf};

use tokio::io::AsyncWriteExt as _;

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task_file_content::TaskFileContent;
use crate::modules::tasks::domain::ports::outbound::task_file_port::TaskFilePort;

/// Largest file (bytes) that can be read or saved from the diff view.
pub const MAX_FILE_BYTES: u64 = 1024 * 1024;

/// Reads and overwrites existing text files inside a worktree.
#[derive(Debug, Default)]
pub struct FsTaskFileAdapter;

impl FsTaskFileAdapter {
    pub fn new() -> Self {
        Self
    }
}

/// Validates a user-supplied relative path purely lexically.
pub fn validate_relative_path(rel: &str) -> Result<PathBuf, TaskError> {
    let invalid = |reason: &str| TaskError::InvalidFilePath(format!("{rel:?}: {reason}"));
    if rel.is_empty() {
        return Err(invalid("vacía"));
    }
    if rel.contains('\0') {
        return Err(invalid("contiene un byte nulo"));
    }
    let mut clean: PathBuf = PathBuf::new();
    for component in Path::new(rel).components() {
        match component {
            Component::Normal(part) if part == ".git" => return Err(invalid("apunta a .git")),
            Component::Normal(part) => clean.push(part),
            Component::CurDir => {}
            Component::ParentDir => return Err(invalid("contiene '..'")),
            Component::RootDir | Component::Prefix(_) => return Err(invalid("es absoluta")),
        }
    }
    if clean.as_os_str().is_empty() {
        return Err(invalid("vacía"));
    }
    Ok(clean)
}

fn not_editable(rel: &str, reason: &str) -> TaskError {
    TaskError::FileNotEditable(format!("{rel}: {reason}"))
}

/// Resolves `rel` to an existing regular file whose canonical path stays inside `root`.
async fn resolve_existing(root: &Path, rel: &str) -> Result<(PathBuf, u64), TaskError> {
    let relative: PathBuf = validate_relative_path(rel)?;
    let canonical_root: PathBuf = tokio::fs::canonicalize(root)
        .await
        .map_err(|e: std::io::Error| TaskError::Storage(format!("raíz del worktree: {e}")))?;
    let target: PathBuf = tokio::fs::canonicalize(canonical_root.join(&relative))
        .await
        .map_err(|e: std::io::Error| match e.kind() {
            std::io::ErrorKind::NotFound => not_editable(rel, "el fichero no existe"),
            _ => TaskError::Storage(format!("{rel}: {e}")),
        })?;
    if !target.starts_with(&canonical_root) {
        return Err(TaskError::InvalidFilePath(format!(
            "{rel:?}: sale del worktree"
        )));
    }
    if target
        .strip_prefix(&canonical_root)
        .is_ok_and(|inner: &Path| {
            inner
                .components()
                .any(|c: Component| c.as_os_str() == ".git")
        })
    {
        return Err(TaskError::InvalidFilePath(format!(
            "{rel:?}: apunta a .git"
        )));
    }
    let metadata: std::fs::Metadata = tokio::fs::metadata(&target)
        .await
        .map_err(|e: std::io::Error| TaskError::Storage(format!("{rel}: {e}")))?;
    if !metadata.is_file() {
        return Err(not_editable(rel, "no es un fichero regular"));
    }
    Ok((target, metadata.len()))
}

#[async_trait::async_trait]
impl TaskFilePort for FsTaskFileAdapter {
    async fn read(&self, root: &Path, rel: &str) -> Result<TaskFileContent, TaskError> {
        let (target, len): (PathBuf, u64) = resolve_existing(root, rel).await?;
        if len > MAX_FILE_BYTES {
            return Err(not_editable(rel, "supera el tamaño máximo de 1 MiB"));
        }
        let bytes: Vec<u8> = tokio::fs::read(&target)
            .await
            .map_err(|e: std::io::Error| TaskError::Storage(format!("{rel}: {e}")))?;
        let content: String =
            String::from_utf8(bytes).map_err(|_| not_editable(rel, "es binario o no es UTF-8"))?;
        Ok(TaskFileContent {
            path: rel.to_string(),
            content,
        })
    }

    async fn write(&self, root: &Path, rel: &str, content: &str) -> Result<(), TaskError> {
        if content.len() as u64 > MAX_FILE_BYTES {
            return Err(not_editable(rel, "supera el tamaño máximo de 1 MiB"));
        }
        let (target, _): (PathBuf, u64) = resolve_existing(root, rel).await?;
        // `create` stays off: only files that already exist can be overwritten.
        let mut file: tokio::fs::File = tokio::fs::OpenOptions::new()
            .write(true)
            .truncate(true)
            .open(&target)
            .await
            .map_err(|e: std::io::Error| TaskError::Storage(format!("{rel}: {e}")))?;
        file.write_all(content.as_bytes())
            .await
            .map_err(|e: std::io::Error| TaskError::Storage(format!("{rel}: {e}")))?;
        file.flush()
            .await
            .map_err(|e: std::io::Error| TaskError::Storage(format!("{rel}: {e}")))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn root() -> (tempfile::TempDir, PathBuf) {
        let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
        let root: PathBuf = dir.path().join("wt");
        std::fs::create_dir_all(root.join("src")).expect("dirs");
        std::fs::write(root.join("src/a.rs"), "fn a() {}\n").expect("file");
        (dir, root)
    }

    #[test]
    fn validation_rejects_unsafe_paths() {
        for bad in [
            "",
            "/etc/passwd",
            "../x",
            "a/../../x",
            "a/..",
            ".git/config",
            "src/.git/hooks",
            ".git",
            "a\0b",
        ] {
            assert!(
                matches!(
                    validate_relative_path(bad),
                    Err(TaskError::InvalidFilePath(_))
                ),
                "{bad:?}"
            );
        }
    }

    #[test]
    fn validation_accepts_nested_relative_paths() {
        assert_eq!(
            validate_relative_path("src/a.rs").expect("ok"),
            PathBuf::from("src/a.rs")
        );
        assert!(validate_relative_path(".github/ci.yml").is_ok());
    }

    #[tokio::test]
    async fn round_trip_overwrites_existing_file() {
        let (_dir, root) = root();
        let adapter: FsTaskFileAdapter = FsTaskFileAdapter::new();

        adapter
            .write(&root, "src/a.rs", "fn b() {}\n")
            .await
            .expect("write");
        let file: TaskFileContent = adapter.read(&root, "src/a.rs").await.expect("read");

        assert_eq!(file.path, "src/a.rs");
        assert_eq!(file.content, "fn b() {}\n");
    }

    #[tokio::test]
    async fn read_rejects_traversal_absolute_and_git() {
        let (_dir, root) = root();
        let adapter: FsTaskFileAdapter = FsTaskFileAdapter::new();
        for bad in ["../outside", "/etc/passwd", ".git/config"] {
            assert!(
                matches!(
                    adapter.read(&root, bad).await,
                    Err(TaskError::InvalidFilePath(_))
                ),
                "{bad}"
            );
        }
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn rejects_symlink_escape() {
        let (dir, root) = root();
        let secret: PathBuf = dir.path().join("secret.txt");
        std::fs::write(&secret, "secret").expect("secret");
        std::os::unix::fs::symlink(&secret, root.join("link.txt")).expect("symlink");
        std::os::unix::fs::symlink(dir.path(), root.join("linkdir")).expect("symlink dir");
        let adapter: FsTaskFileAdapter = FsTaskFileAdapter::new();

        assert!(matches!(
            adapter.read(&root, "link.txt").await,
            Err(TaskError::InvalidFilePath(_))
        ));
        assert!(matches!(
            adapter.write(&root, "linkdir/secret.txt", "x").await,
            Err(TaskError::InvalidFilePath(_))
        ));
        assert_eq!(std::fs::read_to_string(&secret).expect("read"), "secret");
    }

    #[tokio::test]
    async fn rejects_too_large_files() {
        let (_dir, root) = root();
        let big: String = "a".repeat(MAX_FILE_BYTES as usize + 1);
        std::fs::write(root.join("big.txt"), &big).expect("big");
        let adapter: FsTaskFileAdapter = FsTaskFileAdapter::new();

        assert!(matches!(
            adapter.read(&root, "big.txt").await,
            Err(TaskError::FileNotEditable(_))
        ));
        assert!(matches!(
            adapter.write(&root, "src/a.rs", &big).await,
            Err(TaskError::FileNotEditable(_))
        ));
    }

    #[tokio::test]
    async fn rejects_binary_files() {
        let (_dir, root) = root();
        std::fs::write(root.join("bin.dat"), [0xff_u8, 0xfe, 0x00, 0x01]).expect("bin");
        let adapter: FsTaskFileAdapter = FsTaskFileAdapter::new();

        assert!(matches!(
            adapter.read(&root, "bin.dat").await,
            Err(TaskError::FileNotEditable(_))
        ));
    }

    #[tokio::test]
    async fn never_creates_files_or_writes_directories() {
        let (_dir, root) = root();
        let adapter: FsTaskFileAdapter = FsTaskFileAdapter::new();

        assert!(matches!(
            adapter.write(&root, "src/new.rs", "x").await,
            Err(TaskError::FileNotEditable(_))
        ));
        assert!(!root.join("src/new.rs").exists());
        assert!(matches!(
            adapter.write(&root, "src", "x").await,
            Err(TaskError::FileNotEditable(_))
        ));
        assert!(matches!(
            adapter.read(&root, "src").await,
            Err(TaskError::FileNotEditable(_))
        ));
    }
}
