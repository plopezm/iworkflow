use leptos::ev::SubmitEvent;
use leptos::prelude::*;

use crate::core::components::form::{ActionError, TextField};
use crate::core::i18n::Msg;
use crate::modules::jira::presentation::state::jira_connection_state::{
    JiraConnectionState, can_submit,
};

const TOKEN_KEEP_PLACEHOLDER: Msg = Msg {
    en: "Empty = keep the stored token",
    es: "Vacío = conservar el token guardado",
    zh: "留空 = 保留已保存的令牌",
    de: "Leer = gespeichertes Token behalten",
    fr: "Vide = conserver le jeton enregistré",
};

const TOKEN_PLACEHOLDER: Msg = Msg {
    en: "Personal access token",
    es: "Token de acceso personal",
    zh: "个人访问令牌",
    de: "Persönliches Zugriffstoken",
    fr: "Jeton d'accès personnel",
};

const SERVER_URL: Msg = Msg {
    en: "Server URL",
    es: "URL del servidor",
    zh: "服务器 URL",
    de: "Server-URL",
    fr: "URL du serveur",
};

const SERVER_URL_PLACEHOLDER: Msg = Msg {
    en: "https://jira.mycompany.com",
    es: "https://jira.miempresa.com",
    zh: "https://jira.mycompany.com",
    de: "https://jira.meinefirma.com",
    fr: "https://jira.monentreprise.com",
};

const TOKEN_LABEL: Msg = Msg {
    en: "User token (PAT)",
    es: "Token de usuario (PAT)",
    zh: "用户令牌 (PAT)",
    de: "Benutzer-Token (PAT)",
    fr: "Jeton utilisateur (PAT)",
};

const ACCEPT_INVALID_CERTS: Msg = Msg {
    en: "Accept unverified certificate (self-signed or internal CA)",
    es: "Aceptar certificado no verificado (autofirmado o CA interna)",
    zh: "接受未验证的证书（自签名或内部 CA）",
    de: "Nicht verifiziertes Zertifikat akzeptieren (selbstsigniert oder interne CA)",
    fr: "Accepter un certificat non vérifié (auto-signé ou AC interne)",
};

const CANCEL: Msg = Msg {
    en: "Cancel",
    es: "Cancelar",
    zh: "取消",
    de: "Abbrechen",
    fr: "Annuler",
};

const CHECKING: Msg = Msg {
    en: "Checking…",
    es: "Comprobando…",
    zh: "检查中…",
    de: "Wird geprüft…",
    fr: "Vérification…",
};

const CONNECT: Msg = Msg {
    en: "Connect",
    es: "Conectar",
    zh: "连接",
    de: "Verbinden",
    fr: "Connecter",
};

/// URL + personal access token of the Jira Server/Data Center. The server validates
/// them against Jira before storing; the stored token is never shown.
#[component]
pub fn JiraConnectionForm(
    state: JiraConnectionState,
    /// URL of the stored connection, if any: leaving the token blank keeps the stored token, but
    /// only while the URL still points at that server.
    stored_url: Option<String>,
) -> impl IntoView {
    let token_stored: bool = stored_url.is_some();
    let pending: Signal<bool> = state.save.pending().into();
    let submittable: Memo<bool> = Memo::new(move |_| {
        can_submit(
            &state.form_url.get(),
            &state.form_token.get(),
            stored_url.as_deref(),
        )
    });
    let token_placeholder: &str = if token_stored {
        TOKEN_KEEP_PLACEHOLDER.get()
    } else {
        TOKEN_PLACEHOLDER.get()
    };
    view! {
        <form
            class="stack"
            on:submit=move |ev: SubmitEvent| {
                ev.prevent_default();
                state.submit();
            }
        >
            <TextField
                label=SERVER_URL.get()
                value=state.form_url
                placeholder=SERVER_URL_PLACEHOLDER.get()
                required=true
            />
            <label class="stack gap-1.5">
                <span class="text-sm font-medium text-fg">{TOKEN_LABEL.get()}</span>
                <input
                    type="password"
                    class="input"
                    autocomplete="off"
                    placeholder=token_placeholder
                    prop:value=move || state.form_token.get()
                    on:input=move |ev| state.form_token.set(event_target_value(&ev))
                />
            </label>
            <label class="row min-h-10 items-center gap-2 text-sm text-fg">
                <input
                    type="checkbox"
                    class="h-5 w-5"
                    prop:checked=move || state.form_accept_invalid_certs.get()
                    on:change=move |ev| {
                        state.form_accept_invalid_certs.set(event_target_checked(&ev))
                    }
                />
                {ACCEPT_INVALID_CERTS.get()}
            </label>
            <ActionError result=state.save.value().into() />
            <div class="row justify-end gap-2">
                <button
                    type="button"
                    class="btn-secondary"
                    on:click=move |_| state.cancel_edit()
                >
                    {CANCEL.get()}
                </button>
                <button
                    type="submit"
                    class="btn-primary"
                    disabled=move || pending.get() || !submittable.get()
                >
                    {move || if pending.get() { CHECKING.get() } else { CONNECT.get() }}
                </button>
            </div>
        </form>
    }
}
