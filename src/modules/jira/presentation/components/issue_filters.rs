use leptos::prelude::*;

use crate::core::components::form::{SelectField, SelectOption};
use crate::modules::jira::infrastructure::adapters::inbound::dto::jira_dto::IssueLocalStateDto;
use crate::modules::jira::presentation::state::jira_issues_state::JiraIssuesState;

/// Search (applied on Enter/blur), Jira status and local state filters.
#[component]
pub fn IssueFilters(state: JiraIssuesState) -> impl IntoView {
    let local_state_options: Vec<SelectOption> = IssueLocalStateDto::ALL
        .into_iter()
        .map(|s: IssueLocalStateDto| SelectOption {
            value: s.as_str().to_string(),
            label: s.label().to_string(),
        })
        .collect();
    view! {
        <div class="grid gap-3 md:grid-cols-3">
            <label class="stack gap-1.5">
                <span class="text-sm font-medium text-fg">"Buscar"</span>
                <input
                    type="search"
                    class="input"
                    placeholder="Clave, resumen, responsable o etiqueta"
                    prop:value=move || state.search.get()
                    on:change=move |ev| state.search.set(event_target_value(&ev))
                />
            </label>
            {move || {
                let options: Vec<SelectOption> = state
                    .statuses
                    .get()
                    .into_iter()
                    .map(|s: String| SelectOption {
                        value: s.clone(),
                        label: s,
                    })
                    .collect();
                view! {
                    <SelectField
                        label="Estado en Jira"
                        value=state.status
                        options=options
                        placeholder="Todos"
                    />
                }
            }}
            <SelectField
                label="Estado local"
                value=state.local_state
                options=local_state_options
                placeholder="Todos"
            />
        </div>
    }
}
