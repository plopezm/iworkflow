use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};

use crate::modules::agent_profiles::domain::errors::agent_profiles_error::AgentProfilesError;

static TMP_COUNTER: AtomicU64 = AtomicU64::new(0);

fn temp_sibling(target: &Path) -> PathBuf {
    let file_name: String = target
        .file_name()
        .map(|n: &std::ffi::OsStr| n.to_string_lossy().to_string())
        .unwrap_or_else(|| "file".to_string());
    let seq: u64 = TMP_COUNTER.fetch_add(1, Ordering::Relaxed);
    let tmp_name: String = format!(".{file_name}.{}.{seq}.tmp", std::process::id());
    target.with_file_name(tmp_name)
}

/// Escribe `content` en un fichero temporal hermano y lo renombra sobre `target`.
/// Crea el directorio padre si falta. No deja temporales si la escritura falla.
pub async fn atomic_write(target: &Path, content: &str) -> Result<(), AgentProfilesError> {
    if let Some(parent) = target.parent() {
        tokio::fs::create_dir_all(parent)
            .await
            .map_err(|e: std::io::Error| AgentProfilesError::Io(e.to_string()))?;
    }
    let tmp: PathBuf = temp_sibling(target);
    let written: Result<(), std::io::Error> = write_then_rename(&tmp, target, content).await;
    if let Err(e) = written {
        // Mejor esfuerzo: el error original es el relevante.
        let _ignored: Result<(), std::io::Error> = tokio::fs::remove_file(&tmp).await;
        return Err(AgentProfilesError::Io(e.to_string()));
    }
    Ok(())
}

async fn write_then_rename(tmp: &Path, target: &Path, content: &str) -> Result<(), std::io::Error> {
    tokio::fs::write(tmp, content).await?;
    tokio::fs::rename(tmp, target).await
}

/// Lee un fichero como texto; `None` si no existe.
pub async fn read_optional(path: &Path) -> Result<Option<String>, AgentProfilesError> {
    match tokio::fs::read_to_string(path).await {
        Ok(content) => Ok(Some(content)),
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => Ok(None),
        Err(e) => Err(AgentProfilesError::Io(e.to_string())),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn temp_sibling_is_hidden_unique_and_in_same_dir() {
        let target: PathBuf = PathBuf::from("/x/profiles.json");
        let a: PathBuf = temp_sibling(&target);
        let b: PathBuf = temp_sibling(&target);
        assert_ne!(a, b);
        assert_eq!(a.parent(), target.parent());
        assert!(a.to_string_lossy().ends_with(".tmp"));
    }
}
