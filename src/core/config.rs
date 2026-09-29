use std::path::PathBuf;

const DEFAULT_GENTLE_SHELL_BIN: &str = "gentle-shell";
const DEFAULT_DATA_DIR: &str = ".pi-dashboard";
const DEFAULT_DATABASE_FILE: &str = "workspaces.db";
const DEFAULT_WORKTREES_DIR: &str = "worktrees";

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
        Ok(Self {
            database_url,
            gentle_shell_bin,
            worktrees_dir,
            home_dir: home,
        })
    }
}
