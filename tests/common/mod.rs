#![allow(dead_code)]

use std::path::{Path, PathBuf};
use std::process::Command;

use pi_workflow::core::db::pool::{DbPool, build_pool, run_migrations};
use tempfile::TempDir;

/// Base de datos SQLite temporal con migraciones aplicadas. Mantener vivo el `TempDir`.
pub struct TestDb {
    pub dir: TempDir,
    pub pool: DbPool,
}

pub async fn test_db() -> TestDb {
    let dir: TempDir = tempfile::tempdir().expect("tempdir");
    let url: String = dir.path().join("test.db").to_string_lossy().into_owned();
    let pool: DbPool = build_pool(&url).expect("pool");
    run_migrations(&pool).await.expect("migraciones");
    TestDb { dir, pool }
}

pub fn git(dir: &Path, args: &[&str]) -> String {
    let output: std::process::Output = Command::new("git")
        .arg("-C")
        .arg(dir)
        .args(args)
        .output()
        .expect("git");
    assert!(
        output.status.success(),
        "git {args:?} falló: {}",
        String::from_utf8_lossy(&output.stderr)
    );
    String::from_utf8_lossy(&output.stdout).trim().to_string()
}

/// Repositorio git temporal con un commit inicial en la rama `main`.
pub fn git_repo() -> (TempDir, PathBuf) {
    let dir: TempDir = tempfile::tempdir().expect("tempdir");
    let root: PathBuf = dir.path().canonicalize().expect("canonical");
    git(&root, &["init", "-q", "-b", "main"]);
    git(&root, &["config", "user.email", "test@example.com"]);
    git(&root, &["config", "user.name", "Test"]);
    git(&root, &["config", "commit.gpgsign", "false"]);
    std::fs::write(root.join("README.md"), "hola\n").expect("write");
    git(&root, &["add", "."]);
    git(&root, &["commit", "-q", "-m", "init"]);
    (dir, root)
}
