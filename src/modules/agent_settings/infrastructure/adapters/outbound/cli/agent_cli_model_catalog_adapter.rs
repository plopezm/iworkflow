use std::process::Output;
use std::time::Duration;

use tokio::process::Command;

use crate::modules::agent_settings::domain::errors::agent_settings_error::AgentSettingsError;
use crate::modules::agent_settings::domain::models::agent_runtime::AgentRuntime;
use crate::modules::agent_settings::domain::models::model_ref::ModelRef;
use crate::modules::agent_settings::domain::ports::outbound::model_catalog_port::ModelCatalogPort;

const LIST_MODELS_TIMEOUT: Duration = Duration::from_secs(10);

/// Lista los modelos disponibles ejecutando `<bin> --list-models`, donde `<bin>` es
/// gentle-shell o pi según el runtime elegido.
pub struct AgentCliModelCatalogAdapter {
    gentle_shell_bin: String,
    pi_bin: String,
    timeout: Duration,
}

impl AgentCliModelCatalogAdapter {
    pub fn new(gentle_shell_bin: impl Into<String>, pi_bin: impl Into<String>) -> Self {
        Self {
            gentle_shell_bin: gentle_shell_bin.into(),
            pi_bin: pi_bin.into(),
            timeout: LIST_MODELS_TIMEOUT,
        }
    }

    fn bin_for(&self, runtime: AgentRuntime) -> &str {
        match runtime {
            AgentRuntime::GentleShell => &self.gentle_shell_bin,
            AgentRuntime::Pi => &self.pi_bin,
        }
    }
}

#[async_trait::async_trait]
impl ModelCatalogPort for AgentCliModelCatalogAdapter {
    async fn list(&self, runtime: AgentRuntime) -> Result<Vec<ModelRef>, AgentSettingsError> {
        let bin: &str = self.bin_for(runtime);
        let output: Output = tokio::time::timeout(
            self.timeout,
            Command::new(bin).arg("--list-models").output(),
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
        let adapter: AgentCliModelCatalogAdapter =
            AgentCliModelCatalogAdapter::new("gentle-shell-bin", "pi-bin");

        assert_eq!(
            adapter.bin_for(AgentRuntime::GentleShell),
            "gentle-shell-bin"
        );
        assert_eq!(adapter.bin_for(AgentRuntime::Pi), "pi-bin");
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
