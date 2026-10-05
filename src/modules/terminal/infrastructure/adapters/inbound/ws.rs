//! Endpoint WebSocket con una sesión de terminal en el directorio de un workspace.
//!
//! Protocolo: servidor → cliente, frames binarios con la salida del PTY; cliente →
//! servidor, frame binario = bytes de entrada y frame de texto = JSON de redimensionado
//! (ver `ClientMessage`). El tamaño inicial llega en la query `cols`/`rows` (80x24 por
//! defecto). La sesión se cierra al cerrarse el socket o al terminar la shell.

use axum::Router;
use axum::extract::ws::{Message, WebSocket, WebSocketUpgrade};
use axum::extract::{Path, Query, State};
use axum::http::{HeaderMap, StatusCode, header};
use axum::response::{IntoResponse, Response};
use axum::routing::get;
use futures::{SinkExt, StreamExt};
use serde::Deserialize;
use tokio::sync::mpsc::Receiver;
use uuid::Uuid;

use crate::modules::terminal::domain::errors::terminal_error::TerminalError;
use crate::modules::terminal::domain::models::terminal_session::TerminalSession;
use crate::modules::terminal::domain::models::terminal_session_id::TerminalSessionId;
use crate::modules::terminal::infrastructure::adapters::inbound::dto::client_message::ClientMessage;
use crate::modules::terminal::infrastructure::di::terminal_context::TerminalContext;

pub const TERMINAL_WS_PATH: &str = "/terminal/ws/{workspace_id}";
const DEFAULT_COLS: u16 = 80;
const DEFAULT_ROWS: u16 = 24;

#[derive(Debug, Deserialize)]
pub struct TerminalQuery {
    cols: Option<u16>,
    rows: Option<u16>,
}

/// Ruta WebSocket del feature, lista para fusionarse en el router principal.
pub fn terminal_ws_router<S>(context: TerminalContext) -> Router<S>
where
    S: Clone + Send + Sync + 'static,
{
    Router::new()
        .route(TERMINAL_WS_PATH, get(terminal_ws))
        .with_state(context)
}

/// Extrae `host[:puerto]` de un valor de cabecera `Origin` (`scheme://host[:puerto]`).
fn origin_host(origin: &str) -> Option<&str> {
    let (_, rest) = origin.split_once("://")?;
    let host: &str = rest.split('/').next().unwrap_or(rest);
    (!host.is_empty()).then_some(host)
}

/// Un upgrade es aceptable si no trae `Origin` (cliente no navegador) o si el host del
/// `Origin` coincide con el de `Host`. Evita el secuestro de la shell desde otras webs.
pub fn is_same_origin(origin: Option<&str>, host: Option<&str>) -> bool {
    match (origin, host) {
        (None, _) => true,
        (Some(origin), Some(host)) => {
            origin_host(origin).is_some_and(|o: &str| o.eq_ignore_ascii_case(host))
        }
        (Some(_), None) => false,
    }
}

/// Mapea un error de dominio al estado HTTP devuelto antes del upgrade.
pub fn status_for(error: &TerminalError) -> StatusCode {
    match error {
        TerminalError::WorkspaceNotFound | TerminalError::SessionNotFound => StatusCode::NOT_FOUND,
        TerminalError::InvalidSize => StatusCode::BAD_REQUEST,
        TerminalError::Pty(_) => StatusCode::INTERNAL_SERVER_ERROR,
    }
}

fn header_str(headers: &HeaderMap, name: header::HeaderName) -> Option<&str> {
    headers.get(name).and_then(|v| v.to_str().ok())
}

async fn terminal_ws(
    State(context): State<TerminalContext>,
    Path(workspace_id): Path<String>,
    Query(query): Query<TerminalQuery>,
    headers: HeaderMap,
    upgrade: WebSocketUpgrade,
) -> Response {
    if !is_same_origin(
        header_str(&headers, header::ORIGIN),
        header_str(&headers, header::HOST),
    ) {
        return (StatusCode::FORBIDDEN, "origen no permitido").into_response();
    }
    let Ok(workspace_id) = Uuid::parse_str(&workspace_id) else {
        return (StatusCode::BAD_REQUEST, "id de workspace inválido").into_response();
    };

    let session: TerminalSession = match context
        .open
        .execute(
            workspace_id,
            query.cols.unwrap_or(DEFAULT_COLS),
            query.rows.unwrap_or(DEFAULT_ROWS),
        )
        .await
    {
        Ok(session) => session,
        Err(error) => return (status_for(&error), error.to_string()).into_response(),
    };
    let output: Receiver<Vec<u8>> = match context.subscribe.execute(session.id).await {
        Ok(output) => output,
        Err(error) => {
            if let Err(close_error) = context.close.execute(session.id).await {
                tracing::warn!(%close_error, "no se pudo cerrar la sesión de terminal");
            }
            return (status_for(&error), error.to_string()).into_response();
        }
    };

    upgrade.on_upgrade(move |socket: WebSocket| run_session(socket, context, session.id, output))
}

