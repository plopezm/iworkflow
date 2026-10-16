//! Modelos del agente de Cursor. `agent --list-models` usa ids que `session/set_model` de ACP
//! rechaza (`composer-2.5` frente a `composer-2.5[fast=true]`), así que el catálogo sale del
//! propio ACP: se abre una sesión desechable y se leen los `availableModels` de `session/new`.

use std::ffi::OsString;
use std::process::Stdio;
use std::time::Duration;

use serde_json::{Value, json};
use tokio::io::{AsyncBufReadExt, AsyncWriteExt, BufReader};
use tokio::process::{Child, ChildStdin, ChildStdout, Command};

use crate::core::env::{LoginShellInvocation, login_shell_invocation};
use crate::modules::agent_settings::domain::errors::agent_settings_error::AgentSettingsError;
use crate::modules::agent_settings::domain::models::model_ref::ModelRef;

/// `session/new` tarda varios segundos en `agent acp`.
pub const CURSOR_CATALOG_TIMEOUT: Duration = Duration::from_secs(30);
const CURSOR_PROVIDER: &str = "cursor";
const AUTH_METHOD: &str = "cursor_login";

/// Pasos del handshake hasta `session/new`, cuyo resultado trae los modelos.
fn handshake() -> [(&'static str, Value); 3] {
    [
        (
            "initialize",
            json!({
                "protocolVersion": 1,
                "clientCapabilities": { "fs": { "readTextFile": false, "writeTextFile": false }, "terminal": false },
                "clientInfo": { "name": "iworkflow", "version": env!("CARGO_PKG_VERSION") },
            }),
        ),
        ("authenticate", json!({ "methodId": AUTH_METHOD })),
        (
            "session/new",
            json!({ "cwd": std::env::temp_dir(), "mcpServers": [] }),
        ),
    ]
}

/// Modelos de `agent acp` para `bin`. Se limita con `timeout` y el proceso se mata al salir.
pub async fn list_cursor_models(
    bin: &str,
    vars: Vec<(OsString, OsString)>,
    timeout: Duration,
) -> Result<Vec<ModelRef>, AgentSettingsError> {
    tokio::time::timeout(timeout, query_models(bin, vars))
        .await
        .map_err(|_| {
            AgentSettingsError::Catalog("tiempo de espera agotado listando modelos".to_string())
        })?
}

async fn query_models(
    bin: &str,
    vars: Vec<(OsString, OsString)>,
) -> Result<Vec<ModelRef>, AgentSettingsError> {
    let invocation: LoginShellInvocation =
        login_shell_invocation(bin, &["acp".to_string()], vars.into_iter());
    let mut child: Child = Command::new(invocation.shell)
        .env_clear()
        .envs(invocation.env)
        .args(invocation.args)
        .current_dir(std::env::temp_dir())
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::null())
        .kill_on_drop(true)
        .spawn()
        .map_err(|e| AgentSettingsError::Catalog(format!("no se pudo ejecutar {bin}: {e}")))?;
    let mut stdin: ChildStdin = child
        .stdin
        .take()
        .ok_or_else(|| AgentSettingsError::Catalog("sin stdin".to_string()))?;
    let stdout: ChildStdout = child
        .stdout
        .take()
        .ok_or_else(|| AgentSettingsError::Catalog("sin stdout".to_string()))?;
    let mut lines: tokio::io::Lines<BufReader<ChildStdout>> = BufReader::new(stdout).lines();

    let mut last: Value = Value::Null;
    for (index, (method, params)) in handshake().into_iter().enumerate() {
        let id: usize = index + 1;
        let request: Value =
            json!({ "jsonrpc": "2.0", "id": id, "method": method, "params": params });
        stdin
            .write_all(format!("{request}\n").as_bytes())
            .await
            .map_err(|e| AgentSettingsError::Catalog(format!("{bin} cerró stdin: {e}")))?;
        stdin
            .flush()
            .await
            .map_err(|e| AgentSettingsError::Catalog(format!("{bin} cerró stdin: {e}")))?;
        last = response_to(&mut lines, id, method).await?;
    }
    // Cerrar stdin apaga el agente; `kill_on_drop` cubre que no salga.
    drop(stdin);
    Ok(parse_models(&last))
}

