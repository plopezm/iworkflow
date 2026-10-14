use leptos::prelude::*;

use crate::core::components::tree::{TreeChevron, TreeIcon};
use crate::modules::tasks::presentation::components::diff_badges::{DiffCounts, DiffStatusBadge};
use crate::modules::tasks::presentation::state::diff_files::{FileDiff, TreeNode, TreeNodeKind};
use crate::modules::tasks::presentation::state::diff_view_state::{DiffViewState, anchor_id};

/// Horizontal indentation per depth level, in rem.
const INDENT_REM: f32 = 0.875;
/// Base left padding of every row, in rem.
const BASE_PADDING_REM: f32 = 0.75;

/// Left padding style for a row at `depth` (pure).
pub fn indent_style(depth: usize) -> String {
    format!(
        "padding-left: {}rem",
        BASE_PADDING_REM + INDENT_REM * depth as f32
    )
}

/// Tree of the changed files; clicking a file selects it and jumps to its card.
#[component]
pub fn DiffFileTree(state: DiffViewState) -> impl IntoView {
    view! {
        <nav class="diff-tree" aria-label="Ficheros modificados">
            <ul>
                {move || {
                    state
                        .tree
                        .get()
                        .into_iter()
                        .map(|node: TreeNode| view! { <DiffTreeNode node depth=0 state /> })
                        .collect_view()
                }}
            </ul>
        </nav>
    }
}

/// Recursive tree row: folders collapse/expand, files link to their card.
#[component]
pub fn DiffTreeNode(node: TreeNode, depth: usize, state: DiffViewState) -> impl IntoView {
    let TreeNode {
        name,
        path,
        kind,
        additions,
        deletions,
    } = node;
    let style: String = indent_style(depth);
    match kind {
        TreeNodeKind::File(index) => {
            let file: Option<FileDiff> = state
                .files
                .with(|files: &Vec<FileDiff>| files.get(index).cloned());
            let Some(file) = file else {
                return ().into_any();
            };
            let href: String = format!("#{}", anchor_id(&path));
            let select_path: String = path.clone();
            let selected_path: String = path;
            view! {
                <li>
                    <a
                        href=href
                        class="diff-tree-row"
                        class:diff-tree-row-selected=move || {
                            state
                                .selected_file
                                .with(|s: &Option<String>| s.as_deref() == Some(selected_path.as_str()))
                        }
                        style=style
                        title=file.path.clone()
                        on:click=move |_| state.select(&select_path)
                    >
                        <span class="tree-chevron-spacer" aria-hidden="true"></span>
                        <TreeIcon />
                        <span class="min-w-0 flex-1 truncate">{name}</span>
                        <DiffStatusBadge status=file.status />
                        <DiffCounts additions deletions />
                    </a>
                </li>
            }
            .into_any()
        }
        TreeNodeKind::Dir(children) => {
            let toggle_path: String = path.clone();
            let collapsed_path: String = path.clone();
            let show_path: String = path;
            let collapsed: Signal<bool> =
                Signal::derive(move || state.is_dir_collapsed(&collapsed_path));
            view! {
                <li>
                    <button
                        type="button"
                        class="diff-tree-row"
                        style=style
                        aria-expanded=move || (!collapsed.get()).to_string()
                        on:click=move |_| state.toggle_dir(&toggle_path)
                    >
                        <TreeChevron open=Signal::derive(move || !collapsed.get()) />
                        <TreeIcon is_dir=true open=Signal::derive(move || !collapsed.get()) />
                        <span class="min-w-0 flex-1 truncate" title=show_path>
                            {name}
                        </span>
                        <DiffCounts additions deletions />
                    </button>
                    <Show when=move || !collapsed.get()>
                        <ul>
                            {children
                                .clone()
                                .into_iter()
                                .map(|child: TreeNode| {
                                    view! { <DiffTreeNode node=child depth=depth + 1 state /> }
                                })
                                .collect_view()}
                        </ul>
                    </Show>
                </li>
            }
            .into_any()
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn indent_grows_with_depth() {
        assert_eq!(indent_style(0), "padding-left: 0.75rem");
        assert_eq!(indent_style(2), "padding-left: 2.5rem");
    }
}
