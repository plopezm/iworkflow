use std::path::{Path, PathBuf};

const DEFAULT_GENTLE_SHELL_BIN: &str = "gentle-shell";
const DEFAULT_PI_BIN: &str = "pi";
const DEFAULT_CLAUDE_BIN: &str = "claude";
const DEFAULT_DATA_DIR: &str = ".iworkflow";
const DEFAULT_DATABASE_FILE: &str = "workspaces.db";
const DEFAULT_WORKTREES_DIR: &str = "worktrees";
const DEFAULT_AGENTS_FILES_DIR: &str = "agents-files";
const DEFAULT_ATTACHMENTS_DIR: &str = "attachments";
const DEFAULT_GENTLE_PI_CONFIG_DIR: &str = ".pi/gentle-ai";
const DEFAULT_PI_AGENTS_DIR: &str = ".pi/agent/agents";

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
    /// Ejecutable de pi (sin gentle-ai) que se lanza en modo RPC cuando se elige ese runtime.
    pub pi_bin: String,
    /// Ejecutable de Claude Code que se lanza en modo stream-json cuando se elige ese runtime.
    pub claude_bin: String,
    /// Carpeta raíz donde se crean los worktrees de las tareas.
    pub worktrees_dir: PathBuf,
    /// Carpeta home del usuario que ejecuta el proceso (punto de partida del explorador de directorios).
    pub home_dir: PathBuf,
    /// Carpeta de configuración de gentle-pi (contiene `profiles.json`).
    pub gentle_pi_config_home: PathBuf,
    /// Carpeta donde se guardan los ficheros AGENTS.md subidos.
    pub agents_files_dir: PathBuf,
    /// Carpeta donde se guardan las imágenes adjuntas a las tareas.
    pub attachments_dir: PathBuf,
    /// Carpeta de pi donde viven los ficheros `<rol>.md` de los agentes.
    pub pi_agents_dir: PathBuf,
}

impl AppConfig {
    pub fn from_env() -> Result<Self, ConfigError> {
        let home: PathBuf = std::env::var("HOME")
            .map(PathBuf::from)
            .map_err(|_| ConfigError::Missing("HOME"))?;
        let data_dir: PathBuf = home.join(DEFAULT_DATA_DIR);
        let database_url: String = std::env::var("IWF_DATABASE_URL").unwrap_or_else(|_| {
            data_dir
                .join(DEFAULT_DATABASE_FILE)
                .to_string_lossy()
                .into_owned()
        });
        let gentle_shell_bin: String = std::env::var("IWF_GENTLE_SHELL_BIN")
            .unwrap_or_else(|_| DEFAULT_GENTLE_SHELL_BIN.to_string());
        let pi_bin: String =
            std::env::var("IWF_PI_BIN").unwrap_or_else(|_| DEFAULT_PI_BIN.to_string());
        let claude_bin: String =
            std::env::var("IWF_CLAUDE_BIN").unwrap_or_else(|_| DEFAULT_CLAUDE_BIN.to_string());
        let worktrees_dir: PathBuf = std::env::var("IWF_WORKTREES_DIR")
            .map(PathBuf::from)
            .unwrap_or_else(|_| data_dir.join(DEFAULT_WORKTREES_DIR));
        let gentle_pi_config_home: PathBuf = std::env::var("IWF_GENTLE_PI_CONFIG_HOME")
            .map(PathBuf::from)
            .unwrap_or_else(|_| home.join(DEFAULT_GENTLE_PI_CONFIG_DIR));
        let agents_files_dir: PathBuf = resolve_agents_files_dir(
            std::env::var("IWF_AGENTS_FILES_DIR").ok(),
            &database_url,
            &data_dir,
        );
        let attachments_dir: PathBuf = resolve_attachments_dir(
            std::env::var("IWF_ATTACHMENTS_DIR").ok(),
            &database_url,
            &data_dir,
        );
        let pi_agents_dir: PathBuf =
            resolve_pi_agents_dir(std::env::var("IWF_PI_AGENTS_DIR").ok(), &home);
        Ok(Self {
            database_url,
            gentle_shell_bin,
            pi_bin,
            claude_bin,
            worktrees_dir,
            home_dir: home,
            gentle_pi_config_home,
            agents_files_dir,
            attachments_dir,
            pi_agents_dir,
        })
    }
}

/// `IWF_PI_AGENTS_DIR`, o `~/.pi/agent/agents`.
fn resolve_pi_agents_dir(env_value: Option<String>, home: &Path) -> PathBuf {
    match env_value.filter(|v: &String| !v.trim().is_empty()) {
        Some(dir) => PathBuf::from(dir),
        None => home.join(DEFAULT_PI_AGENTS_DIR),
    }
}

