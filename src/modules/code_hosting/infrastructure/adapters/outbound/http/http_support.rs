use reqwest::{RequestBuilder, Response, StatusCode};
use serde::Deserialize;
use serde_json::Value;

use crate::modules::code_hosting::domain::errors::code_host_error::CodeHostError;
use crate::modules::code_hosting::domain::models::hosting_provider::HostingProvider;

/// Codifica un componente de ruta (el proyecto `g/p` o una rama `pi/x`) para la API.
pub(super) fn encode_component(value: &str) -> String {
    let mut encoded: String = String::with_capacity(value.len());
    for byte in value.bytes() {
        if byte.is_ascii_alphanumeric() || matches!(byte, b'-' | b'_' | b'.' | b'~') {
            encoded.push(char::from(byte));
        } else {
            encoded.push_str(&format!("%{byte:02X}"));
        }
    }
    encoded
}

/// Envía la petición: éxito → respuesta; redirección → `Redirected` (nunca se sigue, para no
/// filtrar el token); resto de estados → `Api` con el mensaje del proveedor.
pub(super) async fn send(
    provider: HostingProvider,
    request: RequestBuilder,
) -> Result<Response, CodeHostError> {
    let response: Response = request
        .send()
        .await
        .map_err(|e: reqwest::Error| CodeHostError::Network(e.without_url().to_string()))?;
    let status: StatusCode = response.status();
    if status.is_success() {
        return Ok(response);
    }
    if status.is_redirection() {
        return Err(CodeHostError::Redirected {
            provider,
            status: status.as_u16(),
        });
    }
    Err(api_error(provider, response).await)
}

pub(super) async fn json<T: for<'de> Deserialize<'de>>(
    response: Response,
) -> Result<T, CodeHostError> {
    response
        .json::<T>()
        .await
        .map_err(|e: reqwest::Error| CodeHostError::Network(e.without_url().to_string()))
}

async fn api_error(provider: HostingProvider, response: Response) -> CodeHostError {
    let status: StatusCode = response.status();
    let body: String = response.text().await.unwrap_or_default();
    CodeHostError::Api {
        provider,
        status: status.as_u16(),
        message: error_message(&body, status),
    }
}

/// Detalle de `errors` de GitHub: su `message`, o `campo código` si no lo trae.
fn validation_details(body: &Value) -> Vec<String> {
    let Some(errors) = body.get("errors").and_then(Value::as_array) else {
        return Vec::new();
    };
    errors
        .iter()
        .filter_map(|entry: &Value| match entry {
            Value::String(text) => Some(text.clone()),
            Value::Object(_) => entry
                .get("message")
                .and_then(Value::as_str)
                .map(str::to_string)
                .or_else(|| {
                    let field: &str = entry.get("field").and_then(Value::as_str)?;
                    let code: &str = entry.get("code").and_then(Value::as_str)?;
                    Some(format!("{field} {code}"))
                }),
            _ => None,
        })
        .collect()
}

/// GitLab devuelve `{"message": ...}` (texto, lista u objeto) o `{"error": ...}`; GitHub,
/// `{"message": ..., "errors": [...]}` (el motivo real de un 422 va en `errors`).
fn error_message(body: &str, status: StatusCode) -> String {
    let parsed: Option<Value> = serde_json::from_str(body).ok();
    let message: Option<String> = parsed.and_then(|v: Value| {
        let field: &Value = v.get("message").or_else(|| v.get("error"))?;
        let text: String = match field {
            Value::String(text) => text.clone(),
            other => other.to_string(),
        };
        let details: Vec<String> = validation_details(&v);
        Some(if details.is_empty() {
            text
        } else {
            format!("{text}: {}", details.join("; "))
        })
    });
    message.unwrap_or_else(|| {
        if body.trim().is_empty() {
            status.canonical_reason().unwrap_or("error").to_string()
        } else {
            body.chars().take(300).collect()
        }
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn encodes_slashes_and_reserved_characters() {
        assert_eq!(
            encode_component("grupo/sub/p.q-r_s"),
            "grupo%2Fsub%2Fp.q-r_s"
        );
        assert_eq!(encode_component("a b#c"), "a%20b%23c");
    }

    #[test]
    fn error_message_prefers_message_then_error_then_body() {
        assert_eq!(
            error_message(r#"{"message":"nope"}"#, StatusCode::BAD_REQUEST),
            "nope"
        );
        assert_eq!(
            error_message(r#"{"error":"bad"}"#, StatusCode::BAD_REQUEST),
            "bad"
        );
        assert_eq!(error_message("texto", StatusCode::BAD_REQUEST), "texto");
        assert_eq!(error_message("", StatusCode::NOT_FOUND), "Not Found");
    }

    #[test]
    fn error_message_appends_github_validation_details() {
        assert_eq!(
            error_message(
                r#"{"message":"Validation Failed","errors":[{"resource":"PullRequest","code":"custom","message":"No commits between main and feat"},{"resource":"PullRequest","field":"head","code":"invalid"}]}"#,
                StatusCode::UNPROCESSABLE_ENTITY
            ),
            "Validation Failed: No commits between main and feat; head invalid"
        );
    }
}
