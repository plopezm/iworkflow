use leptos::prelude::*;
use leptos_meta::Title;
use leptos_router::components::A;

use crate::modules::terminal::presentation::components::terminal_view::TerminalView;
use crate::modules::terminal::presentation::state::terminal_page_state::TerminalPageState;

/// Página a altura completa con la terminal interactiva de un workspace.
/// Se reconstruye por cada id (el router reutiliza el componente).
#[component]
pub fn TerminalPage() -> impl IntoView {
    let state: TerminalPageState = TerminalPageState::new();

    view! {
        {move || {
            let workspace_id: String = state.workspace_id.get();
            view! {
                <Title text=move || state.title.get() />
                <section class="stack h-full min-h-0 gap-3 animate-slide-up">
                    <header class="row justify-between">
                        <h1 class="truncate text-base font-semibold text-fg">
                            {move || state.title.get()}
                        </h1>
                        <A href="/workspaces" attr:class="btn-secondary min-h-8 px-2.5 py-1 text-xs">
                            "Volver"
                        </A>
                    </header>
                    // TODO(task 4): temporal; TerminalView espera un id de sesión, no de workspace.
                    <TerminalView session_id=workspace_id />
                </section>
            }
        }}
    }
}
