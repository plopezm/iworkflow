use std::collections::BTreeMap;

use leptos::prelude::*;

use crate::core::components::form::SelectOption;
use crate::modules::agent_profiles::infrastructure::adapters::inbound::dto::agent_profiles_dto::RoutingEntryDto;

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

/// Editor genérico de enrutado: una fila por agente con modelo (con sugerencias y texto libre)
/// y nivel de razonamiento. Mantiene un borrador local y lo entrega a `on_save`.
#[component]
pub fn RoutingEditor(
    /// Filas iniciales; el editor se reconstruye para reiniciar el borrador.
    initial_rows: Vec<RoutingRow>,
    model_options: Vec<SelectOption>,
    thinking_levels: Vec<String>,
    /// Roles que se pueden añadir como fila nueva.
    roles: Vec<String>,
    on_save: Callback<Vec<RoutingRow>>,
    #[prop(into)] saving: Signal<bool>,
    /// Prefijo único de los ids de `<datalist>` si hay varios editores en la página.
    #[prop(into, default = "routing-editor".to_string())]
    id: String,
) -> impl IntoView {
    let rows: RwSignal<Vec<RoutingRow>> = RwSignal::new(initial_rows);
    let to_add: RwSignal<String> = RwSignal::new(String::new());
    let datalist_id: String = format!("{id}-models");
    let datalist_ref: String = datalist_id.clone();
    let roles: StoredValue<Vec<String>> = StoredValue::new(roles);
    let levels: StoredValue<Vec<String>> = StoredValue::new(thinking_levels);
    let addable: Memo<Vec<String>> = Memo::new(move |_| {
        rows.with(|current: &Vec<RoutingRow>| {
            roles.with_value(|known: &Vec<String>| addable_agents(known, current))
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
            <datalist id=datalist_id>
                {model_options
                    .into_iter()
                    .map(|option: SelectOption| {
                        view! { <option value=option.value>{option.label}</option> }
                    })
                    .collect_view()}
            </datalist>
            <ul class="stack gap-2">
                <For
                    each=move || rows.get()
                    key=|row: &RoutingRow| row.agent.clone()
                    children=move |row: RoutingRow| {
                        let agent: String = row.agent.clone();
                        let model_agent: String = agent.clone();
                        let thinking_agent: String = agent.clone();
                        let remove_agent: String = agent.clone();
                        let thinking_current: String = row.thinking.clone();
                        let choices: Vec<String> = levels
                            .with_value(|known: &Vec<String>| {
                                thinking_choices(known, &thinking_current)
                            });
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
                                    <input
                                        type="text"
                                        class="input"
                                        list=datalist_ref.clone()
                                        placeholder="proveedor/modelo (vacío = por defecto)"
                                        prop:value=row.model.clone()
                                        on:input=move |ev| {
                                            let value: String = event_target_value(&ev);
                                            update_row(
                                                &model_agent,
                                                &|r: &mut RoutingRow| r.model = value.clone(),
                                            );
                                        }
                                    />
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
                                        {choices
                                            .into_iter()
                                            .map(|level: String| {
                                                let selected: bool = level == thinking_current;
                                                let value: String = level.clone();
                                                view! {
                                                    <option value=value selected=selected>
                                                        {level}
                                                    </option>
                                                }
                                            })
                                            .collect_view()}
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
}
