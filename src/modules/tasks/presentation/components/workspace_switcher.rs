use leptos::prelude::*;

use crate::core::i18n::Msg;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::WorkspaceOptionDto;

const SWITCH_WORKSPACE: Msg = Msg {
    en: "Switch workspace",
    es: "Cambiar de workspace",
    zh: "切换工作区",
    de: "Workspace wechseln",
    fr: "Changer d'espace de travail",
};

/// Indica si `workspace_id` es el workspace seleccionado.
pub fn is_selected_workspace(selected: Option<&str>, workspace_id: &str) -> bool {
    selected == Some(workspace_id)
}

/// Selector de workspace: un desplegable nativo (en móvil abre el selector del sistema).
#[component]
pub fn WorkspaceSwitcher(
    #[prop(into)] workspaces: Signal<Vec<WorkspaceOptionDto>>,
    #[prop(into)] selected: Signal<Option<String>>,
    on_select: Callback<String>,
) -> impl IntoView {
    view! {
        <label class="select-pill max-w-xs" title=SWITCH_WORKSPACE.get()>
            <span class="sr-only">{SWITCH_WORKSPACE.get()}</span>
            <select
                class="select-pill-input"
                on:change=move |ev| on_select.run(event_target_value(&ev))
            >
                {move || {
                    let current: Option<String> = selected.get();
                    workspaces
                        .get()
                        .into_iter()
                        .map(|workspace: WorkspaceOptionDto| {
                            let is_selected: bool = is_selected_workspace(
                                current.as_deref(),
                                &workspace.id,
                            );
                            view! {
                                <option
                                    value=workspace.id.clone()
                                    selected=is_selected
                                    title=workspace.base_branch.clone()
                                >
                                    {workspace.name}
                                </option>
                            }
                        })
                        .collect_view()
                }}
            </select>
            <svg class="select-pill-chevron" viewBox="0 0 20 20" fill="none" aria-hidden="true">
                <path
                    d="M5.5 8L10 12.5L14.5 8"
                    stroke="currentColor"
                    stroke-width="1.5"
                    stroke-linecap="round"
                    stroke-linejoin="round"
                />
            </svg>
        </label>
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn only_the_matching_workspace_is_selected() {
        assert!(is_selected_workspace(Some("w1"), "w1"));
        assert!(!is_selected_workspace(Some("w1"), "w2"));
        assert!(!is_selected_workspace(None, "w1"));
    }
}
