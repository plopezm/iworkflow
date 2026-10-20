use leptos::prelude::*;

use crate::core::components::busy::Loading;

use crate::core::components::form::{ActionError, NumberInput};
use crate::core::components::layout::{Card, ErrorText};
use crate::core::components::segmented_control::{SegmentOption, SegmentedControl};
use crate::core::i18n::Msg;
use crate::modules::agent_settings::infrastructure::adapters::inbound::dto::agent_settings_dto::AgentSettingsDto;
use crate::modules::agent_settings::presentation::state::agent_settings_state::AgentSettingsState;

const LIMITS_HELP: Msg = Msg {
    en: "Limits the RAM and CPU of each task's agent (Linux with systemd only). Each task can set its own values, which take precedence over these. Applies to tasks started or resumed from now on.",
    es: "Limita la RAM y la CPU del agente de cada tarea (solo Linux con systemd). Cada tarea puede fijar sus propios valores, que prevalecen sobre estos. Se aplica a las tareas que se inicien o reanuden a partir de ahora.",
    zh: "限制每个任务中智能体的内存和 CPU（仅限使用 systemd 的 Linux）。每个任务可以设置自己的值，并优先于此处的值。适用于从现在起启动或恢复的任务。",
    de: "Begrenzt RAM und CPU des Agents jeder Aufgabe (nur Linux mit systemd). Jede Aufgabe kann eigene Werte festlegen, die Vorrang vor diesen haben. Gilt für Aufgaben, die ab jetzt gestartet oder fortgesetzt werden.",
    fr: "Limite la RAM et le CPU de l'agent de chaque tâche (Linux avec systemd uniquement). Chaque tâche peut définir ses propres valeurs, qui prévalent sur celles-ci. S'applique aux tâches démarrées ou reprises à partir de maintenant.",
};
const PAGE_TITLE: Msg = Msg {
    en: "Settings",
    es: "Configuración",
    zh: "设置",
    de: "Einstellungen",
    fr: "Paramètres",
};
const LOADING: Msg = Msg {
    en: "Loading…",
    es: "Cargando…",
    zh: "加载中…",
    de: "Wird geladen…",
    fr: "Chargement…",
};
const RESOURCE_LIMITS: Msg = Msg {
    en: "Resource limits",
    es: "Límites de recursos",
    zh: "资源限制",
    de: "Ressourcenlimits",
    fr: "Limites de ressources",
};
const LIMIT_AGENT_RESOURCES: Msg = Msg {
    en: "Limit agent resources",
    es: "Limitar recursos del agente",
    zh: "限制智能体资源",
    de: "Agent-Ressourcen begrenzen",
    fr: "Limiter les ressources de l'agent",
};
const NO_LIMIT: Msg = Msg {
    en: "no limit",
    es: "sin límite",
    zh: "无限制",
    de: "kein Limit",
    fr: "sans limite",
};

const SECTIONS_LABEL: Msg = Msg {
    en: "Settings sections",
    es: "Secciones de configuración",
    zh: "设置分区",
    de: "Einstellungsbereiche",
    fr: "Sections des paramètres",
};

/// One subtab of the settings screen: a stable id, its translated label and its content.
#[derive(Clone)]
pub struct SettingsSection {
    pub id: &'static str,
    pub label: String,
    pub view: ViewFn,
}

/// Id of the section to show: the selected one if it exists, otherwise the first.
pub fn active_section_id(sections: &[SettingsSection], selected: &str) -> Option<&'static str> {
    sections
        .iter()
        .find(|section: &&SettingsSection| section.id == selected)
        .or_else(|| sections.first())
        .map(|section: &SettingsSection| section.id)
}

