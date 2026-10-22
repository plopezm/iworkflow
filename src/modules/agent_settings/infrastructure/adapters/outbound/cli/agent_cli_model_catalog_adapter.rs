use std::collections::HashMap;
use std::ffi::OsString;
use std::process::Output;
use std::time::{Duration, Instant};

use tokio::process::Command;
use tokio::sync::Mutex;

use crate::core::env::{LoginShellInvocation, login_shell_invocation};
use crate::modules::agent_settings::domain::errors::agent_settings_error::AgentSettingsError;
use crate::modules::agent_settings::domain::models::agent_runtime::AgentRuntime;
use crate::modules::agent_settings::domain::models::model_ref::ModelRef;
use crate::modules::agent_settings::domain::ports::outbound::model_catalog_port::ModelCatalogPort;
use crate::modules::agent_settings::infrastructure::adapters::outbound::cli::cursor_acp_model_catalog::{
    CURSOR_CATALOG_TIMEOUT, list_cursor_models,
};

const LIST_MODELS_TIMEOUT: Duration = Duration::from_secs(10);
const LIST_MODELS_FLAG: &str = "--list-models";
/// Listar modelos lanza un proceso del agente (y, con él, sus servidores MCP): el resultado se
/// reutiliza este tiempo en lugar de lanzar uno por consulta.
const MODELS_CACHE_TTL: Duration = Duration::from_secs(300);

/// Claude Code no tiene `--list-models`: sus modelos se eligen por alias (`claude --model`).
const CLAUDE_PROVIDER: &str = "anthropic";
const CLAUDE_MODEL_ALIASES: [&str; 4] = ["fable", "opus", "sonnet", "haiku"];

/// Lista los modelos disponibles ejecutando `<bin> --list-models`, donde `<bin>` es el binario
/// registrado para el runtime elegido. Un runtime sin binario registrado no tiene modelos.
pub struct AgentCliModelCatalogAdapter {
    bins: HashMap<AgentRuntime, String>,
    timeout: Duration,
    cache: Mutex<HashMap<AgentRuntime, (Instant, Vec<ModelRef>)>>,
}

impl AgentCliModelCatalogAdapter {
    pub fn new(bins: HashMap<AgentRuntime, String>) -> Self {
        Self {
            bins,
            timeout: LIST_MODELS_TIMEOUT,
            cache: Mutex::new(HashMap::new()),
        }
    }

    fn bin_for(&self, runtime: AgentRuntime) -> Option<&str> {
        self.bins.get(&runtime).map(String::as_str)
    }

    /// Modelos de Cursor (ids de ACP), cacheados; un fallo no se cachea.
    async fn cursor_models(&self, bin: &str) -> Result<Vec<ModelRef>, AgentSettingsError> {
        self.cached(AgentRuntime::Cursor, async {
            list_cursor_models(bin, std::env::vars_os().collect(), CURSOR_CATALOG_TIMEOUT).await
        })
        .await
    }

    /// Modelos de `<bin> --list-models`, cacheados; un fallo no se cachea.
    async fn cli_models(
        &self,
        runtime: AgentRuntime,
        bin: &str,
    ) -> Result<Vec<ModelRef>, AgentSettingsError> {
        self.cached(runtime, self.run_list_models(bin)).await
    }

    /// Devuelve la lista cacheada de `runtime` si no ha caducado; si no, ejecuta `fetch` y la
    /// guarda. El bloqueo solo se mantiene para leer y para guardar: retenerlo mientras corre
    /// la CLI haría esperar a los demás todo el lanzamiento (hasta el plazo del catálogo).
    async fn cached(
        &self,
        runtime: AgentRuntime,
        fetch: impl std::future::Future<Output = Result<Vec<ModelRef>, AgentSettingsError>>,
    ) -> Result<Vec<ModelRef>, AgentSettingsError> {
        if let Some((fetched, models)) = self.cache.lock().await.get(&runtime)
            && fetched.elapsed() < MODELS_CACHE_TTL
        {
            return Ok(models.clone());
        }
        let models: Vec<ModelRef> = fetch.await?;
        self.cache
            .lock()
            .await
            .insert(runtime, (Instant::now(), models.clone()));
        Ok(models)
    }

