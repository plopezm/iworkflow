use std::collections::BTreeMap;

use leptos::prelude::*;

use crate::core::components::form::SelectOption;
use crate::modules::agent_profiles::infrastructure::adapters::inbound::dto::agent_profiles_dto::{
    AgentRoleDto, ModelOptionDto, RoutingEntryDto,
};

/// Clave reservada del agente orquestador; siempre se muestra la primera.
pub const ORCHESTRATOR_KEY: &str = "orchestrator";

/// Fila editable del enrutado: agente, modelo y nivel de razonamiento (vacío = heredar).
#[derive(Debug, Clone, PartialEq, Eq, Default)]
pub struct RoutingRow {
    pub agent: String,
    pub model: String,
    pub thinking: String,
}

/// Convierte el mapa de enrutado en filas, con `orchestrator` primero y el resto por orden alfabético.
pub fn rows_from_routing(routing: &BTreeMap<String, RoutingEntryDto>) -> Vec<RoutingRow> {
    let to_row = |(agent, entry): (&String, &RoutingEntryDto)| RoutingRow {
        agent: agent.clone(),
        model: entry.model.clone().unwrap_or_default(),
        thinking: entry.thinking.clone().unwrap_or_default(),
    };
    let orchestrator: Option<RoutingRow> = routing.get_key_value(ORCHESTRATOR_KEY).map(to_row);
    let others = routing
        .iter()
        .filter(|(agent, _): &(&String, &RoutingEntryDto)| agent.as_str() != ORCHESTRATOR_KEY)
        .map(to_row);
    orchestrator.into_iter().chain(others).collect()
}

/// Convierte las filas en el mapa de enrutado: ignora agentes en blanco, recorta espacios,
/// mapea textos vacíos a `None` y, ante agentes repetidos, gana la última fila.
pub fn routing_from_rows(rows: &[RoutingRow]) -> BTreeMap<String, RoutingEntryDto> {
    let non_empty = |raw: &str| Some(raw.trim().to_string()).filter(|v: &String| !v.is_empty());
    rows.iter()
        .filter_map(|row: &RoutingRow| {
            let agent: String = row.agent.trim().to_string();
            if agent.is_empty() {
                return None;
            }
            Some((
                agent,
                RoutingEntryDto {
                    model: non_empty(&row.model),
                    thinking: non_empty(&row.thinking),
                },
            ))
        })
        .collect()
}

/// Opciones de modelo para el selector a partir de las del catálogo.
pub fn model_select_options(options: &[ModelOptionDto]) -> Vec<SelectOption> {
    options
        .iter()
        .map(|o: &ModelOptionDto| SelectOption {
            value: o.id.clone(),
            label: o.label.clone(),
        })
        .collect()
}

/// Nombres de los roles de agente.
pub fn role_names(roles: &[AgentRoleDto]) -> Vec<String> {
    roles
        .iter()
        .map(|role: &AgentRoleDto| role.name.clone())
        .collect()
}

/// Mensaje del error de carga de un `Resource`, si lo hay.
pub fn load_error<T>(loaded: &Option<Result<T, ServerFnError>>) -> Option<String> {
    match loaded {
        Some(Err(error)) => Some(error.to_string()),
        _ => None,
    }
}

/// Etiqueta de la opción "sin valor" del nivel de razonamiento.
const INHERIT_LABEL: &str = "Heredar / por defecto";

/// Niveles de razonamiento ofrecidos para una fila: los conocidos más el valor actual si no
/// figura entre ellos (para no perderlo al renderizar).
fn thinking_choices(levels: &[String], current: &str) -> Vec<String> {
    let mut choices: Vec<String> = levels.to_vec();
    if !current.is_empty() && !choices.iter().any(|level: &String| level == current) {
        choices.push(current.to_string());
    }
    choices
}

/// Opciones de modelo ofrecidas para una fila: las del catálogo más el valor actual si no
/// figura entre ellas (para no perderlo al renderizar).
fn model_choices(options: &[SelectOption], current: &str) -> Vec<SelectOption> {
    let mut choices: Vec<SelectOption> = options.to_vec();
    if !current.is_empty() && !choices.iter().any(|o: &SelectOption| o.value == current) {
        choices.push(SelectOption {
            value: current.to_string(),
            label: current.to_string(),
        });
    }
    choices
}

