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

/// Clave de un chip para el `<For>`: todo lo que el chip pinta (nombre y rama base, que va
/// en el `title`), de modo que cambiar cualquiera de los dos lo reconstruye.
pub fn workspace_chip_key(workspace: &WorkspaceOptionDto) -> String {
    format!(
        "{}|{}|{}",
        workspace.id, workspace.name, workspace.base_branch
    )
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
                key=workspace_chip_key
                children=move |workspace: WorkspaceOptionDto| {
                    let chip_id: String = workspace.id.clone();
                    let click_id: String = workspace.id.clone();
                    view! {
                        <button
                            type="button"
                            class=move || {
                                switcher_chip_class(
                                    selected.with(|s: &Option<String>| s.as_deref() == Some(chip_id.as_str())),
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
    fn the_chip_key_changes_with_name_or_base_branch() {
        let workspace: WorkspaceOptionDto = WorkspaceOptionDto {
            id: "w1".to_string(),
            name: "api".to_string(),
            base_branch: "main".to_string(),
        };
        let before: String = workspace_chip_key(&workspace);
        let renamed: WorkspaceOptionDto = WorkspaceOptionDto {
            name: "web".to_string(),
            ..workspace.clone()
        };
        let rebased: WorkspaceOptionDto = WorkspaceOptionDto {
            base_branch: "develop".to_string(),
            ..workspace
        };
        assert_ne!(before, workspace_chip_key(&renamed));
        assert_ne!(before, workspace_chip_key(&rebased));
    }

    #[test]
    fn selected_workspace_uses_the_selected_chip_style() {
        assert!(switcher_chip_class(true).contains("chip-selected"));
        assert!(!switcher_chip_class(false).contains("chip-selected"));
    }
}
