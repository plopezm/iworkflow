use leptos::prelude::*;

use crate::core::components::form::{ActionError, TextField};
use crate::core::components::layout::{Badge, Card, ErrorText};
use crate::modules::code_hosting::infrastructure::adapters::inbound::dto::code_host_dto::{
    CodeHostSettingsDto, SaveCodeHostSettingsDto,
};
use crate::modules::code_hosting::infrastructure::adapters::inbound::server_fns::{
    SaveCodeHostSettings, TestCodeHostConnection,
};
use crate::modules::code_hosting::presentation::state::code_host_settings_state::CodeHostSettingsState;

const HELP: &str = "URL de tu GitLab on-premise y un token personal (scope api). Se usa para crear merge requests, ver el CI y mezclar desde cada tarea; el proyecto se deduce del remoto origin del repositorio. La app nunca hace push: la rama debe estar ya publicada.";

/// Convierte el token escrito en el campo: vacío = conservar el guardado.
pub fn token_to_save(raw: &str) -> Option<String> {
    let trimmed: &str = raw.trim();
    (!trimmed.is_empty()).then(|| trimmed.to_string())
}

/// Tarjeta de configuración de GitLab (URL, token, TLS) con prueba de conexión.
#[component]
pub fn CodeHostSettingsCard() -> impl IntoView {
    let state: CodeHostSettingsState = CodeHostSettingsState::new();
    view! {
        <Card title="GitLab">
            <p class="muted text-sm">{HELP}</p>
            <Transition fallback=|| view! { <p class="muted">"Cargando…"</p> }>
                {move || Suspend::new(async move {
                    match state.settings.await {
                        Ok(settings) => view! { <CodeHostSettingsForm state settings /> }.into_any(),
                        Err(e) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                    }
                })}
            </Transition>
        </Card>
    }
}

#[component]
fn CodeHostSettingsForm(
    state: CodeHostSettingsState,
    settings: CodeHostSettingsDto,
) -> impl IntoView {
    let base_url: RwSignal<String> = RwSignal::new(settings.base_url);
    let token: RwSignal<String> = RwSignal::new(String::new());
    let insecure: RwSignal<bool> = RwSignal::new(settings.insecure_tls);
    let token_configured: bool = settings.token_configured;
    let saving: Signal<bool> = state.save.pending().into();
    let testing: Signal<bool> = state.test.pending().into();
    let token_placeholder: &str = if token_configured {
        "•••••• (déjalo vacío para conservarlo)"
    } else {
        "glpat-…"
    };

    let on_save = move |_| {
        state.save.dispatch(SaveCodeHostSettings {
            input: SaveCodeHostSettingsDto {
                base_url: base_url.get_untracked(),
                token: token_to_save(&token.get_untracked()),
                insecure_tls: insecure.get_untracked(),
            },
        });
    };
    let on_test = move |_| {
        state.test.dispatch(TestCodeHostConnection {});
    };

    view! {
        <TextField label="URL de GitLab" value=base_url placeholder="https://gitlab.miempresa.local" />
        <label class="stack gap-1.5">
            <span class="text-sm font-medium text-fg">"Token de acceso personal"</span>
            <input
                type="password"
                class="input"
                autocomplete="off"
                placeholder=token_placeholder
                prop:value=move || token.get()
                on:input=move |ev| token.set(event_target_value(&ev))
            />
        </label>
        <label class="row items-center gap-2 text-sm text-fg">
            <input
                type="checkbox"
                class="h-4 w-4"
                prop:checked=move || insecure.get()
                on:change=move |ev| insecure.set(event_target_checked(&ev))
            />
            "Aceptar certificado TLS no confiable (CA interna o autofirmado)"
        </label>
        <div class="row gap-2">
            <button type="button" class="btn-primary" disabled=move || saving.get() on:click=on_save>
                "Guardar"
            </button>
            <button
                type="button"
                class="btn-secondary"
                disabled=move || testing.get() || saving.get() || !token_configured
                on:click=on_test
            >
                "Probar conexión"
            </button>
        </div>
        <ActionError result=state.save.value().into() />
        {move || {
            state
                .test
                .value()
                .get()
                .map(|result: Result<String, ServerFnError>| match result {
                    Ok(user) => {
                        view! { <Badge text=format!("Conectado como {user}") variant="ok" /> }
                            .into_any()
                    }
                    Err(e) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                })
        }}
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn blank_token_means_keep_the_stored_one() {
        assert_eq!(token_to_save(""), None);
        assert_eq!(token_to_save("   "), None);
    }

    #[test]
    fn token_is_trimmed() {
        assert_eq!(token_to_save("  glpat-abc "), Some("glpat-abc".to_string()));
    }
}
