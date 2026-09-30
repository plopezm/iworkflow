use std::path::{Path, PathBuf};

const DEFAULT_GENTLE_SHELL_BIN: &str = "gentle-shell";
const DEFAULT_DATA_DIR: &str = ".pi-dashboard";
const DEFAULT_DATABASE_FILE: &str = "workspaces.db";
const DEFAULT_WORKTREES_DIR: &str = "worktrees";
const DEFAULT_AGENTS_FILES_DIR: &str = "agents-files";
const DEFAULT_GENTLE_PI_CONFIG_DIR: &str = ".pi/gentle-ai";

#[derive(Debug, thiserror::Error)]
pub enum ConfigError {
    #[error("falta la variable de entorno {0}")]
    Missing(&'static str),
}

/// Configuración del proceso leída de variables de entorno (`.env` soportado).
#[derive(Debug, Clone)]
pub struct AppConfig {
    /// Ruta al fichero SQLite.
    pub database_url: String,
    /// Ejecutable de gentle-shell que se lanza en modo RPC.
    pub gentle_shell_bin: String,
    /// Carpeta raíz donde se crean los worktrees de las tareas.
    pub worktrees_dir: PathBuf,
    /// Carpeta home del usuario que ejecuta el proceso (punto de partida del explorador de directorios).
    pub home_dir: PathBuf,
    /// Carpeta de configuración de gentle-pi (contiene `profiles.json`).
    pub gentle_pi_config_home: PathBuf,
    /// Carpeta donde se guardan los ficheros AGENTS.md subidos.
    pub agents_files_dir: PathBuf,
}

impl AppConfig {
    pub fn from_env() -> Result<Self, ConfigError> {
        let home: PathBuf = std::env::var("HOME")
            .map(PathBuf::from)
            .map_err(|_| ConfigError::Missing("HOME"))?;
        let data_dir: PathBuf = home.join(DEFAULT_DATA_DIR);
        let database_url: String = std::env::var("DATABASE_URL").unwrap_or_else(|_| {
            data_dir
                .join(DEFAULT_DATABASE_FILE)
                .to_string_lossy()
                .into_owned()
        });
        let gentle_shell_bin: String = std::env::var("GENTLE_SHELL_BIN")
            .unwrap_or_else(|_| DEFAULT_GENTLE_SHELL_BIN.to_string());
        let worktrees_dir: PathBuf = std::env::var("WORKTREES_DIR")
            .map(PathBuf::from)
            .unwrap_or_else(|_| data_dir.join(DEFAULT_WORKTREES_DIR));
        let gentle_pi_config_home: PathBuf = std::env::var("GENTLE_PI_CONFIG_HOME")
            .map(PathBuf::from)
            .unwrap_or_else(|_| home.join(DEFAULT_GENTLE_PI_CONFIG_DIR));
        let agents_files_dir: PathBuf = resolve_agents_files_dir(
            std::env::var("AGENTS_FILES_DIR").ok(),
            &database_url,
            &data_dir,
        );
        Ok(Self {
            database_url,
            gentle_shell_bin,
            worktrees_dir,
            home_dir: home,
            gentle_pi_config_home,
            agents_files_dir,
        })
    }
}

/// `AGENTS_FILES_DIR`, o `agents-files` junto a la base de datos, o bajo la carpeta de datos.
fn resolve_agents_files_dir(
    env_value: Option<String>,
    database_url: &str,
    data_dir: &Path,
) -> PathBuf {
    if let Some(dir) = env_value.filter(|v: &String| !v.trim().is_empty()) {
        return PathBuf::from(dir);
    }
    match Path::new(database_url).parent() {
        Some(parent) if !parent.as_os_str().is_empty() => parent.join(DEFAULT_AGENTS_FILES_DIR),
        _ => data_dir.join(DEFAULT_AGENTS_FILES_DIR),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn env_value_wins() {
        let dir: PathBuf = resolve_agents_files_dir(
            Some("/custom/agents".to_string()),
            "/data/workspaces.db",
            Path::new("/home/u/.pi-dashboard"),
        );
        assert_eq!(dir, PathBuf::from("/custom/agents"));
    }

    #[test]
    fn defaults_next_to_the_database() {
        let dir: PathBuf = resolve_agents_files_dir(
            None,
            "/data/workspaces.db",
            Path::new("/home/u/.pi-dashboard"),
        );
        assert_eq!(dir, PathBuf::from("/data/agents-files"));
    }

    #[test]
    fn blank_env_falls_back_to_database_parent() {
        let dir: PathBuf = resolve_agents_files_dir(
            Some("  ".to_string()),
            "/data/workspaces.db",
            Path::new("/home/u/.pi-dashboard"),
        );
        assert_eq!(dir, PathBuf::from("/data/agents-files"));
    }

    #[test]
    fn falls_back_to_data_dir_without_database_parent() {
        let dir: PathBuf =
            resolve_agents_files_dir(None, "workspaces.db", Path::new("/home/u/.pi-dashboard"));
        assert_eq!(dir, PathBuf::from("/home/u/.pi-dashboard/agents-files"));
    }
}
