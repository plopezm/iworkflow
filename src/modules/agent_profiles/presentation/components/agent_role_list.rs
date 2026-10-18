use leptos::prelude::*;

use crate::core::i18n::Msg;
use crate::modules::agent_profiles::infrastructure::adapters::inbound::dto::agent_profiles_dto::AgentRoleDto;

const NO_ROLES: Msg = Msg {
    en: "No agent roles yet.",
    es: "Aún no hay roles de agente.",
    zh: "还没有智能体角色。",
    de: "Noch keine Agent-Rollen.",
    fr: "Aucun rôle d'agent pour le moment.",
};

/// Lista compacta de roles de agente con su descripción.
#[component]
pub fn AgentRoleList(roles: Vec<AgentRoleDto>) -> impl IntoView {
    if roles.is_empty() {
        return view! { <p class="muted">{NO_ROLES.get()}</p> }.into_any();
    }
    view! {
        <ul class="stack gap-1.5">
            {roles
                .into_iter()
                .map(|role: AgentRoleDto| {
                    view! {
                        <li class="stack gap-0.5">
                            <span class="text-sm font-medium text-fg">{role.name}</span>
                            <span class="muted text-xs">{role.description}</span>
                        </li>
                    }
                })
                .collect_view()}
        </ul>
    }
    .into_any()
}