/// Puentea socket y PTY hasta que uno de los dos termina; después cierra la sesión.
async fn run_session(
    socket: WebSocket,
    context: TerminalContext,
    id: TerminalSessionId,
    mut output: Receiver<Vec<u8>>,
) {
    let (mut sink, mut stream) = socket.split();
    loop {
        tokio::select! {
            chunk = output.recv() => match chunk {
                Some(bytes) => {
                    if sink.send(Message::Binary(bytes.into())).await.is_err() {
                        break;
                    }
                }
                // La shell terminó: se cierra el socket.
                None => break,
            },
            frame = stream.next() => match frame {
                Some(Ok(Message::Binary(bytes))) => {
                    let message: ClientMessage = ClientMessage::from_binary(bytes.to_vec());
                    if !apply(&context, id, message).await {
                        break;
                    }
                }
                Some(Ok(Message::Text(text))) => match ClientMessage::from_text(text.as_str()) {
                    Ok(message) => {
                        if !apply(&context, id, message).await {
                            break;
                        }
                    }
                    Err(error) => tracing::debug!(%error, "mensaje de terminal ignorado"),
                },
                Some(Ok(Message::Ping(_) | Message::Pong(_))) => {}
                Some(Ok(Message::Close(_)) | Err(_)) | None => break,
            },
        }
    }

    if let Err(error) = context.close.execute(id).await {
        tracing::warn!(%error, "no se pudo cerrar la sesión de terminal");
    }
    // El cliente puede haberse ido ya: el cierre del socket es best-effort.
    if let Err(error) = sink.send(Message::Close(None)).await {
        tracing::debug!(%error, "socket de terminal ya cerrado");
    }
}

/// Aplica un mensaje del cliente. Devuelve `false` si la sesión ya no es utilizable.
async fn apply(context: &TerminalContext, id: TerminalSessionId, message: ClientMessage) -> bool {
    match message {
        ClientMessage::Input(bytes) => match context.write.execute(id, bytes).await {
            Ok(()) => true,
            Err(error) => {
                tracing::debug!(%error, "escritura al terminal fallida");
                false
            }
        },
        ClientMessage::Resize { cols, rows } => {
            match context.resize.execute(id, cols, rows).await {
                Ok(()) | Err(TerminalError::InvalidSize) => true,
                Err(error) => {
                    tracing::debug!(%error, "redimensionado del terminal fallido");
                    false
                }
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn same_host_origin_is_allowed() {
        assert!(is_same_origin(
            Some("http://localhost:3000"),
            Some("localhost:3000")
        ));
        assert!(is_same_origin(
            Some("https://Example.com"),
            Some("example.com")
        ));
        assert!(is_same_origin(Some("http://a.b:80/"), Some("a.b:80")));
    }

    #[test]
    fn cross_origin_is_rejected() {
        assert!(!is_same_origin(
            Some("http://evil.com"),
            Some("localhost:3000")
        ));
        assert!(!is_same_origin(
            Some("http://localhost:4000"),
            Some("localhost:3000")
        ));
        assert!(!is_same_origin(Some("null"), Some("localhost:3000")));
        assert!(!is_same_origin(Some("http://localhost:3000"), None));
    }

    #[test]
    fn missing_origin_is_allowed() {
        assert!(is_same_origin(None, Some("localhost:3000")));
        assert!(is_same_origin(None, None));
    }

    #[test]
    fn domain_errors_map_to_http_status() {
        assert_eq!(
            status_for(&TerminalError::WorkspaceNotFound),
            StatusCode::NOT_FOUND
        );
        assert_eq!(
            status_for(&TerminalError::SessionNotFound),
            StatusCode::NOT_FOUND
        );
        assert_eq!(
            status_for(&TerminalError::InvalidSize),
            StatusCode::BAD_REQUEST
        );
        assert_eq!(
            status_for(&TerminalError::Pty("x".to_string())),
            StatusCode::INTERNAL_SERVER_ERROR
        );
    }
}