/// Lee líneas hasta la respuesta con `id`; los errores JSON-RPC se convierten en `Catalog`.
async fn response_to(
    lines: &mut tokio::io::Lines<BufReader<ChildStdout>>,
    id: usize,
    method: &str,
) -> Result<Value, AgentSettingsError> {
    loop {
        let line: String = lines
            .next_line()
            .await
            .map_err(|e| AgentSettingsError::Catalog(format!("error leyendo ACP: {e}")))?
            .ok_or_else(|| {
                AgentSettingsError::Catalog(format!("el agente terminó antes de {method}"))
            })?;
        let Ok(record) = serde_json::from_str::<Value>(&line) else {
            continue;
        };
        if record.get("id").and_then(Value::as_u64) != Some(id as u64) {
            continue;
        }
        if let Some(error) = record.get("error") {
            let message: &str = error
                .get("message")
                .and_then(Value::as_str)
                .unwrap_or("error desconocido");
            return Err(AgentSettingsError::Catalog(format!(
                "{method} falló (¿has ejecutado `agent login`?): {message}"
            )));
        }
        return Ok(record.get("result").cloned().unwrap_or(Value::Null));
    }
}

/// `models.availableModels[{modelId}]` de la respuesta de `session/new`.
pub fn parse_models(session_new_result: &Value) -> Vec<ModelRef> {
    session_new_result
        .get("models")
        .and_then(|m: &Value| m.get("availableModels"))
        .and_then(Value::as_array)
        .map(|models: &Vec<Value>| {
            models
                .iter()
                .filter_map(|m: &Value| m.get("modelId").and_then(Value::as_str))
                .map(|id: &str| ModelRef {
                    provider: CURSOR_PROVIDER.to_string(),
                    model: id.to_string(),
                })
                .collect()
        })
        .unwrap_or_default()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn models_keep_the_exact_acp_model_id() {
        let result: Value = json!({ "sessionId": "s", "models": { "currentModelId": "default[]",
            "availableModels": [
                { "modelId": "default[]", "name": "Auto" },
                { "modelId": "composer-2.5[fast=true]", "name": "composer-2.5" }
            ] } });

        let models: Vec<ModelRef> = parse_models(&result);

        assert_eq!(
            models,
            vec![
                ModelRef {
                    provider: "cursor".into(),
                    model: "default[]".into()
                },
                ModelRef {
                    provider: "cursor".into(),
                    model: "composer-2.5[fast=true]".into()
                },
            ]
        );
    }

    #[test]
    fn a_result_without_models_is_an_empty_catalog() {
        assert!(parse_models(&json!({ "sessionId": "s" })).is_empty());
        assert!(parse_models(&Value::Null).is_empty());
    }

    #[test]
    fn the_real_session_new_capture_lists_models() {
        let path: std::path::PathBuf = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("tests/fixtures/cursor_acp_session.jsonl");
        let recording: String = std::fs::read_to_string(path).expect("fixture");
        let record: Value = recording
            .lines()
            .filter_map(|l: &str| l.strip_prefix("<< "))
            .filter_map(|l: &str| serde_json::from_str::<Value>(l).ok())
            .find(|r: &Value| r.get("id").and_then(Value::as_u64) == Some(3))
            .expect("respuesta de session/new");

        let models: Vec<ModelRef> = parse_models(&record["result"]);

        assert!(models.len() > 3);
        assert!(models.iter().any(|m: &ModelRef| m.model == "default[]"));
    }

    /// Contra el `agent` real (requiere `agent login`): `cargo test ... -- --ignored`.
    #[tokio::test]
    #[ignore = "requiere el CLI de Cursor con sesión iniciada"]
    async fn real_cursor_agent_lists_acp_model_ids() {
        let models: Vec<ModelRef> = list_cursor_models(
            "agent",
            std::env::vars_os().collect(),
            CURSOR_CATALOG_TIMEOUT,
        )
        .await
        .expect("catálogo");

        assert!(
            models.iter().any(|m: &ModelRef| m.model == "default[]"),
            "{models:?}"
        );
    }
}
