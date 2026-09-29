use leptos::prelude::*;

/// Tiempo tras el cual una notificación se descarta sola.
#[cfg(feature = "hydrate")]
const TOAST_AUTO_DISMISS: std::time::Duration = std::time::Duration::from_secs(8);

/// Notificación efímera y descartable mostrada como pila flotante.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ToastMessage {
    pub id: u32,
    pub text: String,
}

/// Añade un mensaje a la pila de notificaciones con un id incremental.
pub fn push_toast(toasts: RwSignal<Vec<ToastMessage>>, next_id: RwSignal<u32>, text: String) {
    let id: u32 = next_id.get_untracked();
    next_id.set(id.wrapping_add(1));
    toasts.update(|list: &mut Vec<ToastMessage>| list.push(ToastMessage { id, text }));
}

/// Pila de notificaciones flotante, genérica y reutilizable entre features.
#[component]
pub fn ToastStack(toasts: RwSignal<Vec<ToastMessage>>) -> impl IntoView {
    view! {
        <div
            class="fixed inset-x-0 bottom-4 z-[60] flex flex-col items-center gap-2 px-4 sm:items-end sm:px-6"
            aria-live="polite"
        >
            <For
                each=move || toasts.get()
                key=|m: &ToastMessage| m.id
                children=move |msg: ToastMessage| view! { <ToastItem msg toasts /> }
            />
        </div>
    }
}

/// Una notificación: se descarta con su botón o automáticamente pasado `TOAST_AUTO_DISMISS`
#[component]
fn ToastItem(msg: ToastMessage, toasts: RwSignal<Vec<ToastMessage>>) -> impl IntoView {
    let id: u32 = msg.id;
    let dismiss = move || {
        toasts.update(|list: &mut Vec<ToastMessage>| list.retain(|m: &ToastMessage| m.id != id))
    };

    #[cfg(feature = "hydrate")]
    if let Ok(handle) = set_timeout_with_handle(dismiss, TOAST_AUTO_DISMISS) {
        on_cleanup(move || handle.clear());
    }

    view! {
        <div class="relative overflow-hidden rounded-xl border border-border/60 bg-surface/90 p-3 text-sm text-fg shadow-lg backdrop-blur-xl sm:w-auto animate-slide-up">
            <div class="absolute bottom-0 left-0 right-0 h-0.5 bg-gradient-to-r from-accent to-purple">
                #[cfg(feature = "hydrate")]
                {
                    let _ = msg.id;
                    view! {
                        <div
                            class="h-full bg-gradient-to-r from-accent to-purple"
                            style=move || "animation: shimmer 8s linear infinite; width: 100%;"
                        ></div>
                    }
                }
            </div>
            <div class="row items-start gap-3">
                <span class="flex-1">{msg.text}</span>
                <button
                    class="btn-ghost min-h-10 min-w-10 rounded-full px-2"
                    aria-label="Cerrar notificación"
                    on:click=move |_| dismiss()
                >
                    "✕"
                </button>
            </div>
        </div>
    }
}
