use leptos::prelude::*;

use crate::core::components::layout::ErrorText;
use crate::core::components::select::{OptionSelect, SelectOption};
use crate::core::components::toast::ToastStack;
use crate::modules::agent_settings::infrastructure::adapters::inbound::dto::agent_settings_dto::{
    AgentSettingsDto, ModelOptionDto, ProfileCatalogDto,
};
use crate::modules::agent_settings::presentation::state::agent_settings_state::{
    AgentSettingsState, encode_model_value,
};

/// Runtime por defecto (el que representa la opción vacía del selector).
const DEFAULT_RUNTIME: &str = "gentle-shell";
/// Identificador del runtime `pi` (el dominio es `AgentRuntime::Pi`).
const PI_RUNTIME: &str = "pi";
const APPLIES_TO_NEW_TASKS_HINT: &str = "Los cambios se aplican a las tareas nuevas que se inicien, no a las que ya están en marcha ni a las que se reanudan.";

/// Icono de runtime (terminal) para el selector de implementación de agente.
fn runtime_icon() -> impl IntoView {
    view! {
        <svg viewBox="0 0 20 20" fill="none">
            <rect x="2.5" y="3.5" width="15" height="13" rx="2" stroke="currentColor" stroke-width="1.5" />
            <path
                d="M6 8L9 10.5L6 13M10.5 13H14"
                stroke="currentColor"
                stroke-width="1.5"
                stroke-linecap="round"
                stroke-linejoin="round"
            />
        </svg>
    }
}

/// Icono de modelo (cpu) para el selector de modelo.
fn model_icon() -> impl IntoView {
    view! {
        <svg viewBox="0 0 20 20" fill="none">
            <rect x="6" y="6" width="8" height="8" rx="1.5" stroke="currentColor" stroke-width="1.5" />
            <path
                d="M8.5 2.5V6M11.5 2.5V6M8.5 14V17.5M11.5 14V17.5M2.5 8.5H6M2.5 11.5H6M14 8.5H17.5M14 11.5H17.5"
                stroke="currentColor"
                stroke-width="1.5"
                stroke-linecap="round"
            />
        </svg>
    }
}

/// Icono de perfil (capas) para el selector de perfil.
fn profile_icon() -> impl IntoView {
    view! {
        <svg viewBox="0 0 20 20" fill="none">
            <path
                d="M10 3L17 7L10 11L3 7L10 3Z"
                stroke="currentColor"
                stroke-width="1.5"
                stroke-linejoin="round"
            />
            <path
                d="M3 10.5L10 14.5L17 10.5M3 14L10 18L17 14"
                stroke="currentColor"
                stroke-width="1.5"
                stroke-linecap="round"
                stroke-linejoin="round"
            />
        </svg>
    }
}

/// Icono de razonamiento (medidor) para el selector de nivel de pensamiento.
fn thinking_icon() -> impl IntoView {
    view! {
        <svg viewBox="0 0 20 20" fill="none">
            <path
                d="M3 14A7 7 0 0 1 17 14"
                stroke="currentColor"
                stroke-width="1.5"
                stroke-linecap="round"
            />
            <path
                d="M10 14L13 9"
                stroke="currentColor"
                stroke-width="1.5"
                stroke-linecap="round"
            />
            <circle cx="10" cy="14" r="1" fill="currentColor" />
        </svg>
    }
}

/// Etiqueta en español de un nivel de razonamiento (identificador de pi en
/// minúsculas -> texto mostrado en el selector).
fn thinking_option_label(id: &str) -> String {
    match id {
        "off" => "Sin razonamiento",
        "minimal" => "Mínimo",
        "low" => "Bajo",
        "medium" => "Medio",
        "high" => "Alto",
        "xhigh" => "Muy alto",
        "max" => "Máximo",
        other => other,
    }
    .to_string()
}

/// Etiqueta visible de una opción de perfil, marcando cuál es el activo globalmente.
fn profile_option_label(name: &str, active: Option<&str>) -> String {
    if active == Some(name) {
        format!("{name} (activo)")
    } else {
        name.to_string()
    }
}

