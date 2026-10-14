use crate::core::dock::link::{TabLink, TabRedirect};
use leptos::prelude::*;
use leptos_meta::Title;

use crate::core::components::layout::{Card, ErrorText};
use crate::modules::terminal::infrastructure::adapters::inbound::dto::terminal_session_dto::TerminalSessionDto;
use crate::modules::terminal::presentation::state::new_task_terminal_state::NewTaskTerminalState;
use crate::modules::terminal::presentation::state::terminal_page_state::terminal_session_path;

/// Abre una sesión en el worktree de la tarea de la ruta y redirige a ella.
#[component]
pub fn NewTaskTerminalPage() -> impl IntoView {
    let state: NewTaskTerminalState = NewTaskTerminalState::new();

    view! {
        <Title text="Terminal" />
        <section class="stack p-4 animate-slide-up">
            <Card title="Abriendo terminal">
                <Suspense fallback=|| view! { <p class="muted">"Abriendo terminal…"</p> }>
                    {move || Suspend::new(async move {
                        match state.opened.await {
                            Ok(session) => {
                                let session: TerminalSessionDto = session;
                                view! { <TabRedirect path=terminal_session_path(&session.id) /> }
                                    .into_any()
                            }
                            Err(e) => {
                                view! {
                                    <div class="stack">
                                        <ErrorText message=e.to_string() />
                                        <TabLink href="/" class="btn-secondary w-fit">
                                            "Volver al tablero"
                                        </TabLink>
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
