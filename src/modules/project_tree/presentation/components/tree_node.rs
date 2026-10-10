use leptos::prelude::*;

use crate::core::components::layout::ErrorText;
use crate::modules::project_tree::infrastructure::adapters::inbound::dto::project_tree_dto::TreeEntryDto;
use crate::modules::project_tree::presentation::state::project_tree_state::{
    ChildrenState, ProjectTreeState,
};

/// Horizontal indentation per depth level, in rem.
const INDENT_REM: f32 = 1.0;
/// Base left padding of every row, in rem.
const BASE_PADDING_REM: f32 = 0.25;

/// Left padding style for a row at `depth` (pure).
pub fn indent_style(depth: usize) -> String {
    format!(
        "padding-left: {}rem",
        BASE_PADDING_REM + INDENT_REM * depth as f32
    )
}

/// Glyph shown before the entry name (pure).
pub fn glyph(is_dir: bool, expanded: bool) -> &'static str {
    match (is_dir, expanded) {
        (true, true) => "▾",
        (true, false) => "▸",
        (false, _) => "·",
    }
}

/// Recursive tree row: directories toggle and lazily render their children.
#[component]
pub fn TreeNode(entry: TreeEntryDto, depth: usize, state: ProjectTreeState) -> impl IntoView {
    let TreeEntryDto { name, path, is_dir } = entry;
    let style: String = indent_style(depth);
    let title: String = name.clone();

    if !is_dir {
        return view! {
            <li>
                <div class="row min-w-0 gap-1.5 py-0.5 text-muted" style=style>
                    <span aria-hidden="true" class="w-3 shrink-0 text-center">
                        {glyph(false, false)}
                    </span>
                    <span class="truncate" title=title>
                        {name}
                    </span>
                </div>
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
    let children_path: String = path;

    view! {
        <li>
            <button
                type="button"
                class="row w-full min-w-0 gap-1.5 py-0.5 text-left text-fg hover:bg-surface-2"
                style=style
                aria-expanded=move || expanded.get().to_string()
                on:click=move |_| state.toggle(&toggle_path)
            >
                <span aria-hidden="true" class="w-3 shrink-0 text-center text-muted">
                    {move || glyph(true, expanded.get())}
                </span>
                <span class="truncate font-medium" title=title>
                    {name}
                </span>
            </button>
            <Show when=move || state.is_expanded(&expanded_path)>
                {
                    let children_path: String = children_path.clone();
                    move || match state.children(&children_path) {
                        None | Some(ChildrenState::Loading) => {
                            view! {
                                <p class="muted py-0.5" style=indent_style(depth + 1)>
                                    "Cargando…"
                                </p>
                            }
                                .into_any()
                        }
                        Some(ChildrenState::Failed(message)) => {
                            view! {
                                <div style=indent_style(depth + 1)>
                                    <ErrorText message />
                                </div>
                            }
                                .into_any()
                        }
                        Some(ChildrenState::Loaded(entries)) if entries.is_empty() => {
                            view! {
                                <p class="muted py-0.5" style=indent_style(depth + 1)>
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
        assert_eq!(indent_style(0), "padding-left: 0.25rem");
        assert_eq!(indent_style(2), "padding-left: 2.25rem");
    }

    #[test]
    fn glyph_reflects_kind_and_expansion() {
        assert_eq!(glyph(true, false), "▸");
        assert_eq!(glyph(true, true), "▾");
        assert_eq!(glyph(false, true), "·");
    }
}