    async fn run_list_models(&self, bin: &str) -> Result<Vec<ModelRef>, AgentSettingsError> {
        let output: Output = tokio::time::timeout(
            self.timeout,
            list_models_command(bin, std::env::vars_os()).output(),
        )
        .await
        .map_err(|_| {
            AgentSettingsError::Catalog("tiempo de espera agotado listando modelos".to_string())
        })?
        .map_err(|e| AgentSettingsError::Catalog(format!("no se pudo ejecutar {}: {e}", bin)))?;
        if !output.status.success() {
            return Err(AgentSettingsError::Catalog(format!(
                "{} --list-models falló: {}",
                bin,
                String::from_utf8_lossy(&output.stderr)
            )));
        }
        Ok(parse_model_table(&String::from_utf8_lossy(&output.stdout)))
    }
}

#[async_trait::async_trait]
impl ModelCatalogPort for AgentCliModelCatalogAdapter {
    async fn list(&self, runtime: AgentRuntime) -> Result<Vec<ModelRef>, AgentSettingsError> {
        if runtime == AgentRuntime::Claude {
            return Ok(claude_models());
        }
        let Some(bin) = self.bin_for(runtime) else {
            return Ok(Vec::new());
        };
        if runtime == AgentRuntime::Cursor {
            return self.cursor_models(bin).await;
        }
        self.cli_models(runtime, bin).await
    }
}

fn claude_models() -> Vec<ModelRef> {
    CLAUDE_MODEL_ALIASES
        .iter()
        .map(|alias: &&str| ModelRef {
            provider: CLAUDE_PROVIDER.to_string(),
            model: alias.to_string(),
        })
        .collect()
}

/// Comando `<bin> --list-models` lanzado por la shell de login con el entorno vacío salvo las
/// variables de sesión de `vars`.
fn list_models_command(bin: &str, vars: impl Iterator<Item = (OsString, OsString)>) -> Command {
    let invocation: LoginShellInvocation =
        login_shell_invocation(bin, &[LIST_MODELS_FLAG.to_string()], vars);
    let mut command: Command = Command::new(invocation.shell);
    command
        .env_clear()
        .envs(invocation.env)
        .args(invocation.args)
        // Si vence el plazo se descarta el future; sin esto el proceso seguiría vivo.
        .kill_on_drop(true);
    command
}

