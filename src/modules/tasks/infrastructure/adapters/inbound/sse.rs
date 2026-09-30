//! Endpoint SSE con el historial y el stream en vivo de una tarea.
//!
//! Protocolo: primero un evento `reset`, luego el historial persistido y después los
//! eventos en vivo (`message` con un `TaskEventDto` en JSON). Si el suscriptor se
//! queda atrás se emite `lagged` y el cliente debe reconectar.

use std::convert::Infallible;

use axum::Router;
use axum::extract::{Path, State};
use axum::http::StatusCode;
use axum::response::sse::{Event, KeepAlive, Sse};
use axum::routing::get;
use futures::stream::{self, Stream, StreamExt};
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
    match serde_json::to_string(&dto) {
        Ok(json) => Event::default().data(json),
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

    // Suscribirse antes de leer el historial evita perder eventos intermedios.
    let receiver: tokio::sync::broadcast::Receiver<TaskEvent> = context.event_bus.subscribe();
    let history: Vec<TaskEvent> = context
        .get_task_events
        .execute(task_id, None)
        .await
        .map_err(|e: TaskError| (StatusCode::INTERNAL_SERVER_ERROR, e.to_string()))?;
    let last_seq: Option<i64> = history.last().and_then(|event: &TaskEvent| event.seq);

    let reset = stream::once(async { control_event(RESET_EVENT) });
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
