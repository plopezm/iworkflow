use leptos::prelude::*;

/// Sondeo lento de respaldo: las vistas se actualizan sobre todo vía SSE.
#[cfg(feature = "hydrate")]
const FALLBACK_POLL_INTERVAL: std::time::Duration = std::time::Duration::from_secs(60);

/// Espera antes de reabrir el stream SSE cuando el navegador lo cierra por completo.
#[cfg(feature = "hydrate")]
const SSE_RECONNECT_DELAY: std::time::Duration = std::time::Duration::from_secs(3);

#[cfg(not(feature = "hydrate"))]
pub fn subscribe_task_events(_tick: RwSignal<u32>) {}

/// Abre el stream SSE del tablero: cualquier `reset`/`changed`/`lagged` refresca el
/// listado. Si el navegador cierra la conexión por completo (`readyState == CLOSED`),
/// la reabre tras `SSE_RECONNECT_DELAY` (los cortes recuperables ya los reintenta el
/// propio `EventSource`). Además mantiene un sondeo lento de respaldo por si el stream
/// se corta sin disparar `onerror` (p.ej. proxies que cierran la conexión en silencio).
#[cfg(feature = "hydrate")]
pub fn subscribe_task_events(tick: RwSignal<u32>) {
    use wasm_bindgen::JsCast;
    use wasm_bindgen::closure::Closure;
    use web_sys::{Event, EventSource};

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

    /// `skip_first_reset`: el `reset` inicial solo confirma la conexión y los datos ya
    /// llegaron con el render/hidratación, así que no se vuelve a pedir el listado.
    fn open(
        tick: RwSignal<u32>,
        slot: StoredValue<SseState, LocalStorage>,
        skip_first_reset: bool,
    ) {
        let Ok(source) = EventSource::new("/api/tasks/events") else {
            return;
        };
        let on_bump: Handler =
            Closure::new(move |_: Event| tick.update(|n: &mut u32| *n = n.wrapping_add(1)));
        let reset_pending_skip: std::rc::Rc<std::cell::Cell<bool>> =
            std::rc::Rc::new(std::cell::Cell::new(skip_first_reset));
        let on_reset: Handler = Closure::new(move |_: Event| {
            if reset_pending_skip.replace(false) {
                return;
            }
            tick.update(|n: &mut u32| *n = n.wrapping_add(1));
        });
        for (name, handler) in [
            ("reset", &on_reset),
            ("changed", &on_bump),
            ("lagged", &on_bump),
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
            let handle: Option<TimeoutHandle> =
                set_timeout_with_handle(move || open(tick, slot, false), SSE_RECONNECT_DELAY).ok();
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
                _handlers: vec![on_bump, on_reset, on_error],
            });
        });
    }

    let slot: StoredValue<SseState, LocalStorage> = StoredValue::new_local(SseState::default());
    open(tick, slot, true);

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
        move || tick.update(|n: &mut u32| *n = n.wrapping_add(1)),
        FALLBACK_POLL_INTERVAL,
    ) {
        on_cleanup(move || handle.clear());
    }
}