/// Pantalla de configuración: una subpestaña por sección, aportadas por la raíz de composición.
#[component]
pub fn SettingsPage(sections: Vec<SettingsSection>) -> impl IntoView {
    let selected: RwSignal<String> = RwSignal::new(String::new());
    let options: Vec<SegmentOption> = sections
        .iter()
        .map(|section: &SettingsSection| SegmentOption {
            value: section.id.to_string(),
            label: section.label.clone(),
        })
        .collect();
    let active: Signal<String> = Signal::derive({
        let sections: Vec<SettingsSection> = sections.clone();
        move || {
            active_section_id(&sections, &selected.get())
                .unwrap_or_default()
                .to_string()
        }
    });

    view! {
        <div class="flex h-full min-h-0 flex-col">
            <div class="flex shrink-0 flex-wrap items-center justify-between gap-2 border-b border-border bg-surface px-4 py-2.5">
                <h1 class="page-title">{PAGE_TITLE.get()}</h1>
                <SegmentedControl
                    options=Signal::stored(options)
                    selected=active
                    aria_label=SECTIONS_LABEL.get().to_string()
                    on_select=Callback::new(move |id: String| selected.set(id))
                />
            </div>
            <div class="stack min-h-0 flex-1 overflow-y-auto p-4">
                {move || {
                    let current: String = active.get();
                    sections
                        .iter()
                        .find(|section: &&SettingsSection| section.id == current)
                        .map(|section: &SettingsSection| section.view.run())
                }}
            </div>
        </div>
    }
}

/// Card with the agent's RAM and CPU limits.
#[component]
pub fn ResourceLimitsCard() -> impl IntoView {
    let state: AgentSettingsState = AgentSettingsState::shared();
    let pending: Signal<bool> = state.update.pending().into();

    view! {
                <Transition fallback=|| view! { <Loading text=LOADING.get() /> }>
                    {move || Suspend::new(async move {
                        let settings: AgentSettingsDto = match state.settings.await {
                            Ok(settings) => settings,
                            Err(e) => {
                                return view! { <ErrorText message=e.to_string() /> }.into_any();
                            }
                        };
                        let memory: String = settings
                            .memory_limit_mb
                            .map(|v: u32| v.to_string())
                            .unwrap_or_default();
                        let cpu: String = settings
                            .cpu_limit_percent
                            .map(|v: u32| v.to_string())
                            .unwrap_or_default();
                        let enabled: bool = !settings.limits_disabled;
                        let for_toggle: AgentSettingsDto = settings.clone();
                        let for_memory: AgentSettingsDto = settings.clone();
                        let inputs_disabled: Signal<bool> =
                            Signal::derive(move || pending.get() || !enabled);
                        view! {
                            <Card title=RESOURCE_LIMITS.get()>
                                <p class="muted text-sm">{LIMITS_HELP.get()}</p>
                                <label class="row items-center gap-2 text-sm text-fg">
                                    <input
                                        type="checkbox"
                                        class="h-4 w-4"
                                        prop:checked=enabled
                                        disabled=move || pending.get()
                                        class:is-busy=move || pending.get()
                                        on:change=move |ev| {
                                            state
                                                .set_limits_enabled(
                                                    &for_toggle,
                                                    event_target_checked(&ev),
                                                )
                                        }
                                    />
                                    {LIMIT_AGENT_RESOURCES.get()}
                                </label>
                                <div class="row flex-wrap gap-2">
                                    <NumberInput
                                        label="RAM"
                                        value=memory
                                        unit="MiB"
                                        placeholder=NO_LIMIT.get()
                                        on_change=Callback::new(move |value: String| {
                                            state.set_memory_limit(&for_memory, value)
                                        })
                                        disabled=inputs_disabled
                                    />
                                    <NumberInput
                                        label="CPU"
                                        value=cpu
                                        unit="%"
                                        placeholder=NO_LIMIT.get()
                                        on_change=Callback::new(move |value: String| {
                                            state.set_cpu_limit(&settings, value)
                                        })
                                        disabled=inputs_disabled
                                    />
                                </div>
                                <ActionError result=state.update.value().into() />
                            </Card>
                        }
                            .into_any()
                    })}
                </Transition>
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn section(id: &'static str) -> SettingsSection {
        SettingsSection {
            id,
            label: id.to_string(),
            view: ViewFn::from(|| ()),
        }
    }

    #[test]
    fn active_section_is_the_selected_one() {
        let sections: Vec<SettingsSection> = vec![section("a"), section("b")];
        assert_eq!(active_section_id(&sections, "b"), Some("b"));
    }

    #[test]
    fn active_section_falls_back_to_first() {
        let sections: Vec<SettingsSection> = vec![section("a"), section("b")];
        assert_eq!(active_section_id(&sections, ""), Some("a"));
        assert_eq!(active_section_id(&sections, "zzz"), Some("a"));
    }

    #[test]
    fn no_sections_means_no_active_section() {
        assert_eq!(active_section_id(&[], "a"), None);
    }
}
