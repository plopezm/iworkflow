use leptos::prelude::*;

use crate::core::components::layout::ErrorText;
use crate::core::components::tree::{TreeChevron, TreeIcon};
use crate::core::dock::link::TabLink;
use crate::modules::project_tree::infrastructure::adapters::inbound::dto::project_tree_dto::TreeEntryDto;
use crate::modules::project_tree::presentation::state::project_file_state::file_href;
use crate::modules::project_tree::presentation::state::project_tree_state::{
    ChildrenState, ProjectTreeState,
};

/// Horizontal indentation per depth level, in rem.
const INDENT_REM: f32 = 1.125;
/// Base left padding of every row, in rem.
const BASE_PADDING_REM: f32 = 0.75;

/// Left padding style for a row at `depth` (pure).
pub fn indent_style(depth: usize) -> String {
    format!(
        "padding-left: {}rem",
        BASE_PADDING_REM + INDENT_REM * depth as f32
    )
}

/// Recursive tree row: directories toggle and lazily render their children.
#[component]
pub fn TreeNode(entry: TreeEntryDto, depth: usize, state: ProjectTreeState) -> impl IntoView {
    let TreeEntryDto { name, path, is_dir } = entry;
    let style: String = indent_style(depth);
    let title: String = name.clone();

    if !is_dir {
        let class_path: String = path.clone();
        return view! {
            <li>
                <TabLink
                    href=file_href(&state.workspace_id.get_value(), &path)
                    class=move || format!("tree-row {}", state.git_class(&class_path))
                    style=style
                    beside=true
                >
                    <span class="tree-chevron-spacer" aria-hidden="true"></span>
                    <TreeIcon />
                    <span class="truncate" title=title>
                        {name}
                    </span>
                </TabLink>
            </li>
        }
        .into_any();
    }

    let toggle_path: String = path.clone();
    let expanded_path: String = path.clone();
    let expanded: Signal<bool> = Signal::derive({
        let path: String = path.clone();
        move || state.is_expanded(&path)
    });
    let children_path: String = path.clone();
    let class_path: String = path;

    view! {
        <li>
            <button
                type="button"
                class=move || format!("tree-row {}", state.git_class(&class_path))
                style=style
                aria-expanded=move || expanded.get().to_string()
                on:click=move |_| state.toggle(&toggle_path)
            >
                <TreeChevron open=expanded />
                <TreeIcon is_dir=true open=expanded />
                <span class="truncate" title=title>
                    {name}
                </span>
            </button>
            <Show when=move || state.is_expanded(&expanded_path)>
                {
                    let children_path: String = children_path.clone();
                    move || match state.children(&children_path) {
                        None | Some(ChildrenState::Loading) => {
                            view! {
                                <p class="py-1 pr-2 text-xs text-muted" style=indent_style(depth + 2)>
                                    "Cargando…"
                                </p>
                            }
                                .into_any()
                        }
                        Some(ChildrenState::Failed(message)) => {
                            view! {
                                <div style=indent_style(depth + 2)>
                                    <ErrorText message />
                                </div>
                            }
                                .into_any()
                        }
                        Some(ChildrenState::Loaded(entries)) if entries.is_empty() => {
                            view! {
                                <p class="py-1 pr-2 text-xs text-muted" style=indent_style(depth + 2)>
                                    "Carpeta vacía."
                                </p>
                            }
                                .into_any()
                        }
                        Some(ChildrenState::Loaded(entries)) => {
                            view! {
                                <ul>
                                    <For
                                        each=move || entries.clone()
                                        key=|e: &TreeEntryDto| e.path.clone()
                                        children=move |e: TreeEntryDto| {
                                            view! { <TreeNode entry=e depth=depth + 1 state /> }
                                        }
                                    />
                                </ul>
                            }
                                .into_any()
                        }
                    }
                }
            </Show>
        </li>
    }
    .into_any()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn indent_grows_with_depth() {
        assert_eq!(indent_style(0), "padding-left: 0.75rem");
        assert_eq!(indent_style(2), "padding-left: 3rem");
    }
}
