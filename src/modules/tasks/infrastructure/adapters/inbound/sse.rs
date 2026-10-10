//! Endpoint SSE con el historial y el stream en vivo de una tarea.
//!
//! Protocolo: primero un evento `reset`, luego el historial persistido y después los
//! eventos en vivo (`message` con un `TaskEventDto` en JSON). Cada evento persistido
//! lleva `id: <seq>`; los efímeros no llevan id. Si la petición trae `Last-Event-ID` (o
//! `?after=<seq>`) solo se reenvían los eventos posteriores y no se emite `reset`, porque
//! el cliente conserva lo ya recibido. Si el suscriptor se queda atrás se emite `lagged`
//! y el cliente debe reconectar.

use std::convert::Infallible;

use axum::Router;
use axum::extract::{Path, Query, State};
use axum::http::{HeaderMap, StatusCode};
use axum::response::sse::{Event, KeepAlive, Sse};
use axum::routing::get;
use futures::stream::{self, Stream, StreamExt};
use serde::Deserialize;
use tokio_stream::wrappers::BroadcastStream;
use tokio_stream::wrappers::errors::BroadcastStreamRecvError;

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::agent_event::AgentEvent;
use crate::modules::tasks::domain::models::task::TaskId;
use crate::modules::tasks::domain::models::task_event::{TaskEvent, TaskEventKind};
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_event_dto::TaskEventDto;
use crate::modules::tasks::infrastructure::di::tasks_context::TasksContext;

pub const TASK_EVENTS_PATH: &str = "/api/tasks/{id}/events";
/// Endpoint global del tablero: un `changed` por tarea que cambia de estado o necesita atención.
pub const BOARD_EVENTS_PATH: &str = "/api/tasks/events";
pub const RESET_EVENT: &str = "reset";
pub const LAGGED_EVENT: &str = "lagged";
pub const CHANGED_EVENT: &str = "changed";
const LAST_EVENT_ID_HEADER: &str = "last-event-id";

#[derive(Debug, Default, Deserialize)]
struct ReplayQuery {
    after: Option<String>,
}

fn parse_seq(raw: &str) -> Option<i64> {
    raw.trim().parse::<i64>().ok().filter(|seq: &i64| *seq >= 0)
}

/// Cursor de reanudación: `Last-Event-ID` manda sobre `?after=`; valores inválidos se ignoran.
pub fn parse_replay_cursor(last_event_id: Option<&str>, after: Option<&str>) -> Option<i64> {
    last_event_id
        .and_then(parse_seq)
        .or_else(|| after.and_then(parse_seq))
}

/// Los navegadores descartan eventos SSE sin datos (y axum omite `data:` si está vacío),
/// así que los eventos de control llevan su nombre como dato.
fn control_event(name: &'static str) -> Event {
    Event::default().event(name).data(name)
}

/// Rutas SSE del feature, listas para fusionarse en el router principal.
pub fn task_events_router<S>(context: TasksContext) -> Router<S>
where
    S: Clone + Send + Sync + 'static,
{
    Router::new()
        .route(TASK_EVENTS_PATH, get(stream_task_events))
        .route(BOARD_EVENTS_PATH, get(stream_board_events))
        .with_state(context)
}

fn to_sse(event: TaskEvent) -> Event {
    let dto: TaskEventDto = TaskEventDto::from(event);
    let seq: Option<i64> = dto.seq;
    match serde_json::to_string(&dto) {
        Ok(json) => match seq {
            Some(seq) => Event::default().id(seq.to_string()).data(json),
            None => Event::default().data(json),
        },
        Err(error) => Event::default().event("error").data(error.to_string()),
    }
}

/// Filtra el stream en vivo: solo eventos de la tarea y posteriores al historial enviado.
pub fn live_event(
    item: Result<TaskEvent, BroadcastStreamRecvError>,
    task_id: TaskId,
    last_seq: Option<i64>,
) -> Option<Event> {
    match item {
        Ok(event)
            if event.task_id == task_id
                && event.seq.is_none_or(|seq: i64| Some(seq) > last_seq) =>
        {
            Some(to_sse(event))
        }
        Ok(_) => None,
        Err(BroadcastStreamRecvError::Lagged(_)) => Some(control_event(LAGGED_EVENT)),
    }
}

