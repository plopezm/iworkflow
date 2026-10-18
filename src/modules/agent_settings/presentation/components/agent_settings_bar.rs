use leptos::prelude::*;

use crate::core::components::layout::ErrorText;
use crate::core::components::select::{OptionSelect, SelectOption};
use crate::core::components::toast::ToastStack;
use crate::core::i18n::Msg;
use crate::modules::agent_settings::infrastructure::adapters::inbound::dto::agent_settings_dto::{
    AgentSettingsDto, ModelOptionDto, ProfileCatalogDto,
};
use crate::modules::agent_settings::presentation::state::agent_settings_state::{
    AgentSettingsState, encode_model_value,
};

/// Runtime por defecto (el que representa la opción vacía del selector).
const DEFAULT_RUNTIME: &str = "gentle-shell";
const APPLIES_TO_NEW_TASKS_HINT: Msg = Msg {
    en: "Changes apply to newly started tasks, not to ones already running or being resumed.",
    es: "Los cambios se aplican a las tareas nuevas que se inicien, no a las que ya están en marcha ni a las que se reanudan.",
    zh: "更改适用于新启动的任务，不适用于已在运行或正在恢复的任务。",
    de: "Änderungen gelten für neu gestartete Aufgaben, nicht für bereits laufende oder fortgesetzte.",
    fr: "Les modifications s'appliquent aux nouvelles tâches démarrées, pas à celles déjà en cours ni à celles qui sont reprises.",
};
const AGENT: Msg = Msg {
    en: "Agent",
    es: "Agente",
    zh: "智能体",
    de: "Agent",
    fr: "Agent",
};
const MODEL: Msg = Msg {
    en: "Model",
    es: "Modelo",
    zh: "模型",
    de: "Modell",
    fr: "Modèle",
};
const PROFILE: Msg = Msg {
    en: "Profile",
    es: "Perfil",
    zh: "配置文件",
    de: "Profil",
    fr: "Profil",
};
const THINKING: Msg = Msg {
    en: "Reasoning",
    es: "Razonamiento",
    zh: "推理",
    de: "Reasoning",
    fr: "Raisonnement",
};
const DEFAULT: Msg = Msg {
    en: "Default",
    es: "Por defecto",
    zh: "默认",
    de: "Standard",
    fr: "Par défaut",
};
const APPLIES_TO_NEW_TASKS: Msg = Msg {
    en: "Applies to new tasks.",
    es: "Se aplica a las tareas nuevas.",
    zh: "适用于新任务。",
    de: "Gilt für neue Aufgaben.",
    fr: "S'applique aux nouvelles tâches.",
};
const ACTIVE_SUFFIX: Msg = Msg {
    en: "{name} (active)",
    es: "{name} (activo)",
    zh: "{name}（当前）",
    de: "{name} (aktiv)",
    fr: "{name} (actif)",
};
const THINKING_OFF: Msg = Msg {
    en: "No reasoning",
    es: "No reasoning",
    zh: "无推理",
    de: "Kein Reasoning",
    fr: "Sans raisonnement",
};
const THINKING_MINIMAL: Msg = Msg {
    en: "Minimal",
    es: "Mínimo",
    zh: "最低",
    de: "Minimal",
    fr: "Minimal",
};
const THINKING_LOW: Msg = Msg {
    en: "Low",
    es: "Bajo",
    zh: "低",
    de: "Niedrig",
    fr: "Faible",
};
const THINKING_MEDIUM: Msg = Msg {
    en: "Medium",
    es: "Medium",
    zh: "中",
    de: "Mittel",
    fr: "Moyen",
};
const THINKING_HIGH: Msg = Msg {
    en: "High",
    es: "Alto",
    zh: "高",
    de: "Hoch",
    fr: "Élevé",
};
const THINKING_XHIGH: Msg = Msg {
    en: "Very high",
    es: "Very high",
    zh: "非常高",
    de: "Sehr hoch",
    fr: "Très élevé",
};
const THINKING_MAX: Msg = Msg {
    en: "Maximum",
    es: "Maximum",
    zh: "最高",
    de: "Maximal",
    fr: "Maximum",
};

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

/// Etiqueta traducida de un nivel de razonamiento (identificador de pi en
/// minúsculas -> texto mostrado en el selector).
fn thinking_option_label(id: &str) -> String {
    let label: Option<Msg> = match id {
        "off" => Some(THINKING_OFF),
        "minimal" => Some(THINKING_MINIMAL),
        "low" => Some(THINKING_LOW),
        "medium" => Some(THINKING_MEDIUM),
        "high" => Some(THINKING_HIGH),
        "xhigh" => Some(THINKING_XHIGH),
        "max" => Some(THINKING_MAX),
        _ => None,
    };
    label.map_or_else(|| id.to_string(), |msg: Msg| msg.get().to_string())
}

/// Etiqueta visible de una opción de perfil, marcando cuál es el activo globalmente.
fn profile_option_label(name: &str, active: Option<&str>) -> String {
    if active == Some(name) {
        ACTIVE_SUFFIX.fmt(&[("name", name)])
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
    let state: AgentSettingsState = AgentSettingsState::shared();
    let pending: Signal<bool> = state.update.pending().into();

    Effect::new(move |_| {
        if let Some(Err(e)) = state.update.value().get() {
            state.notify_error(e.to_string());
        }
    });

    view! {
        <div
            class="flex flex-col gap-1.5"
            title=APPLIES_TO_NEW_TASKS_HINT.get()
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
                            label=AGENT.get()
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
                            label=MODEL.get()
                            value=current_value
                            options=options
                            default_label=DEFAULT.get()
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
                            label=THINKING.get()
                            value=current_value
                            options=options
                            default_label=DEFAULT.get()
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
                            label=PROFILE.get()
                            value=current_value
                            options=options
                            default_label=DEFAULT.get()
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
            <p class="text-[11px] text-muted">{APPLIES_TO_NEW_TASKS.get()}</p>
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
            "Claude (active)"
        );
        assert_eq!(profile_option_label("Claude", Some("Other")), "Claude");
        assert_eq!(profile_option_label("Claude", None), "Claude");
    }

    #[test]
    fn thinking_label_translates_known_ids() {
        assert_eq!(thinking_option_label("off"), "No reasoning");
        assert_eq!(thinking_option_label("minimal"), "Minimal");
        assert_eq!(thinking_option_label("low"), "Low");
        assert_eq!(thinking_option_label("medium"), "Medium");
        assert_eq!(thinking_option_label("high"), "High");
        assert_eq!(thinking_option_label("xhigh"), "Very high");
        assert_eq!(thinking_option_label("max"), "Maximum");
    }

    #[test]
    fn thinking_label_falls_back_to_the_raw_id() {
        assert_eq!(thinking_option_label("unknown"), "unknown");
    }
}
