use leptos::prelude::*;
use leptos_meta::Title;
use leptos_router::components::{A, Redirect};

use crate::core::components::layout::{Card, ErrorText};
use crate::modules::terminal::infrastructure::adapters::inbound::dto::terminal_session_dto::TerminalSessionDto;
use crate::modules::terminal::presentation::state::new_terminal_state::NewTerminalState;
use crate::modules::terminal::presentation::state::terminal_page_state::terminal_session_path;

/// Abre una sesión para el workspace de la ruta y redirige a ella.
#[component]
pub fn NewTerminalPage() -> impl IntoView {
    let state: NewTerminalState = NewTerminalState::new();

    view! {
        <Title text="Terminal" />
        <section class="stack p-4 animate-slide-up">
            <Card title="Abriendo terminal">
                <Suspense fallback=|| view! { <p class="muted">"Abriendo terminal…"</p> }>
                    {move || Suspend::new(async move {
                        match state.opened.await {
                            Ok(session) => {
                                let session: TerminalSessionDto = session;
                                view! { <Redirect path=terminal_session_path(&session.id) /> }
                                    .into_any()
                            }
                            Err(e) => {
                                view! {
                                    <div class="stack">
                                        <ErrorText message=e.to_string() />
                                        <A href="/workspaces" attr:class="btn-secondary w-fit">
                                            "Volver a workspaces"
                                        </A>
                                    </div>
                                }
                                    .into_any()
                            }
                        }
                    })}
                </Suspense>
            </Card>
        </section>
    }
}
