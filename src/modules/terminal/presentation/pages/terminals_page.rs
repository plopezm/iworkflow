use crate::core::dock::link::TabRedirect;
use leptos::prelude::*;
use leptos_meta::Title;

use crate::core::components::form::ActionError;
use crate::core::components::layout::{Card, ErrorText};
use crate::modules::terminal::infrastructure::adapters::inbound::dto::terminal_session_dto::TerminalSessionDto;
use crate::modules::terminal::presentation::components::terminal_list::TerminalList;
use crate::modules::terminal::presentation::state::terminal_page_state::terminal_session_path;
use crate::modules::terminal::presentation::state::terminals_state::TerminalsState;

/// Listado de sesiones de terminal abiertas.
#[component]
pub fn TerminalsPage() -> impl IntoView {
    let state: TerminalsState = TerminalsState::new();
    let on_close: Callback<String> = Callback::new(move |id: String| state.close_session(id));

    view! {
        <Title text="Terminales" />
        <div class="flex h-full min-h-0 flex-col">
            <div class="flex shrink-0 items-center justify-between border-b border-border bg-surface px-4 py-2.5">
                <h1 class="page-title">"Terminales"</h1>
                <div class="row gap-1.5">
                    <button
                        class="btn-primary min-h-8 px-2.5 py-1 text-xs"
                        disabled=move || state.open_home.pending().get()
                        on:click=move |_| state.open_home_session()
                    >
                        "Nueva terminal en $HOME"
                    </button>
                    <button
                        class="btn-secondary min-h-8 px-2.5 py-1 text-xs"
                        on:click=move |_| state.reload()
                    >
                        "Actualizar"
                    </button>
                </div>
            </div>
            <div class="stack min-h-0 flex-1 overflow-y-auto p-4">
                <ActionError result=state.open_home.value().into() />
                {move || {
                    state
                        .open_home
                        .value()
                        .get()
                        .and_then(|r: Result<TerminalSessionDto, ServerFnError>| r.ok())
                        .map(|s: TerminalSessionDto| {
                            view! { <TabRedirect path=terminal_session_path(&s.id) /> }
                        })
                }}
                <Card title="Sesiones">
                    <ActionError result=state.close.value().into() />
                    <Transition fallback=|| view! { <p class="muted">"Cargando…"</p> }>
                        {move || Suspend::new(async move {
                            match state.sessions.await {
                                Ok(list) => {
                                    let list: Vec<TerminalSessionDto> = list;
                                    view! {
                                        <TerminalList
                                            sessions=list
                                            on_close
                                            closing=state.close.pending()
                                        />
                                    }
                                        .into_any()
                                }
                                Err(e) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                            }
                        })}
                    </Transition>
                </Card>
            </div>
        </div>
    }
}
