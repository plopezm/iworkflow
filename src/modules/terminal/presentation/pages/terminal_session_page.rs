use crate::core::dock::link::{TabLink, TabRedirect};
use leptos::prelude::*;

use crate::core::components::form::ActionError;
use crate::modules::terminal::presentation::components::terminal_view::TerminalView;
use crate::modules::terminal::presentation::state::terminal_page_state::terminals_path;
use crate::modules::terminal::presentation::state::terminal_session_state::{
    TerminalSessionState, session_title, title_for,
};

/// Página a altura completa con la terminal interactiva de una sesión.
/// Se reconstruye por cada id (el router reutiliza el componente).
#[component]
pub fn TerminalSessionPage() -> impl IntoView {
    let state: TerminalSessionState = TerminalSessionState::new();

    view! {
        {move || {
            let session_id: String = state.session_id.get();
            let fallback_id: String = session_id.clone();
            let fallback_title = move || session_title(None, &fallback_id);
            let title_id: String = session_id.clone();
            let title_view = move || {
                let id: String = title_id.clone();
                Suspend::new(async move { title_for(&state.sessions.await, &id) })
            };
            view! {
                <Show when=move || state.closed.get()>
                    <TabRedirect path=terminals_path() />
                </Show>
                <section class="stack h-full min-h-0 gap-3 px-4 pt-4 pb-6 animate-slide-up">
                    <header class="row justify-between">
                        <h1 class="page-title truncate text-xl">
                            <Suspense fallback=fallback_title>
                                {title_view}
                            </Suspense>
                        </h1>
                        <div class="row gap-1.5">
                            <TabLink href=terminals_path() class="btn-secondary min-h-8 px-2.5 py-1 text-xs">
                                "Todas las terminales"
                            </TabLink>
                            <button
                                class="btn-danger min-h-8 px-2.5 py-1 text-xs"
                                disabled=move || state.close.pending().get()
                                on:click=move |_| state.close_session()
                            >
                                "Cerrar sesión"
                            </button>
                        </div>
                    </header>
                    <ActionError result=state.close.value().into() />
                    <TerminalView session_id=session_id />
                </section>
            }
        }}
    }
}
