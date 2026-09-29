use leptos::prelude::*;

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
            <For each=move || toasts.get() key=|m: &ToastMessage| m.id let(msg)>
                {
                    let id: u32 = msg.id;
                    view! {
                        <div class="row w-full max-w-sm items-start justify-between gap-3 rounded-lg border border-error/40 bg-surface p-3 text-sm text-fg shadow-lg sm:w-auto">
                            <span class="flex-1">{msg.text}</span>
                            <button
                                class="btn-ghost min-h-10 min-w-10 px-2"
                                aria-label="Cerrar notificación"
                                on:click=move |_| {
                                    toasts.update(|list: &mut Vec<ToastMessage>| list.retain(|m: &ToastMessage| m.id != id))
                                }
                            >
                                "x"
                            </button>
                        </div>
                    }
                }
            </For>
        </div>
    }
}
