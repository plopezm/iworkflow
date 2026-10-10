#[cfg(feature = "hydrate")]
use leptos::prelude::*;

/// Sondeo lento de respaldo: las vistas se actualizan sobre todo vía SSE.
#[cfg(feature = "hydrate")]
const FALLBACK_POLL_INTERVAL: std::time::Duration = std::time::Duration::from_secs(60);

/// Espera antes de reabrir el stream SSE cuando el navegador lo cierra por completo.
#[cfg(feature = "hydrate")]
const SSE_RECONNECT_DELAY: std::time::Duration = std::time::Duration::from_secs(3);

/// Lo que el stream del tablero comunica a la vista.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum TaskStreamEvent {
    /// Reconexión, `lagged` o sondeo de respaldo: el listado completo puede estar desfasado.
    Reload,
    /// Una tarea cambió: basta con pedir esa tarea.
    Changed(String),
    /// Una tarea se eliminó.
    Removed(String),
}

#[cfg(not(feature = "hydrate"))]
pub fn subscribe_task_events(_on_event: impl Fn(TaskStreamEvent) + Clone + 'static) {}

/// Abre el stream SSE del tablero (una sola vez, desde el shell) y traduce sus eventos a
/// `TaskStreamEvent`: `reset`/`lagged` piden recarga y `changed`/`removed` llevan el id de
/// la tarea. Si el navegador cierra la conexión por completo (`readyState == CLOSED`), la
/// reabre tras `SSE_RECONNECT_DELAY` (los cortes recuperables ya los reintenta el propio
/// `EventSource`). Además mantiene un sondeo lento de respaldo por si el stream se corta
/// sin disparar `onerror` (p.ej. proxies que cierran la conexión en silencio).
#[cfg(feature = "hydrate")]
pub fn subscribe_task_events<F: Fn(TaskStreamEvent) + Clone + 'static>(on_event: F) {
    use wasm_bindgen::JsCast;
    use wasm_bindgen::closure::Closure;
    use web_sys::{Event, EventSource, MessageEvent};

    type Handler = Closure<dyn FnMut(Event)>;

    struct Connection {
        source: EventSource,
        _handlers: Vec<Handler>,
    }

    #[derive(Default)]
    struct SseState {
        connection: Option<Connection>,
        reconnect: Option<TimeoutHandle>,
    }

    /// Id de tarea que el servidor envía como dato de `changed`/`removed`.
    fn event_task_id(event: &Event) -> Option<String> {
        event
            .dyn_ref::<MessageEvent>()
            .and_then(|message: &MessageEvent| message.data().as_string())
            .filter(|id: &String| !id.is_empty())
    }

    /// `skip_first_reset`: el `reset` inicial solo confirma la conexión y los datos ya
    /// llegaron con el render/hidratación, así que no se vuelve a pedir el listado.
    fn open<F: Fn(TaskStreamEvent) + Clone + 'static>(
        on_event: F,
        slot: StoredValue<SseState, LocalStorage>,
        skip_first_reset: bool,
    ) {
        let Ok(source) = EventSource::new("/api/tasks/events") else {
            return;
        };
        let reset_pending_skip: std::rc::Rc<std::cell::Cell<bool>> =
            std::rc::Rc::new(std::cell::Cell::new(skip_first_reset));
        let reset_sink: F = on_event.clone();
        let on_reset: Handler = Closure::new(move |_: Event| {
            if reset_pending_skip.replace(false) {
                return;
            }
            reset_sink(TaskStreamEvent::Reload);
        });
        let lagged_sink: F = on_event.clone();
        let on_lagged: Handler = Closure::new(move |_: Event| lagged_sink(TaskStreamEvent::Reload));
        let changed_sink: F = on_event.clone();
        let on_changed: Handler = Closure::new(move |event: Event| {
            changed_sink(match event_task_id(&event) {
                Some(id) => TaskStreamEvent::Changed(id),
                None => TaskStreamEvent::Reload,
            });
        });
        let removed_sink: F = on_event.clone();
        let on_removed: Handler = Closure::new(move |event: Event| {
            removed_sink(match event_task_id(&event) {
                Some(id) => TaskStreamEvent::Removed(id),
                None => TaskStreamEvent::Reload,
            });
        });
        for (name, handler) in [
            ("reset", &on_reset),
            ("changed", &on_changed),
            ("removed", &on_removed),
            ("lagged", &on_lagged),
        ] {
            if source
                .add_event_listener_with_callback(name, handler.as_ref().unchecked_ref())
                .is_err()
            {
                leptos::logging::error!("no se pudo suscribir al evento SSE '{name}' del tablero");
            }
        }
        let watched_source: EventSource = source.clone();
        let on_error: Handler = Closure::new(move |_: Event| {
            if watched_source.ready_state() != EventSource::CLOSED {
                return;
            }
            let sink: F = on_event.clone();
            let handle: Option<TimeoutHandle> =
                set_timeout_with_handle(move || open(sink, slot, false), SSE_RECONNECT_DELAY).ok();
            slot.update_value(|state: &mut SseState| state.reconnect = handle);
        });
        if source
            .add_event_listener_with_callback("error", on_error.as_ref().unchecked_ref())
            .is_err()
        {
            leptos::logging::error!("no se pudo suscribir al evento de error SSE del tablero");
        }
        slot.update_value(|state: &mut SseState| {
            state.connection = Some(Connection {
                source,
                _handlers: vec![on_reset, on_lagged, on_changed, on_removed, on_error],
            });
        });
    }

    let slot: StoredValue<SseState, LocalStorage> = StoredValue::new_local(SseState::default());
    let fallback_sink: F = on_event.clone();
    open(on_event, slot, true);

    on_cleanup(move || {
        slot.update_value(|state: &mut SseState| {
            if let Some(connection) = state.connection.take() {
                connection.source.close();
            }
            if let Some(handle) = state.reconnect.take() {
                handle.clear();
            }
        });
    });

    if let Ok(handle) = set_interval_with_handle(
        move || fallback_sink(TaskStreamEvent::Reload),
        FALLBACK_POLL_INTERVAL,
    ) {
        on_cleanup(move || handle.clear());
    }
}