/// El tablero solo necesita saber qué tarea cambió, para refetchear su estado.
fn is_board_relevant(kind: &TaskEventKind) -> bool {
    match kind {
        TaskEventKind::Status { .. } | TaskEventKind::Changed | TaskEventKind::Removed => true,
        TaskEventKind::Agent { event } => matches!(
            event,
            AgentEvent::DialogRequested(_)
                | AgentEvent::DialogResolved { .. }
                | AgentEvent::ProcessExited { .. }
                | AgentEvent::SubagentsChanged { .. }
        ),
    }
}

/// Filtra el stream en vivo para el tablero: un `changed` con el id de la tarea por cada
/// evento de estado o de agente relevante (diálogo abierto/resuelto, proceso terminado).
pub fn board_event(item: Result<TaskEvent, BroadcastStreamRecvError>) -> Option<Event> {
    match item {
        Ok(event) if is_board_relevant(&event.kind) => Some(
            Event::default()
                .event(CHANGED_EVENT)
                .data(event.task_id.to_string()),
        ),
        Ok(_) => None,
        Err(BroadcastStreamRecvError::Lagged(_)) => Some(control_event(LAGGED_EVENT)),
    }
}

async fn stream_board_events(
    State(context): State<TasksContext>,
) -> Sse<impl Stream<Item = Result<Event, Infallible>>> {
    let receiver: tokio::sync::broadcast::Receiver<TaskEvent> = context.event_bus.subscribe();
    let reset = stream::once(async { control_event(RESET_EVENT) });
    let live = BroadcastStream::new(receiver).filter_map(|item| async move { board_event(item) });
    let events = reset.chain(live).map(Ok::<Event, Infallible>);
    Sse::new(events).keep_alive(KeepAlive::default())
}

