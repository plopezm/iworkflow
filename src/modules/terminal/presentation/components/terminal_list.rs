use crate::core::dock::link::TabLink;
use leptos::prelude::*;

use crate::core::i18n::Msg;

use crate::modules::terminal::infrastructure::adapters::inbound::dto::terminal_session_dto::TerminalSessionDto;
use crate::modules::terminal::presentation::state::terminal_page_state::terminal_session_path;
use crate::modules::terminal::presentation::state::terminals_state::{
    short_id, started_at_label, status_class, status_label,
};

const NO_TERMINALS: Msg = Msg {
    en: "No open terminals.",
    es: "No hay terminales abiertas.",
    zh: "没有打开的终端。",
    de: "Keine Terminals geöffnet.",
    fr: "Aucun terminal ouvert.",
};
const WORKSPACE: Msg = Msg {
    en: "Workspace",
    es: "Workspace",
    zh: "工作区",
    de: "Workspace",
    fr: "Espace de travail",
};
const SESSION: Msg = Msg {
    en: "Session",
    es: "Sesión",
    zh: "会话",
    de: "Sitzung",
    fr: "Session",
};
const STARTED: Msg = Msg {
    en: "Started",
    es: "Inicio",
    zh: "开始时间",
    de: "Start",
    fr: "Début",
};
const STATUS: Msg = Msg {
    en: "Status",
    es: "Estado",
    zh: "状态",
    de: "Status",
    fr: "Statut",
};
const OPEN: Msg = Msg {
    en: "Open",
    es: "Abrir",
    zh: "打开",
    de: "Öffnen",
    fr: "Ouvrir",
};
const CLOSE: Msg = Msg {
    en: "Close",
    es: "Cerrar",
    zh: "关闭",
    de: "Schließen",
    fr: "Fermer",
};

/// Tabla de sesiones de terminal con acciones abrir/cerrar.
#[component]
pub fn TerminalList(
    sessions: Vec<TerminalSessionDto>,
    on_close: Callback<String>,
    #[prop(into)] closing: Signal<bool>,
) -> impl IntoView {
    if sessions.is_empty() {
        return view! { <p class="muted">{NO_TERMINALS.get()}</p> }.into_any();
    }

    view! {
        <div class="overflow-x-auto">
            <table class="data-table">
                <thead>
                    <tr>
                        <th>{WORKSPACE.get()}</th>
                        <th>{SESSION.get()}</th>
                        <th>{STARTED.get()}</th>
                        <th>{STATUS.get()}</th>
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
                                                {OPEN.get()}
                                            </TabLink>
                                            <button
                                                class="btn-danger min-h-8 px-2.5 py-1 text-xs"
                                                disabled=move || closing.get()
                                                on:click=move |_| on_close.run(id.clone())
                                            >
                                                {CLOSE.get()}
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
