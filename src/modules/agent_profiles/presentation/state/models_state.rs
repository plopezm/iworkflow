use leptos::prelude::*;

use crate::core::components::toast::ToastMessage;
use crate::core::i18n::Msg;
use crate::modules::agent_profiles::infrastructure::adapters::inbound::dto::agent_profiles_dto::{
    AgentRoleDto, ModelOptionDto, NewAgentRoleDto, RoutingEntryDto,
};
use crate::modules::agent_profiles::infrastructure::adapters::inbound::server_fns::{
    create_agent_role, get_global_routing, list_agent_roles, list_model_options,
    list_profile_thinking_levels, set_global_routing,
};
use crate::modules::agent_profiles::presentation::components::routing_editor::{
    RoutingRow, routing_from_rows,
};
use crate::modules::agent_profiles::presentation::state::profiles_state::Notifier;
use std::collections::BTreeMap;

type Mutation<I, O> = Action<I, Result<O, ServerFnError>>;

const SAVED_NONE: Msg = Msg {
    en: "Global routing saved (no agents)",
    es: "Routing global guardado (sin agentes)",
    zh: "全局路由已保存（无智能体）",
    de: "Globales Routing gespeichert (keine Agents)",
    fr: "Routage global enregistré (aucun agent)",
};

const SAVED_ONE: Msg = Msg {
    en: "Global routing saved (1 agent)",
    es: "Routing global guardado (1 agente)",
    zh: "全局路由已保存（1 个智能体）",
    de: "Globales Routing gespeichert (1 Agent)",
    fr: "Routage global enregistré (1 agent)",
};

const SAVED_MANY: Msg = Msg {
    en: "Global routing saved ({n} agents)",
    es: "Routing global guardado ({n} agentes)",
    zh: "全局路由已保存（{n} 个智能体）",
    de: "Globales Routing gespeichert ({n} Agents)",
    fr: "Routage global enregistré ({n} agents)",
};

const ROLE_CREATED: Msg = Msg {
    en: "Role created",
    es: "Rol creado",
    zh: "角色已创建",
    de: "Rolle erstellt",
    fr: "Rôle créé",
};

/// Mensaje de éxito al guardar el routing global, con el número de agentes guardados.
pub fn saved_message(agents: usize) -> String {
    match agents {
        0 => SAVED_NONE.get().to_string(),
        1 => SAVED_ONE.get().to_string(),
        n => SAVED_MANY.fmt(&[("n", &n.to_string())]),
    }
}

/// Estado de la pantalla de modelos: routing global, opciones y mutaciones con toast.
/// Las lecturas se refrescan mediante el contador de versión de cada acción.
#[derive(Clone, Copy)]
pub struct ModelsState {
    pub routing: Resource<Result<BTreeMap<String, RoutingEntryDto>, ServerFnError>>,
    pub roles: Resource<Result<Vec<AgentRoleDto>, ServerFnError>>,
    pub model_options: Resource<Result<Vec<ModelOptionDto>, ServerFnError>>,
    pub thinking_levels: Resource<Result<Vec<String>, ServerFnError>>,
    pub role_dialog_open: RwSignal<bool>,
    pub role_name: RwSignal<String>,
    pub role_description: RwSignal<String>,
    pub role_prompt: RwSignal<String>,
    pub toasts: RwSignal<Vec<ToastMessage>>,
    save: Mutation<Vec<RoutingRow>, ()>,
    create_role: Mutation<NewAgentRoleDto, ()>,
}

impl ModelsState {
    pub fn new() -> Self {
        let role_dialog_open: RwSignal<bool> = RwSignal::new(false);
        let role_name: RwSignal<String> = RwSignal::new(String::new());
        let role_description: RwSignal<String> = RwSignal::new(String::new());
        let role_prompt: RwSignal<String> = RwSignal::new(String::new());
        let notifier: Notifier = Notifier {
            toasts: RwSignal::new(Vec::new()),
            seq: RwSignal::new(0),
        };

        let save: Mutation<Vec<RoutingRow>, ()> = Action::new(move |rows: &Vec<RoutingRow>| {
            let routing: BTreeMap<String, RoutingEntryDto> = routing_from_rows(rows);
            let count: usize = routing.len();
            async move {
                let result: Result<(), ServerFnError> = set_global_routing(routing).await;
                notifier.report(&result, &saved_message(count));
                result
            }
        });
        let create_role: Mutation<NewAgentRoleDto, ()> =
            Action::new(move |input: &NewAgentRoleDto| {
                let input: NewAgentRoleDto = input.clone();
                async move {
                    let result: Result<(), ServerFnError> = create_agent_role(input).await;
                    if result.is_ok() {
                        role_dialog_open.set(false);
                        role_name.set(String::new());
                        role_description.set(String::new());
                        role_prompt.set(String::new());
                    }
                    notifier.report(&result, ROLE_CREATED.get());
                    result
                }
            });

        let routing: Resource<Result<BTreeMap<String, RoutingEntryDto>, ServerFnError>> =
            Resource::new(move || save.version().get(), |_| get_global_routing());
        let roles: Resource<Result<Vec<AgentRoleDto>, ServerFnError>> =
            Resource::new(move || create_role.version().get(), |_| list_agent_roles());
        let model_options: Resource<Result<Vec<ModelOptionDto>, ServerFnError>> =
            Resource::new(|| (), |_| list_model_options());
        let thinking_levels: Resource<Result<Vec<String>, ServerFnError>> =
            Resource::new(|| (), |_| list_profile_thinking_levels());

        Self {
            routing,
            roles,
            model_options,
            thinking_levels,
            role_dialog_open,
            role_name,
            role_description,
            role_prompt,
            toasts: notifier.toasts,
            save,
            create_role,
        }
    }

    /// Hay alguna mutación en curso.
    pub fn busy(&self) -> Signal<bool> {
        let state: ModelsState = *self;
        Signal::derive(move || state.save.pending().get() || state.create_role.pending().get())
    }

    pub fn saving(&self) -> Signal<bool> {
        self.save.pending().into()
    }

    pub fn save(&self, rows: Vec<RoutingRow>) {
        self.save.dispatch(rows);
    }

    pub fn open_role_dialog(&self) {
        self.role_dialog_open.set(true);
    }

    pub fn close_role_dialog(&self) {
        self.role_dialog_open.set(false);
    }

    pub fn create_role(&self) {
        self.create_role.dispatch(NewAgentRoleDto {
            name: self.role_name.get_untracked().trim().to_string(),
            description: self.role_description.get_untracked().trim().to_string(),
            prompt: self.role_prompt.get_untracked(),
            model: None,
            thinking: None,
        });
    }
}

impl Default for ModelsState {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::core::i18n::Locale;

    #[test]
    fn saved_message_singular() {
        assert_eq!(saved_message(1), "Global routing saved (1 agent)");
    }

    #[test]
    fn saved_message_plural() {
        assert_eq!(saved_message(3), "Global routing saved (3 agents)");
    }

    #[test]
    fn saved_message_empty() {
        assert_eq!(saved_message(0), "Global routing saved (no agents)");
    }

    #[test]
    fn saved_message_spanish_keeps_original_text() {
        assert_eq!(
            SAVED_MANY.in_locale(Locale::Es).replace("{n}", "3"),
            "Routing global guardado (3 agentes)"
        );
    }
}