/// Parsea la tabla de `--list-models`: cabecera
/// `provider model context max-out thinking images` seguida de una fila por modelo.
fn parse_model_table(text: &str) -> Vec<ModelRef> {
    text.lines()
        .skip(1)
        .filter_map(|line: &str| {
            let mut columns = line.split_whitespace();
            let provider: &str = columns.next()?;
            let model: &str = columns.next()?;
            Some(ModelRef {
                provider: provider.to_string(),
                model: model.to_string(),
            })
        })
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn picks_the_binary_of_the_selected_runtime() {
        let adapter: AgentCliModelCatalogAdapter = both_runtimes("gentle-shell-bin", "pi-bin");

        assert_eq!(
            adapter.bin_for(AgentRuntime::GentleShell),
            Some("gentle-shell-bin")
        );
        assert_eq!(adapter.bin_for(AgentRuntime::Pi), Some("pi-bin"));
    }

    fn both_runtimes(gentle_shell: &str, pi: &str) -> AgentCliModelCatalogAdapter {
        AgentCliModelCatalogAdapter::new(HashMap::from([
            (AgentRuntime::GentleShell, gentle_shell.to_string()),
            (AgentRuntime::Pi, pi.to_string()),
        ]))
    }

    #[tokio::test]
    async fn claude_lists_its_static_aliases_without_running_a_binary() {
        let adapter: AgentCliModelCatalogAdapter = AgentCliModelCatalogAdapter::new(HashMap::new());

        let models: Vec<String> = adapter
            .list(AgentRuntime::Claude)
            .await
            .expect("catálogo de claude")
            .into_iter()
            .map(|m: ModelRef| format!("{}/{}", m.provider, m.model))
            .collect();

        assert_eq!(
            models,
            vec![
                "anthropic/fable",
                "anthropic/opus",
                "anthropic/sonnet",
                "anthropic/haiku"
            ]
        );
    }

    #[tokio::test]
    async fn runtime_without_a_registered_binary_lists_no_models() {
        let adapter: AgentCliModelCatalogAdapter = AgentCliModelCatalogAdapter::new(HashMap::from(
            [(AgentRuntime::Pi, "/no/existe".to_string())],
        ));

        let models: Result<Vec<ModelRef>, AgentSettingsError> =
            adapter.list(AgentRuntime::GentleShell).await;

        assert_eq!(models, Ok(Vec::new()));
    }

    #[cfg(unix)]
    #[test]
    fn list_models_command_runs_the_binary_in_a_login_shell_with_a_clean_env() {
        let vars: Vec<(OsString, OsString)> = vec![
            (OsString::from("SHELL"), OsString::from("/bin/zsh")),
            (OsString::from("HOME"), OsString::from("/home/u")),
            (OsString::from("LEPTOS_ENV"), OsString::from("dev")),
        ];

        let command: Command = list_models_command("my pi", vars.into_iter());

        let std_command: &std::process::Command = command.as_std();
        assert_eq!(std_command.get_program(), "/bin/zsh");
        let args: Vec<String> = std_command
            .get_args()
            .map(|arg| arg.to_string_lossy().into_owned())
            .collect();
        assert_eq!(args, vec!["-l", "-c", "exec 'my pi' '--list-models'"]);
        let keys: Vec<String> = std_command
            .get_envs()
            .map(|(key, _)| key.to_string_lossy().into_owned())
            .collect();
        assert_eq!(keys, vec!["HOME", "PATH", "SHELL"]);
    }

    #[cfg(unix)]
    /// Escribe un agente falso ejecutable con el cuerpo de shell dado.
    fn fake_bin(dir: &tempfile::TempDir, body: &str) -> String {
        use std::os::unix::fs::PermissionsExt;
        let path: std::path::PathBuf = dir.path().join("fake-agent");
        std::fs::write(&path, format!("#!/bin/sh\n{body}\n")).expect("write");
        std::fs::set_permissions(&path, std::fs::Permissions::from_mode(0o755)).expect("chmod");
        path.to_string_lossy().into_owned()
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn lists_models_through_the_login_shell_without_parent_variables() {
        let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
        // Si el agente viera CARGO_MANIFEST_DIR (lo exporta cargo test) saldría un modelo extra.
        let bin: String = fake_bin(
            &dir,
            "echo 'provider model context max-out thinking images'\n\
             echo 'anthropic claude-5 200000 8192 yes yes'\n\
             if [ -n \"$CARGO_MANIFEST_DIR\" ]; then echo 'leak leaked 1 1 no no'; fi",
        );
        let adapter: AgentCliModelCatalogAdapter = both_runtimes(&bin, &bin);

        let models: Vec<ModelRef> = adapter.list(AgentRuntime::Pi).await.expect("list");

        assert_eq!(
            models,
            vec![ModelRef {
                provider: "anthropic".to_string(),
                model: "claude-5".to_string(),
            }]
        );
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn failing_binary_is_a_catalog_error() {
        let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
        let bin: String = fake_bin(&dir, "echo boom >&2; exit 2");
        let adapter: AgentCliModelCatalogAdapter = both_runtimes(&bin, &bin);

        let result: Result<Vec<ModelRef>, AgentSettingsError> =
            adapter.list(AgentRuntime::GentleShell).await;

        assert!(
            matches!(result, Err(AgentSettingsError::Catalog(message)) if message.contains("boom"))
        );
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn repeated_listing_reuses_the_cached_models_instead_of_spawning_again() {
        let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
        let counter: std::path::PathBuf = dir.path().join("runs");
        let bin: String = fake_bin(
            &dir,
            &format!(
                "echo run >> '{}'\necho 'provider model context max-out thinking images'\n\
                 echo 'anthropic claude-5 200000 8192 yes yes'",
                counter.display()
            ),
        );
        let adapter: AgentCliModelCatalogAdapter = both_runtimes(&bin, &bin);

        let first: Vec<ModelRef> = adapter.list(AgentRuntime::Pi).await.expect("first");
        let second: Vec<ModelRef> = adapter.list(AgentRuntime::Pi).await.expect("second");

        assert_eq!(first, second);
        let runs: String = std::fs::read_to_string(&counter).expect("counter");
        assert_eq!(runs.lines().count(), 1);
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn failures_are_not_cached() {
        let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
        let counter: std::path::PathBuf = dir.path().join("runs");
        let bin: String = fake_bin(
            &dir,
            &format!("echo run >> '{}'; echo boom >&2; exit 2", counter.display()),
        );
        let adapter: AgentCliModelCatalogAdapter = both_runtimes(&bin, &bin);

        assert!(adapter.list(AgentRuntime::Pi).await.is_err());
        assert!(adapter.list(AgentRuntime::Pi).await.is_err());

        let runs: String = std::fs::read_to_string(&counter).expect("counter");
        assert_eq!(runs.lines().count(), 2);
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn each_runtime_has_its_own_cache_entry() {
        let dir: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
        let bin: String = fake_bin(
            &dir,
            "echo 'provider model context max-out thinking images'\n\
             echo 'anthropic claude-5 200000 8192 yes yes'",
        );
        let other: tempfile::TempDir = tempfile::tempdir().expect("tempdir");
        let other_bin: String = fake_bin(
            &other,
            "echo 'provider model context max-out thinking images'\n\
             echo 'openai gpt-5 128000 4096 no yes'",
        );
        let adapter: AgentCliModelCatalogAdapter = both_runtimes(&bin, &other_bin);

        let shell: Vec<ModelRef> = adapter
            .list(AgentRuntime::GentleShell)
            .await
            .expect("shell");
        let pi: Vec<ModelRef> = adapter.list(AgentRuntime::Pi).await.expect("pi");

        assert_eq!(shell[0].provider, "anthropic");
        assert_eq!(pi[0].provider, "openai");
    }

    #[test]
    fn parses_table_skipping_header() {
        let text: &str = "provider model context max-out thinking images\n\
             anthropic claude-5 200000 8192 yes yes\n\
             openai gpt-5 128000 4096 no yes\n";

        let models: Vec<ModelRef> = parse_model_table(text);

        assert_eq!(
            models,
            vec![
                ModelRef {
                    provider: "anthropic".to_string(),
                    model: "claude-5".to_string(),
                },
                ModelRef {
                    provider: "openai".to_string(),
                    model: "gpt-5".to_string(),
                },
            ]
        );
    }

    #[test]
    fn header_only_yields_no_models() {
        assert!(parse_model_table("provider model context max-out thinking images\n").is_empty());
    }

    #[test]
    fn empty_input_yields_no_models() {
        assert!(parse_model_table("").is_empty());
    }

    #[test]
    fn ignores_blank_lines() {
        let text: &str = "provider model context max-out thinking images\n\n\
             anthropic claude-5 200000 8192 yes yes\n";

        assert_eq!(
            parse_model_table(text),
            vec![ModelRef {
                provider: "anthropic".to_string(),
                model: "claude-5".to_string(),
            }]
        );
    }
}
