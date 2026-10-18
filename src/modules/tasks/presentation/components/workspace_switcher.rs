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

/// Clase del chip de un workspace según esté o no seleccionado.
pub fn switcher_chip_class(selected: bool) -> &'static str {
    if selected {
        "chip chip-selected shrink-0"
    } else {
        "chip shrink-0"
    }
}

/// Selector rápido de workspace: una fila desplazable de chips, uno por workspace.
#[component]
pub fn WorkspaceSwitcher(
    #[prop(into)] workspaces: Signal<Vec<WorkspaceOptionDto>>,
    #[prop(into)] selected: Signal<Option<String>>,
    on_select: Callback<String>,
) -> impl IntoView {
    view! {
        <nav
            class="flex min-w-0 items-center gap-1.5 overflow-x-auto pb-0.5"
            aria-label=SWITCH_WORKSPACE.get()
        >
            <For
                each=move || workspaces.get()
                key=|w: &WorkspaceOptionDto| format!("{}|{}", w.id, w.name)
                children=move |workspace: WorkspaceOptionDto| {
                    let id: String = workspace.id.clone();
                    let click_id: String = workspace.id.clone();
                    view! {
                        <button
                            type="button"
                            class=move || {
                                switcher_chip_class(
                                    selected.with(|s: &Option<String>| s.as_deref() == Some(id.as_str())),
                                )
                            }
                            title=workspace.base_branch.clone()
                            on:click=move |_| on_select.run(click_id.clone())
                        >
                            {workspace.name}
                        </button>
                    }
                }
            />
        </nav>
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn selected_workspace_uses_the_selected_chip_style() {
        assert!(switcher_chip_class(true).contains("chip-selected"));
        assert!(!switcher_chip_class(false).contains("chip-selected"));
    }
}
