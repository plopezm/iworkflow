//! Endpoint HTTP que sirve el contenido de las imágenes adjuntas de una tarea.
//!
//! Los adjuntos son inmutables por id (cada subida genera un UUID nuevo y editar una tarea
//! solo añade o quita adjuntos), así que la respuesta es cacheable indefinidamente.

use axum::Router;
use axum::extract::{Path, State};
use axum::http::{HeaderName, HeaderValue, StatusCode, header};
use axum::response::{IntoResponse, Response};
use axum::routing::get;
use uuid::Uuid;

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::task::TaskId;
use crate::modules::tasks::domain::models::task_attachment_content::TaskAttachmentContent;
use crate::modules::tasks::infrastructure::di::tasks_context::TasksContext;

pub const TASK_ATTACHMENT_PATH: &str = "/api/tasks/{task_id}/attachments/{attachment_id}";
pub const IMMUTABLE_CACHE_CONTROL: &str = "private, max-age=31536000, immutable";

/// Rutas de adjuntos del feature, listas para fusionarse en el router principal.
pub fn task_attachments_router<S>(context: TasksContext) -> Router<S>
where
    S: Clone + Send + Sync + 'static,
{
    Router::new()
        .route(TASK_ATTACHMENT_PATH, get(serve_attachment))
        .with_state(context)
}

/// Ambos identificadores deben ser UUID: cualquier otra cosa (`..`, separadores, etc.) se
/// rechaza antes de tocar casos de uso o disco.
fn parse_ids(task_id: &str, attachment_id: &str) -> Option<(TaskId, String)> {
    let task: TaskId = TaskId::parse(task_id).ok()?;
    let attachment: Uuid = Uuid::parse_str(attachment_id).ok()?;
    Some((task, attachment.to_string()))
}

/// Traduce el resultado del caso de uso en la respuesta HTTP.
pub fn attachment_response(result: Result<TaskAttachmentContent, TaskError>) -> Response {
    match result {
        Ok(content) => match HeaderValue::from_str(&content.mime_type) {
            Ok(content_type) => (
                [
                    (header::CONTENT_TYPE, content_type),
                    (
                        header::CACHE_CONTROL,
                        HeaderValue::from_static(IMMUTABLE_CACHE_CONTROL),
                    ),
                    (
                        HeaderName::from_static("x-content-type-options"),
                        HeaderValue::from_static("nosniff"),
                    ),
                ],
                content.bytes,
            )
                .into_response(),
            Err(_) => StatusCode::INTERNAL_SERVER_ERROR.into_response(),
        },
        Err(TaskError::NotFound(_)) => StatusCode::NOT_FOUND.into_response(),
        Err(error) => {
            tracing::error!(%error, "no se pudo servir un adjunto");
            StatusCode::INTERNAL_SERVER_ERROR.into_response()
        }
    }
}

async fn serve_attachment(
    State(context): State<TasksContext>,
    Path((task_id, attachment_id)): Path<(String, String)>,
) -> Response {
    let Some((task, attachment)) = parse_ids(&task_id, &attachment_id) else {
        return StatusCode::NOT_FOUND.into_response();
    };
    attachment_response(context.get_task_attachment.execute(task, &attachment).await)
}

#[cfg(test)]
mod tests {
    use super::*;

    const TASK: &str = "0b8f6a52-3c1e-4d7a-9f10-6a2b3c4d5e6f";
    const ATTACHMENT: &str = "8e0c1f7e-2b34-4c5d-8a9b-0c1d2e3f4a5b";

    fn content() -> TaskAttachmentContent {
        TaskAttachmentContent {
            mime_type: "image/png".to_string(),
            bytes: vec![1, 2, 3],
        }
    }

    #[test]
    fn found_attachment_has_content_type_and_immutable_cache_headers() {
        let response: Response = attachment_response(Ok(content()));

        assert_eq!(response.status(), StatusCode::OK);
        assert_eq!(response.headers()[header::CONTENT_TYPE], "image/png");
        assert_eq!(
            response.headers()[header::CACHE_CONTROL],
            "private, max-age=31536000, immutable"
        );
        assert_eq!(response.headers()["x-content-type-options"], "nosniff");
    }

    #[tokio::test]
    async fn found_attachment_body_is_the_raw_bytes() {
        let response: Response = attachment_response(Ok(content()));

        let body: axum::body::Bytes = axum::body::to_bytes(response.into_body(), usize::MAX)
            .await
            .expect("body");

        assert_eq!(body.as_ref(), &[1, 2, 3]);
    }

    #[test]
    fn not_found_is_404_without_cache_headers() {
        let response: Response =
            attachment_response(Err(TaskError::NotFound("adjunto x".to_string())));

        assert_eq!(response.status(), StatusCode::NOT_FOUND);
        assert!(response.headers().get(header::CACHE_CONTROL).is_none());
    }

    #[test]
    fn other_errors_are_500() {
        let response: Response = attachment_response(Err(TaskError::Storage("disco".to_string())));

        assert_eq!(response.status(), StatusCode::INTERNAL_SERVER_ERROR);
        assert!(response.headers().get(header::CACHE_CONTROL).is_none());
    }

    #[test]
    fn ids_must_be_uuids() {
        assert!(parse_ids(TASK, ATTACHMENT).is_some());
        assert!(parse_ids("..", ATTACHMENT).is_none());
        assert!(parse_ids(TASK, "..").is_none());
        assert!(parse_ids(TASK, "../../etc/passwd").is_none());
        assert!(parse_ids(TASK, "a/b").is_none());
        assert!(parse_ids(TASK, "").is_none());
    }

    #[test]
    fn attachment_id_is_normalised_to_canonical_form() {
        let upper: String = ATTACHMENT.to_uppercase();

        let parsed: Option<(TaskId, String)> = parse_ids(TASK, &upper);

        assert_eq!(parsed.map(|(_, id)| id), Some(ATTACHMENT.to_string()));
    }
}