/// `IWF_AGENTS_FILES_DIR`, o `agents-files` junto a la base de datos, o bajo la carpeta de datos.
fn resolve_agents_files_dir(
    env_value: Option<String>,
    database_url: &str,
    data_dir: &Path,
) -> PathBuf {
    resolve_data_subdir(env_value, database_url, data_dir, DEFAULT_AGENTS_FILES_DIR)
}

/// `IWF_ATTACHMENTS_DIR`, o `attachments` junto a la base de datos, o bajo la carpeta de datos.
fn resolve_attachments_dir(
    env_value: Option<String>,
    database_url: &str,
    data_dir: &Path,
) -> PathBuf {
    resolve_data_subdir(env_value, database_url, data_dir, DEFAULT_ATTACHMENTS_DIR)
}

fn resolve_data_subdir(
    env_value: Option<String>,
    database_url: &str,
    data_dir: &Path,
    default_name: &str,
) -> PathBuf {
    if let Some(dir) = env_value.filter(|v: &String| !v.trim().is_empty()) {
        return PathBuf::from(dir);
    }
    match Path::new(database_url).parent() {
        Some(parent) if !parent.as_os_str().is_empty() => parent.join(default_name),
        _ => data_dir.join(default_name),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn pi_agents_dir_env_value_wins() {
        let dir: PathBuf =
            resolve_pi_agents_dir(Some("/custom/agents".to_string()), Path::new("/home/u"));
        assert_eq!(dir, PathBuf::from("/custom/agents"));
    }

    #[test]
    fn pi_agents_dir_defaults_under_home() {
        let dir: PathBuf = resolve_pi_agents_dir(None, Path::new("/home/u"));
        assert_eq!(dir, PathBuf::from("/home/u/.pi/agent/agents"));
    }

    #[test]
    fn pi_agents_dir_blank_env_falls_back_to_default() {
        let dir: PathBuf = resolve_pi_agents_dir(Some(" ".to_string()), Path::new("/home/u"));
        assert_eq!(dir, PathBuf::from("/home/u/.pi/agent/agents"));
    }

    #[test]
    fn env_value_wins() {
        let dir: PathBuf = resolve_agents_files_dir(
            Some("/custom/agents".to_string()),
            "/data/workspaces.db",
            Path::new("/home/u/.iworkflow"),
        );
        assert_eq!(dir, PathBuf::from("/custom/agents"));
    }

    #[test]
    fn defaults_next_to_the_database() {
        let dir: PathBuf =
            resolve_agents_files_dir(None, "/data/workspaces.db", Path::new("/home/u/.iworkflow"));
        assert_eq!(dir, PathBuf::from("/data/agents-files"));
    }

    #[test]
    fn blank_env_falls_back_to_database_parent() {
        let dir: PathBuf = resolve_agents_files_dir(
            Some("  ".to_string()),
            "/data/workspaces.db",
            Path::new("/home/u/.iworkflow"),
        );
        assert_eq!(dir, PathBuf::from("/data/agents-files"));
    }

    #[test]
    fn falls_back_to_data_dir_without_database_parent() {
        let dir: PathBuf =
            resolve_agents_files_dir(None, "workspaces.db", Path::new("/home/u/.iworkflow"));
        assert_eq!(dir, PathBuf::from("/home/u/.iworkflow/agents-files"));
    }

    #[test]
    fn attachments_env_value_wins() {
        let dir: PathBuf = resolve_attachments_dir(
            Some("/custom/attachments".to_string()),
            "/data/workspaces.db",
            Path::new("/home/u/.iworkflow"),
        );
        assert_eq!(dir, PathBuf::from("/custom/attachments"));
    }

    #[test]
    fn attachments_default_next_to_the_database() {
        let dir: PathBuf =
            resolve_attachments_dir(None, "/data/workspaces.db", Path::new("/home/u/.iworkflow"));
        assert_eq!(dir, PathBuf::from("/data/attachments"));
    }

    #[test]
    fn attachments_blank_env_falls_back_to_database_parent() {
        let dir: PathBuf = resolve_attachments_dir(
            Some("  ".to_string()),
            "/data/workspaces.db",
            Path::new("/home/u/.iworkflow"),
        );
        assert_eq!(dir, PathBuf::from("/data/attachments"));
    }

    #[test]
    fn attachments_fall_back_to_data_dir_without_database_parent() {
        let dir: PathBuf =
            resolve_attachments_dir(None, "workspaces.db", Path::new("/home/u/.iworkflow"));
        assert_eq!(dir, PathBuf::from("/home/u/.iworkflow/attachments"));
    }
}