async fn stream_task_events(
    State(context): State<TasksContext>,
    Path(id): Path<String>,
    Query(query): Query<ReplayQuery>,
    headers: HeaderMap,
) -> Result<Sse<impl Stream<Item = Result<Event, Infallible>>>, (StatusCode, String)> {
    let task_id: TaskId =
        TaskId::parse(&id).map_err(|e| (StatusCode::BAD_REQUEST, e.to_string()))?;
    context
        .get_task
        .execute(task_id)
        .await
        .map_err(|e: TaskError| match e {
            TaskError::NotFound(_) => (StatusCode::NOT_FOUND, e.to_string()),
            other => (StatusCode::INTERNAL_SERVER_ERROR, other.to_string()),
        })?;

    let last_event_id: Option<&str> = headers
        .get(LAST_EVENT_ID_HEADER)
        .and_then(|value: &axum::http::HeaderValue| value.to_str().ok());
    let cursor: Option<i64> = parse_replay_cursor(last_event_id, query.after.as_deref());

    // Suscribirse antes de leer el historial evita perder eventos intermedios.
    let receiver: tokio::sync::broadcast::Receiver<TaskEvent> = context.event_bus.subscribe();
    let history: Vec<TaskEvent> = context
        .get_task_events
        .execute(task_id, cursor)
        .await
        .map_err(|e: TaskError| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;
    let last_seq: Option<i64> = history
        .last()
        .and_then(|event: &TaskEvent| event.seq)
        .or(cursor);

    // Con cursor el cliente conserva su transcript: no se reinicia.
    let reset = stream::iter(cursor.is_none().then(|| control_event(RESET_EVENT)));
    let past = stream::iter(history.into_iter().map(to_sse));
    let live = BroadcastStream::new(receiver)
        .filter_map(move |item| async move { live_event(item, task_id, last_seq) });
    let events = reset.chain(past).chain(live).map(Ok::<Event, Infallible>);

    Ok(Sse::new(events).keep_alive(KeepAlive::default()))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::domain::models::agent_event::AgentEvent;
    use crate::modules::tasks::domain::models::task_event::TaskEventKind;

    fn event(task_id: TaskId, seq: Option<i64>) -> TaskEvent {
        TaskEvent {
            task_id,
            seq,
            kind: TaskEventKind::Agent {
                event: AgentEvent::RunStarted,
            },
            created_at: chrono::Utc::now(),
        }
    }

    #[test]
    fn replay_cursor_prefers_header_and_ignores_invalid_values() {
        assert_eq!(parse_replay_cursor(None, None), None);
        assert_eq!(parse_replay_cursor(Some("12"), Some("3")), Some(12));
        assert_eq!(parse_replay_cursor(None, Some(" 3 ")), Some(3));
        assert_eq!(parse_replay_cursor(Some("abc"), Some("3")), Some(3));
        assert_eq!(parse_replay_cursor(Some("-1"), None), None);
        assert_eq!(parse_replay_cursor(Some(""), Some("x")), None);
        assert_eq!(parse_replay_cursor(Some("0"), None), Some(0));
    }

    #[test]
    fn persisted_events_carry_id_and_ephemeral_do_not() {
        let task_id: TaskId = TaskId::new();
        let persisted: String = format!("{:?}", to_sse(event(task_id, Some(9))));
        let ephemeral: String = format!("{:?}", to_sse(event(task_id, None)));
        assert!(persisted.contains("id: 9"), "{persisted}");
        assert!(!ephemeral.contains("id:"), "{ephemeral}");
    }

    #[test]
    fn control_events_carry_data() {
        let rendered: String = format!("{:?}", control_event(RESET_EVENT));
        assert!(rendered.contains("data: reset"), "{rendered}");
    }

    #[test]
    fn live_filter_skips_other_tasks_and_already_sent_history() {
        let task_id: TaskId = TaskId::new();

        assert!(live_event(Ok(event(TaskId::new(), None)), task_id, None).is_none());
        assert!(live_event(Ok(event(task_id, Some(5))), task_id, Some(5)).is_none());
        assert!(live_event(Ok(event(task_id, Some(6))), task_id, Some(5)).is_some());
        assert!(live_event(Ok(event(task_id, None)), task_id, Some(5)).is_some());
        assert!(live_event(Err(BroadcastStreamRecvError::Lagged(3)), task_id, None).is_some());
    }

    fn with_kind(kind: TaskEventKind) -> TaskEvent {
        TaskEvent {
            task_id: TaskId::new(),
            seq: None,
            kind,
            created_at: chrono::Utc::now(),
        }
    }

    #[test]
    fn board_forwards_status_and_lifecycle_notifications() {
        assert!(
            board_event(Ok(with_kind(TaskEventKind::status(
                crate::modules::tasks::domain::models::task::TaskStatus::Idle,
                None
            ))))
            .is_some()
        );
        assert!(board_event(Ok(with_kind(TaskEventKind::Changed))).is_some());
        assert!(board_event(Ok(with_kind(TaskEventKind::Removed))).is_some());
    }

    #[test]
    fn board_forwards_only_specific_agent_events() {
        let dialog_requested = TaskEventKind::Agent {
            event: AgentEvent::DialogRequested(
                crate::modules::tasks::domain::models::agent_event::DialogRequest {
                    id: "d1".to_string(),
                    title: "t".to_string(),
                    kind: crate::modules::tasks::domain::models::agent_event::DialogKind::Confirm {
                        message: None,
                    },
                    timeout_ms: None,
                },
            ),
        };
        let dialog_resolved = TaskEventKind::Agent {
            event: AgentEvent::DialogResolved {
                dialog_id: "d1".to_string(),
            },
        };
        let process_exited = TaskEventKind::Agent {
            event: AgentEvent::ProcessExited { code: Some(0) },
        };
        let text_delta = TaskEventKind::Agent {
            event: AgentEvent::TextDelta {
                delta: "hi".to_string(),
            },
        };

        assert!(board_event(Ok(with_kind(dialog_requested))).is_some());
        assert!(board_event(Ok(with_kind(dialog_resolved))).is_some());
        assert!(board_event(Ok(with_kind(process_exited))).is_some());
        assert!(board_event(Ok(with_kind(text_delta))).is_none());
    }

    #[test]
    fn board_forwards_lagged_as_control_event() {
        assert!(board_event(Err(BroadcastStreamRecvError::Lagged(2))).is_some());
    }

    #[test]
    fn board_event_carries_task_id_as_data() {
        let event: TaskEvent = with_kind(TaskEventKind::Changed);
        let task_id: TaskId = event.task_id;
        let rendered: String = format!("{:?}", board_event(Ok(event)).expect("evento"));
        assert!(rendered.contains(&task_id.to_string()), "{rendered}");
    }
}