/// Barra de ajustes del agente: selector de agente (gentle-shell/pi), de modelo (orquestador) y de perfil
/// (`gentle:profiles`), aplicados a las próximas tareas que se inicien. Cada selector
/// vive en su propia `Transition` para que una lista de modelos lenta (llama a
/// `gentle-shell --list-models`) no bloquee el resto de la barra ni de la página.
#[component]
pub fn AgentSettingsBar() -> impl IntoView {
    let state: AgentSettingsState = AgentSettingsState::new();
    let pending: Signal<bool> = state.update.pending().into();

    Effect::new(move |_| {
        if let Some(Err(e)) = state.update.value().get() {
            state.notify_error(e.to_string());
        }
    });

    view! {
        <div
            class="flex flex-col gap-1.5"
            title=APPLIES_TO_NEW_TASKS_HINT
        >
            <Transition fallback=|| view! {
                <span class="select-pill-skeleton" aria-hidden="true"></span>
            }>
                {move || Suspend::new(async move {
                    let settings: AgentSettingsDto = match state.settings.await {
                        Ok(settings) => settings,
                        Err(e) => return view! { <ErrorText message=e.to_string() /> }.into_any(),
                    };
                    let runtimes: Vec<String> = match state.runtimes.await {
                        Ok(runtimes) => runtimes,
                        Err(e) => return view! { <ErrorText message=e.to_string() /> }.into_any(),
                    };
                    let current_value: String = settings.runtime.clone().unwrap_or_default();
                    let options: Vec<SelectOption> = runtimes
                        .iter()
                        .filter(|id: &&String| id.as_str() != DEFAULT_RUNTIME)
                        .map(|id: &String| SelectOption::new(id.clone(), id.clone()))
                        .collect();
                    view! {
                        <OptionSelect
                            label="Agente"
                            value=current_value
                            options=options
                            default_label=DEFAULT_RUNTIME
                            on_change=Callback::new(move |value: String| {
                                state.select_runtime(&settings, value)
                            })
                            disabled=pending
                            icon=runtime_icon
                        />
                    }
                        .into_any()
                })}
            </Transition>
            <Transition fallback=|| view! {
                <span class="select-pill-skeleton" aria-hidden="true"></span>
            }>
                {move || Suspend::new(async move {
                    let settings: AgentSettingsDto = match state.settings.await {
                        Ok(settings) => settings,
                        Err(e) => return view! { <ErrorText message=e.to_string() /> }.into_any(),
                    };
                    let models: Vec<ModelOptionDto> = match state.models.await {
                        Ok(models) => models,
                        Err(e) => return view! { <ErrorText message=e.to_string() /> }.into_any(),
                    };
                    let current_value: String = match (&settings.model_provider, &settings.model_id) {
                        (Some(provider), Some(model)) => encode_model_value(provider, model),
                        _ => String::new(),
                    };
                    let options: Vec<SelectOption> = models
                        .iter()
                        .map(|m: &ModelOptionDto| {
                            SelectOption::grouped(
                                m.provider.clone(),
                                encode_model_value(&m.provider, &m.model),
                                m.model.clone(),
                            )
                        })
                        .collect();
                    view! {
                        <OptionSelect
                            label="Modelo"
                            value=current_value
                            options=options
                            default_label="Por defecto"
                            on_change=Callback::new(move |value: String| {
                                state.select_model(&settings, value)
                            })
                            disabled=pending
                            icon=model_icon
                        />
                    }
                        .into_any()
                })}
            </Transition>
            <Transition fallback=|| view! {
                <span class="select-pill-skeleton" aria-hidden="true"></span>
            }>
                {move || Suspend::new(async move {
                    let settings: AgentSettingsDto = match state.settings.await {
                        Ok(settings) => settings,
                        Err(e) => return view! { <ErrorText message=e.to_string() /> }.into_any(),
                    };
                    let levels: Vec<String> = match state.thinking_levels.await {
                        Ok(levels) => levels,
                        Err(e) => return view! { <ErrorText message=e.to_string() /> }.into_any(),
                    };
                    let current_value: String = settings.thinking.clone().unwrap_or_default();
                    let options: Vec<SelectOption> = levels
                        .iter()
                        .map(|id: &String| SelectOption::new(id.clone(), thinking_option_label(id)))
                        .collect();
                    view! {
                        <OptionSelect
                            label="Razonamiento"
                            value=current_value
                            options=options
                            default_label="Por defecto"
                            on_change=Callback::new(move |value: String| {
                                state.select_thinking(&settings, value)
                            })
                            disabled=pending
                            icon=thinking_icon
                        />
                    }
                        .into_any()
                })}
            </Transition>
            <Transition fallback=|| view! {
                <span class="select-pill-skeleton" aria-hidden="true"></span>
            }>
                {move || Suspend::new(async move {
                    let settings: AgentSettingsDto = match state.settings.await {
                        Ok(settings) => settings,
                        Err(e) => return view! { <ErrorText message=e.to_string() /> }.into_any(),
                    };
                    if settings.runtime.as_deref() == Some(PI_RUNTIME) {
                        // Los perfiles son de gentle-ai: no aplican con `pi`.
                        return ().into_any();
                    }
                    let catalog: ProfileCatalogDto = match state.profiles.await {
                        Ok(catalog) => catalog,
                        Err(e) => return view! { <ErrorText message=e.to_string() /> }.into_any(),
                    };
                    let current_value: String = settings.profile.clone().unwrap_or_default();
                    let active: Option<String> = catalog.active.clone();
                    let options: Vec<SelectOption> = catalog
                        .names
                        .iter()
                        .map(|name: &String| {
                            SelectOption::new(
                                name.clone(),
                                profile_option_label(name, active.as_deref()),
                            )
                        })
                        .collect();
                    view! {
                        <OptionSelect
                            label="Perfil"
                            value=current_value
                            options=options
                            default_label="Por defecto"
                            on_change=Callback::new(move |value: String| {
                                state.select_profile(&settings, value)
                            })
                            disabled=pending
                            icon=profile_icon
                        />
                    }
                        .into_any()
                })}
            </Transition>
            <p class="text-[11px] text-muted">"Se aplica a las tareas nuevas."</p>
            <ToastStack toasts=state.toasts />
        </div>
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn profile_label_marks_active_profile() {
        assert_eq!(
            profile_option_label("Claude", Some("Claude")),
            "Claude (activo)"
        );
        assert_eq!(profile_option_label("Claude", Some("Other")), "Claude");
        assert_eq!(profile_option_label("Claude", None), "Claude");
    }

    #[test]
    fn thinking_label_translates_known_ids() {
        assert_eq!(thinking_option_label("off"), "Sin razonamiento");
        assert_eq!(thinking_option_label("minimal"), "Mínimo");
        assert_eq!(thinking_option_label("low"), "Bajo");
        assert_eq!(thinking_option_label("medium"), "Medio");
        assert_eq!(thinking_option_label("high"), "Alto");
        assert_eq!(thinking_option_label("xhigh"), "Muy alto");
        assert_eq!(thinking_option_label("max"), "Máximo");
    }

    #[test]
    fn thinking_label_falls_back_to_the_raw_id() {
        assert_eq!(thinking_option_label("unknown"), "unknown");
    }
}