/// Agentes que aún no tienen fila y se pueden añadir (`orchestrator` incluido), sin repetidos.
fn addable_agents(roles: &[String], rows: &[RoutingRow]) -> Vec<String> {
    let mut candidates: Vec<String> = vec![ORCHESTRATOR_KEY.to_string()];
    candidates.extend(roles.iter().cloned());
    let mut result: Vec<String> = Vec::new();
    for agent in candidates {
        let used: bool = rows.iter().any(|row: &RoutingRow| row.agent == agent);
        if !used && !result.contains(&agent) {
            result.push(agent);
        }
    }
    result
}

/// Editor genérico de enrutado: una fila por agente con modelo (desplegable del catálogo)
/// y nivel de razonamiento. Mantiene un borrador local y lo entrega a `on_save`.
#[component]
pub fn RoutingEditor(
    /// Filas iniciales; el editor se reconstruye para reiniciar el borrador.
    initial_rows: Vec<RoutingRow>,
    /// Catálogo de modelos, niveles y roles: son señales para que recargarlos no reconstruya
    /// el editor ni descarte el borrador sin guardar.
    #[prop(into)]
    model_options: Signal<Vec<SelectOption>>,
    #[prop(into)] thinking_levels: Signal<Vec<String>>,
    /// Roles que se pueden añadir como fila nueva.
    #[prop(into)]
    roles: Signal<Vec<String>>,
    on_save: Callback<Vec<RoutingRow>>,
    #[prop(into)] saving: Signal<bool>,
) -> impl IntoView {
    let rows: RwSignal<Vec<RoutingRow>> = RwSignal::new(initial_rows);
    let to_add: RwSignal<String> = RwSignal::new(String::new());
    let addable: Memo<Vec<String>> = Memo::new(move |_| {
        rows.with(|current: &Vec<RoutingRow>| {
            roles.with(|known: &Vec<String>| addable_agents(known, current))
        })
    });

    let update_row = move |agent: &str, apply: &dyn Fn(&mut RoutingRow)| {
        rows.update(|current: &mut Vec<RoutingRow>| {
            if let Some(row) = current
                .iter_mut()
                .find(|r: &&mut RoutingRow| r.agent == agent)
            {
                apply(row);
            }
        });
    };
    let add_row = move |_| {
        let agent: String = to_add.get_untracked();
        if agent.is_empty() {
            return;
        }
        rows.update(|current: &mut Vec<RoutingRow>| {
            current.push(RoutingRow {
                agent,
                ..RoutingRow::default()
            })
        });
        to_add.set(String::new());
    };

    view! {
        <div class="stack gap-3">
            <ul class="stack gap-2">
                <For
                    each=move || rows.get()
                    key=|row: &RoutingRow| row.agent.clone()
                    children=move |row: RoutingRow| {
                        let agent: String = row.agent.clone();
                        let model_agent: String = agent.clone();
                        let thinking_agent: String = agent.clone();
                        let remove_agent: String = agent.clone();
                        let model_agent_live: String = agent.clone();
                        let current_model = move || -> String {
                            rows.with(|current: &Vec<RoutingRow>| {
                                current
                                    .iter()
                                    .find(|r: &&RoutingRow| r.agent == model_agent_live)
                                    .map(|r: &RoutingRow| r.model.clone())
                                    .unwrap_or_default()
                            })
                        };
                        let blank_selected = current_model.clone();
                        let thinking_current: String = row.thinking.clone();
                        let choices_current: String = thinking_current.clone();
                        view! {
                            <li class="card stack gap-2">
                                <div class="row justify-between">
                                    <span class="font-medium text-fg">{agent.clone()}</span>
                                    <button
                                        type="button"
                                        class="btn-danger min-h-9 rounded-lg px-2.5 py-1 text-xs"
                                        on:click=move |_| {
                                            rows.update(|current: &mut Vec<RoutingRow>| {
                                                current
                                                    .retain(|r: &RoutingRow| r.agent != remove_agent)
                                            })
                                        }
                                    >
                                        "Quitar"
                                    </button>
                                </div>
                                <label class="stack gap-1.5">
                                    <span class="text-sm font-medium text-fg">"Modelo"</span>
                                    <select
                                        class="input"
                                        on:change=move |ev| {
                                            let value: String = event_target_value(&ev);
                                            update_row(
                                                &model_agent,
                                                &|r: &mut RoutingRow| r.model = value.clone(),
                                            );
                                        }
                                    >
                                        <option value="" selected=move || blank_selected().is_empty()>
                                            {INHERIT_LABEL}
                                        </option>
                                        {move || {
                                            let live: String = current_model();
                                            model_choices(&model_options.get(), &live)
                                                .into_iter()
                                                .map(|option: SelectOption| {
                                                    let selected: bool = option.value == live;
                                                    view! {
                                                        <option value=option.value selected=selected>
                                                            {option.label}
                                                        </option>
                                                    }
                                                })
                                                .collect_view()
                                        }}
                                    </select>
                                </label>
                                <label class="stack gap-1.5">
                                    <span class="text-sm font-medium text-fg">"Razonamiento"</span>
                                    <select
                                        class="input"
                                        on:change=move |ev| {
                                            let value: String = event_target_value(&ev);
                                            update_row(
                                                &thinking_agent,
                                                &|r: &mut RoutingRow| r.thinking = value.clone(),
                                            );
                                        }
                                    >
                                        <option value="" selected=thinking_current.is_empty()>
                                            {INHERIT_LABEL}
                                        </option>
                                        {move || {
                                            thinking_choices(&thinking_levels.get(), &choices_current)
                                                .into_iter()
                                                .map(|level: String| {
                                                    let selected: bool = level == choices_current;
                                                    let value: String = level.clone();
                                                    view! {
                                                        <option value=value selected=selected>
                                                            {level}
                                                        </option>
                                                    }
                                                })
                                                .collect_view()
                                        }}
                                    </select>
                                </label>
                            </li>
                        }
                    }
                />
            </ul>
            <div class="row flex-wrap gap-2">
                <select
                    class="input w-auto"
                    aria-label="Agente a añadir"
                    prop:value=move || to_add.get()
                    on:change=move |ev| to_add.set(event_target_value(&ev))
                >
                    <option value="">"Añadir agente…"</option>
                    <For
                        each=move || addable.get()
                        key=|agent: &String| agent.clone()
                        children=move |agent: String| {
                            let value: String = agent.clone();
                            view! { <option value=value>{agent}</option> }
                        }
                    />
                </select>
                <button
                    type="button"
                    class="btn-secondary"
                    disabled=move || to_add.get().is_empty()
                    on:click=add_row
                >
                    "Añadir fila"
                </button>
                <button
                    type="button"
                    class="btn-primary ml-auto"
                    disabled=move || saving.get()
                    on:click=move |_| on_save.run(rows.get_untracked())
                >
                    {move || if saving.get() { "Guardando…" } else { "Guardar enrutado" }}
                </button>
            </div>
        </div>
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn entry(model: Option<&str>, thinking: Option<&str>) -> RoutingEntryDto {
        RoutingEntryDto {
            model: model.map(str::to_string),
            thinking: thinking.map(str::to_string),
        }
    }

    #[test]
    fn rows_put_orchestrator_first_then_alphabetical() {
        let routing: BTreeMap<String, RoutingEntryDto> = BTreeMap::from([
            ("sdd-apply".to_string(), entry(Some("a/b"), None)),
            ("alpha".to_string(), entry(None, Some("high"))),
            (
                ORCHESTRATOR_KEY.to_string(),
                entry(Some("c/d"), Some("low")),
            ),
        ]);

        let rows: Vec<RoutingRow> = rows_from_routing(&routing);

        let agents: Vec<&str> = rows.iter().map(|r: &RoutingRow| r.agent.as_str()).collect();
        assert_eq!(agents, vec!["orchestrator", "alpha", "sdd-apply"]);
        assert_eq!(rows[0].model, "c/d");
        assert_eq!(rows[0].thinking, "low");
        assert_eq!(rows[1].model, "");
        assert_eq!(rows[1].thinking, "high");
    }

    #[test]
    fn routing_maps_empty_strings_to_none() {
        let rows: Vec<RoutingRow> = vec![RoutingRow {
            agent: "sdd-apply".to_string(),
            model: "  ".to_string(),
            thinking: String::new(),
        }];

        let routing: BTreeMap<String, RoutingEntryDto> = routing_from_rows(&rows);

        assert_eq!(routing.get("sdd-apply"), Some(&entry(None, None)));
    }

    #[test]
    fn routing_trims_values_and_skips_blank_agents() {
        let rows: Vec<RoutingRow> = vec![
            RoutingRow {
                agent: " ".to_string(),
                model: "x/y".to_string(),
                thinking: String::new(),
            },
            RoutingRow {
                agent: "alpha".to_string(),
                model: " a/b ".to_string(),
                thinking: "high".to_string(),
            },
        ];

        let routing: BTreeMap<String, RoutingEntryDto> = routing_from_rows(&rows);

        assert_eq!(routing.len(), 1);
        assert_eq!(
            routing.get("alpha"),
            Some(&entry(Some("a/b"), Some("high")))
        );
    }

    #[test]
    fn last_duplicate_row_wins() {
        let rows: Vec<RoutingRow> = vec![
            RoutingRow {
                agent: "alpha".to_string(),
                model: "first".to_string(),
                thinking: String::new(),
            },
            RoutingRow {
                agent: "alpha".to_string(),
                model: "second".to_string(),
                thinking: String::new(),
            },
        ];

        let routing: BTreeMap<String, RoutingEntryDto> = routing_from_rows(&rows);

        assert_eq!(routing.get("alpha"), Some(&entry(Some("second"), None)));
    }

    #[test]
    fn rows_roundtrip_through_routing() {
        let routing: BTreeMap<String, RoutingEntryDto> = BTreeMap::from([
            (
                ORCHESTRATOR_KEY.to_string(),
                entry(Some("c/d"), Some("low")),
            ),
            ("alpha".to_string(), entry(None, None)),
        ]);

        assert_eq!(routing_from_rows(&rows_from_routing(&routing)), routing);
    }

    #[test]
    fn thinking_choices_keep_unknown_current_value() {
        let levels: Vec<String> = vec!["low".to_string(), "high".to_string()];

        assert_eq!(thinking_choices(&levels, "high"), levels);
        assert_eq!(thinking_choices(&levels, ""), levels);
        assert_eq!(
            thinking_choices(&levels, "ultra"),
            vec!["low".to_string(), "high".to_string(), "ultra".to_string()]
        );
    }

    #[test]
    fn model_choices_keep_unknown_current_value() {
        let known: Vec<SelectOption> = vec![SelectOption {
            value: "p/m".to_string(),
            label: "p/m".to_string(),
        }];

        assert_eq!(model_choices(&known, "p/m"), known);
        assert_eq!(model_choices(&known, ""), known);
        let extended: Vec<SelectOption> = model_choices(&known, "x/y");
        assert_eq!(extended.len(), 2);
        assert_eq!(extended[1].value, "x/y");
    }

    #[test]
    fn addable_agents_excludes_existing_rows_and_includes_orchestrator() {
        let rows: Vec<RoutingRow> = vec![RoutingRow {
            agent: "alpha".to_string(),
            ..RoutingRow::default()
        }];
        let roles: Vec<String> = vec!["alpha".to_string(), "beta".to_string(), "beta".to_string()];

        assert_eq!(
            addable_agents(&roles, &rows),
            vec!["orchestrator".to_string(), "beta".to_string()]
        );
    }

    #[test]
    fn model_select_options_map_id_and_label() {
        let options: Vec<ModelOptionDto> = vec![ModelOptionDto {
            id: "a/b".to_string(),
            label: "A B".to_string(),
        }];
        let mapped: Vec<SelectOption> = model_select_options(&options);
        assert_eq!(mapped.len(), 1);
        assert_eq!(mapped[0].value, "a/b");
        assert_eq!(mapped[0].label, "A B");
    }

    #[test]
    fn load_error_reports_only_failures() {
        let ok: Option<Result<u8, ServerFnError>> = Some(Ok(1));
        let pending: Option<Result<u8, ServerFnError>> = None;
        let failed: Option<Result<u8, ServerFnError>> = Some(Err(ServerFnError::new("boom")));
        assert_eq!(load_error(&ok), None);
        assert_eq!(load_error(&pending), None);
        assert!(load_error(&failed).is_some_and(|m: String| m.contains("boom")));
    }
}
