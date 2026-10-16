use leptos::prelude::*;

use crate::core::components::form::{ActionError, NumberInput};
use crate::core::components::layout::{Card, ErrorText};
use crate::modules::agent_settings::infrastructure::adapters::inbound::dto::agent_settings_dto::AgentSettingsDto;
use crate::modules::agent_settings::presentation::state::agent_settings_state::AgentSettingsState;

const LIMITS_HELP: &str = "Limita la RAM y la CPU del agente de cada tarea (solo Linux con systemd). Cada tarea puede fijar sus propios valores, que prevalecen sobre estos. Se aplica a las tareas que se inicien o reanuden a partir de ahora.";

/// Pantalla de configuración: límites de recursos del agente.
#[component]
pub fn SettingsPage(
    /// Secciones adicionales (las aporta el composition root para no acoplar features).
    #[prop(optional)]
    children: Option<Children>,
) -> impl IntoView {
    let state: AgentSettingsState = AgentSettingsState::shared();
    let pending: Signal<bool> = state.update.pending().into();

    view! {
        <div class="flex h-full min-h-0 flex-col">
            <div class="flex shrink-0 items-center justify-between border-b border-border bg-surface px-4 py-2.5">
                <h1 class="page-title">"Configuración"</h1>
            </div>
            <div class="stack min-h-0 flex-1 overflow-y-auto p-4">
                <Transition fallback=|| view! { <p class="muted">"Cargando…"</p> }>
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
                            <Card title="Límites de recursos">
                                <p class="muted text-sm">{LIMITS_HELP}</p>
                                <label class="row items-center gap-2 text-sm text-fg">
                                    <input
                                        type="checkbox"
                                        class="h-4 w-4"
                                        prop:checked=enabled
                                        disabled=move || pending.get()
                                        on:change=move |ev| {
                                            state
                                                .set_limits_enabled(
                                                    &for_toggle,
                                                    event_target_checked(&ev),
                                                )
                                        }
                                    />
                                    "Limitar recursos del agente"
                                </label>
                                <div class="row flex-wrap gap-2">
                                    <NumberInput
                                        label="RAM"
                                        value=memory
                                        unit="MiB"
                                        placeholder="sin límite"
                                        on_change=Callback::new(move |value: String| {
                                            state.set_memory_limit(&for_memory, value)
                                        })
                                        disabled=inputs_disabled
                                    />
                                    <NumberInput
                                        label="CPU"
                                        value=cpu
                                        unit="%"
                                        placeholder="sin límite"
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
                {children.map(|extra: Children| extra())}
            </div>
        </div>
    }
}
