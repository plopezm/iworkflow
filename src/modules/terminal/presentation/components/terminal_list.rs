use crate::core::dock::link::TabLink;
use leptos::prelude::*;

use crate::modules::terminal::infrastructure::adapters::inbound::dto::terminal_session_dto::TerminalSessionDto;
use crate::modules::terminal::presentation::state::terminal_page_state::terminal_session_path;
use crate::modules::terminal::presentation::state::terminals_state::{
    short_id, started_at_label, status_class, status_label,
};

/// Tabla de sesiones de terminal con acciones abrir/cerrar.
#[component]
pub fn TerminalList(
    sessions: Vec<TerminalSessionDto>,
    on_close: Callback<String>,
    #[prop(into)] closing: Signal<bool>,
) -> impl IntoView {
    if sessions.is_empty() {
        return view! { <p class="muted">"No hay terminales abiertas."</p> }.into_any();
    }

    view! {
        <div class="overflow-x-auto">
            <table class="data-table">
                <thead>
                    <tr>
                        <th>"Workspace"</th>
                        <th>"Sesión"</th>
                        <th>"Inicio"</th>
                        <th>"Estado"</th>
                        <th></th>
                    </tr>
                </thead>
                <tbody>
                    {sessions
                        .into_iter()
                        .map(|s: TerminalSessionDto| {
                            let id: String = s.id.clone();
                            view! {
                                <tr>
                                    <td class="font-medium text-fg">{s.workspace_name}</td>
                                    <td>
                                        <code class="font-mono text-xs text-muted">{short_id(&s.id)}</code>
                                    </td>
                                    <td class="text-muted">{started_at_label(&s.created_at)}</td>
                                    <td>
                                        <span class=status_class(s.status)>{status_label(s.status)}</span>
                                    </td>
                                    <td>
                                        <div class="row justify-end gap-1.5">
                                            <TabLink
                                                href=terminal_session_path(&s.id)
                                                class="btn-secondary min-h-8 px-2.5 py-1 text-xs"
                                            >
                                                "Abrir"
                                            </TabLink>
                                            <button
                                                class="btn-danger min-h-8 px-2.5 py-1 text-xs"
                                                disabled=move || closing.get()
                                                on:click=move |_| on_close.run(id.clone())
                                            >
                                                "Cerrar"
                                            </button>
                                        </div>
                                    </td>
                                </tr>
                            }
                        })
                        .collect_view()}
                </tbody>
            </table>
        </div>
    }
    .into_any()
}
