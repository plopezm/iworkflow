use leptos::prelude::*;

use crate::core::components::layout::ErrorText;
use crate::core::i18n::Msg;
use crate::modules::jira::infrastructure::adapters::inbound::dto::jira_dto::JiraConnectionDto;
use crate::modules::jira::presentation::state::formatting::short_timestamp;
use crate::modules::jira::presentation::state::jira_connection_state::JiraConnectionState;

const NEVER: Msg = Msg {
    en: "never",
    es: "nunca",
    zh: "从未",
    de: "nie",
    fr: "jamais",
};

const SERVER: Msg = Msg {
    en: "Server: ",
    es: "Servidor: ",
    zh: "服务器：",
    de: "Server: ",
    fr: "Serveur : ",
};

const LAST_SYNC: Msg = Msg {
    en: "Last sync: ",
    es: "Última sincronización: ",
    zh: "上次同步：",
    de: "Letzte Synchronisierung: ",
    fr: "Dernière synchronisation : ",
};

const LAST_FAILURE: Msg = Msg {
    en: "Last failure: {error}",
    es: "Último fallo: {error}",
    zh: "上次失败：{error}",
    de: "Letzter Fehler: {error}",
    fr: "Dernier échec : {error}",
};

const EDIT: Msg = Msg {
    en: "Edit connection",
    es: "Editar conexión",
    zh: "编辑连接",
    de: "Verbindung bearbeiten",
    fr: "Modifier la connexion",
};

const FORGET_NOTICE: Msg = Msg {
    en: "The token and imported issues will be forgotten.",
    es: "Se olvidan el token y las tareas importadas.",
    zh: "令牌和已导入的任务将被清除。",
    de: "Token und importierte Aufgaben werden vergessen.",
    fr: "Le jeton et les tâches importées seront oubliés.",
};

const DISCONNECT: Msg = Msg {
    en: "Disconnect",
    es: "Desconectar",
    zh: "断开连接",
    de: "Trennen",
    fr: "Déconnecter",
};

const CANCEL: Msg = Msg {
    en: "Cancel",
    es: "Cancelar",
    zh: "取消",
    de: "Abbrechen",
    fr: "Annuler",
};

/// The stored connection with edit / disconnect actions.
#[component]
pub fn JiraConnectionSummary(
    connection: JiraConnectionDto,
    state: JiraConnectionState,
) -> impl IntoView {
    let for_edit: JiraConnectionDto = connection.clone();
    let last_sync: String = connection
        .last_sync_at
        .as_deref()
        .map(short_timestamp)
        .unwrap_or_else(|| NEVER.get().to_string());
    view! {
        <div class="stack gap-2">
            <p class="text-sm">
                <span class="muted">{SERVER.get()}</span>
                <strong>{connection.base_url}</strong>
            </p>
            <p class="text-sm">
                <span class="muted">{LAST_SYNC.get()}</span>
                {last_sync}
            </p>
            {connection
                .last_sync_error
                .map(|e: String| view! { <ErrorText message=LAST_FAILURE.fmt(&[("error", &e)]) /> })}
            <div class="row flex-wrap gap-2">
                <button
                    class="btn-secondary"
                    on:click=move |_| state.begin_edit(Some(&for_edit))
                >
                    {EDIT.get()}
                </button>
                {move || {
                    if state.confirming_delete.get() {
                        view! {
                            <span class="row items-center gap-2 text-sm">
                                {FORGET_NOTICE.get()}
                                <button
                                    class="btn-danger"
                                    disabled=move || state.delete.pending().get()
                                    class:is-busy=move || state.delete.pending().get()
                                    on:click=move |_| state.disconnect()
                                >
                                    {DISCONNECT.get()}
                                </button>
                                <button
                                    class="btn-ghost"
                                    on:click=move |_| state.confirming_delete.set(false)
                                >
                                    {CANCEL.get()}
                                </button>
                            </span>
                        }
                            .into_any()
                    } else {
                        view! {
                            <button
                                class="btn-ghost"
                                on:click=move |_| state.confirming_delete.set(true)
                            >
                                {DISCONNECT.get()}
                            </button>
                        }
                            .into_any()
                    }
                }}
            </div>
        </div>
    }
}
