use crate::core::dock::link::{TabLink, TabRedirect};
use leptos::prelude::*;

use crate::core::i18n::Msg;

use crate::core::components::layout::{Card, ErrorText};
use crate::modules::terminal::infrastructure::adapters::inbound::dto::terminal_session_dto::TerminalSessionDto;
use crate::modules::terminal::presentation::state::new_task_terminal_state::NewTaskTerminalState;
use crate::modules::terminal::presentation::state::terminal_page_state::terminal_session_path;

const OPENING_TERMINAL: Msg = Msg {
    en: "Opening terminal",
    es: "Abriendo terminal",
    zh: "正在打开终端",
    de: "Terminal wird geöffnet",
    fr: "Ouverture du terminal",
};
const OPENING_TERMINAL_LOADING: Msg = Msg {
    en: "Opening terminal…",
    es: "Abriendo terminal…",
    zh: "正在打开终端…",
    de: "Terminal wird geöffnet…",
    fr: "Ouverture du terminal…",
};
const BACK_TO_BOARD: Msg = Msg {
    en: "Back to board",
    es: "Volver al tablero",
    zh: "返回看板",
    de: "Zurück zum Board",
    fr: "Retour au tableau",
};

/// Abre una sesión en el worktree de la tarea de la ruta y redirige a ella.
#[component]
pub fn NewTaskTerminalPage() -> impl IntoView {
    let state: NewTaskTerminalState = NewTaskTerminalState::new();

    view! {
        <section class="stack p-4 animate-slide-up">
            <Card title=OPENING_TERMINAL.get()>
                <Suspense fallback=|| view! { <p class="muted">{OPENING_TERMINAL_LOADING.get()}</p> }>
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
                                            {BACK_TO_BOARD.get()}
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
