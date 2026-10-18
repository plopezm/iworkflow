use leptos::prelude::*;

use crate::core::components::form::{SelectField, SelectOption};
use crate::core::i18n::Msg;
use crate::modules::jira::infrastructure::adapters::inbound::dto::jira_dto::IssueLocalStateDto;
use crate::modules::jira::presentation::state::formatting::local_state_label;
use crate::modules::jira::presentation::state::jira_issues_state::JiraIssuesState;

const SEARCH: Msg = Msg {
    en: "Search",
    es: "Buscar",
    zh: "搜索",
    de: "Suchen",
    fr: "Rechercher",
};

const SEARCH_PLACEHOLDER: Msg = Msg {
    en: "Key, summary, assignee or label",
    es: "Clave, resumen, responsable o etiqueta",
    zh: "键、摘要、负责人或标签",
    de: "Schlüssel, Zusammenfassung, Bearbeiter oder Label",
    fr: "Clé, résumé, responsable ou étiquette",
};

const JIRA_STATUS: Msg = Msg {
    en: "Status in Jira",
    es: "Estado en Jira",
    zh: "Jira 中的状态",
    de: "Status in Jira",
    fr: "Statut dans Jira",
};

const LOCAL_STATUS: Msg = Msg {
    en: "Local status",
    es: "Estado local",
    zh: "本地状态",
    de: "Lokaler Status",
    fr: "Statut local",
};

const ALL: Msg = Msg {
    en: "All",
    es: "Todos",
    zh: "全部",
    de: "Alle",
    fr: "Tous",
};

/// Search (applied on Enter/blur), Jira status and local state filters.
#[component]
pub fn IssueFilters(state: JiraIssuesState) -> impl IntoView {
    let local_state_options: Vec<SelectOption> = IssueLocalStateDto::ALL
        .into_iter()
        .map(|s: IssueLocalStateDto| SelectOption {
            value: s.as_str().to_string(),
            label: local_state_label(s).to_string(),
        })
        .collect();
    view! {
        <div class="grid gap-3 md:grid-cols-3">
            <label class="stack gap-1.5">
                <span class="text-sm font-medium text-fg">{SEARCH.get()}</span>
                <input
                    type="search"
                    class="input"
                    placeholder=SEARCH_PLACEHOLDER.get()
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
                        label=JIRA_STATUS.get()
                        value=state.status
                        options=options
                        placeholder=ALL.get()
                    />
                }
            }}
            <SelectField
                label=LOCAL_STATUS.get()
                value=state.local_state
                options=local_state_options
                placeholder=ALL.get()
            />
        </div>
    }
}
