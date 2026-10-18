use leptos::prelude::*;

use crate::core::i18n::Msg;

use crate::modules::tasks::presentation::components::diff_badges::{DiffCounts, DiffStatusBadge};
use crate::modules::tasks::presentation::components::diff_file_editor::DiffFileEditor;
use crate::modules::tasks::presentation::state::diff_files::{
    Cell, CellKind, FileDiff, Hunk, SplitRow, UnifiedLine,
};
use crate::modules::tasks::presentation::state::diff_view_state::{
    DiffViewState, ViewMode, anchor_id,
};

const EDIT: Msg = Msg {
    en: "Edit",
    es: "Editar",
    zh: "编辑",
    de: "Bearbeiten",
    fr: "Modifier",
};

const BINARY_FILE: Msg = Msg {
    en: "Binary file: content is not shown.",
    es: "Fichero binario: no se muestra el contenido.",
    zh: "二进制文件：不显示内容。",
    de: "Binärdatei: Der Inhalt wird nicht angezeigt.",
    fr: "Fichier binaire : le contenu n’est pas affiché.",
};

const NO_CONTENT_CHANGES: Msg = Msg {
    en: "No content changes.",
    es: "Sin cambios de contenido.",
    zh: "内容没有变化。",
    de: "Keine Inhaltsänderungen.",
    fr: "Aucune modification de contenu.",
};

const BEFORE: Msg = Msg {
    en: "Before",
    es: "Antes",
    zh: "之前",
    de: "Vorher",
    fr: "Avant",
};

const AFTER: Msg = Msg {
    en: "After",
    es: "Después",
    zh: "之后",
    de: "Nachher",
    fr: "Après",
};

/// CSS class of a split-view cell; a missing cell is the blank side of an add/remove.
pub fn cell_class(cell: Option<&Cell>) -> &'static str {
    match cell.map(|c: &Cell| c.kind) {
        None => "diff-cell diff-cell-empty",
        Some(kind) => kind_class(kind),
    }
}

/// CSS class of a cell or unified line of the given kind.
pub fn kind_class(kind: CellKind) -> &'static str {
    match kind {
        CellKind::Context => "diff-cell",
        CellKind::Added => "diff-cell diff-cell-add",
        CellKind::Removed => "diff-cell diff-cell-del",
    }
}

/// Sign shown before a unified line.
pub fn kind_sign(kind: CellKind) -> &'static str {
    match kind {
        CellKind::Context => " ",
        CellKind::Added => "+",
        CellKind::Removed => "-",
    }
}

/// Title of the card: `old → new` for renames.
pub fn file_title(file: &FileDiff) -> String {
    match &file.old_path {
        Some(old) => format!("{old} → {}", file.path),
        None => file.path.clone(),
    }
}

fn number_text(number: Option<usize>) -> String {
    number.map(|n: usize| n.to_string()).unwrap_or_default()
}

/// One changed file: header (path, status, counters, collapse, edit) and its diff or editor.
#[component]
pub fn DiffFileCard(file: FileDiff, state: DiffViewState, editable: bool) -> impl IntoView {
    let path: String = file.path.clone();
    let id: String = anchor_id(&path);
    let title: String = file_title(&file);
    let toggle_path: String = path.clone();
    let collapsed_path: String = path.clone();
    let editing_path: String = path.clone();
    let editor_path: String = path.clone();
    let edit_path: String = path.clone();
    let selected_path: String = path;
    let collapsed: Signal<bool> = Signal::derive(move || state.is_file_collapsed(&collapsed_path));
    let editing: Signal<bool> = Signal::derive(move || state.is_editing(&editing_path));
    let FileDiff {
        status,
        additions,
        deletions,
        binary,
        hunks,
        ..
    } = file;
    let hunks: StoredValue<Vec<Hunk>> = StoredValue::new(hunks);
    let has_hunks: bool = hunks.with_value(|h: &Vec<Hunk>| !h.is_empty());
    view! {
        <section
            id=id
            class="diff-card"
            class:diff-card-selected=move || {
                state
                    .selected_file
                    .with(|s: &Option<String>| s.as_deref() == Some(selected_path.as_str()))
            }
        >
            <header class="diff-card-header">
                <button
                    type="button"
                    class="diff-card-toggle"
                    aria-expanded=move || (!collapsed.get()).to_string()
                    on:click=move |_| state.toggle_file(&toggle_path)
                >
                    <span aria-hidden="true" class="w-3 shrink-0 text-center text-muted">
                        {move || if collapsed.get() { "▸" } else { "▾" }}
                    </span>
                    <DiffStatusBadge status />
                    <span class="min-w-0 flex-1 truncate font-mono text-xs" title=title.clone()>
                        {title.clone()}
                    </span>
                    <DiffCounts additions deletions />
                </button>
                <Show when=move || editable && !editing.get()>
                    <button
                        type="button"
                        class="btn-secondary min-h-7 px-2 py-0.5"
                        on:click={
                            let edit_path: String = edit_path.clone();
                            move |_| state.start_edit(&edit_path)
                        }
                    >
                        {EDIT.get()}
                    </button>
                </Show>
            </header>
            <Show when=move || !collapsed.get()>
                {
                    let editor_path: String = editor_path.clone();
                    move || {
                        if editing.get() {
                            view! { <DiffFileEditor path=editor_path.clone() state /> }.into_any()
                        } else if binary {
                            view! { <p class="muted p-3">{BINARY_FILE.get()}</p> }
                                .into_any()
                        } else if !has_hunks {
                            view! { <p class="muted p-3">{NO_CONTENT_CHANGES.get()}</p> }.into_any()
                        } else {
                            match state.view_mode.get() {
                                ViewMode::Split => {
                                    view! { <SplitBody hunks=hunks.get_value() /> }.into_any()
                                }
                                ViewMode::Unified => {
                                    view! { <UnifiedBody hunks=hunks.get_value() /> }.into_any()
                                }
                            }
                        }
                    }
                }
            </Show>
        </section>
    }
}

/// Two columns, Antes | Después, with line numbers and add/remove highlighting.
#[component]
fn SplitBody(hunks: Vec<Hunk>) -> impl IntoView {
    view! {
        <div class="diff-scroll">
            <table class="diff-table">
                <thead>
                    <tr>
                        <th class="diff-num"></th>
                        <th class="diff-col-head">{BEFORE.get()}</th>
                        <th class="diff-num"></th>
                        <th class="diff-col-head">{AFTER.get()}</th>
                    </tr>
                </thead>
                <tbody>
                    {hunks
                        .into_iter()
                        .map(|hunk: Hunk| {
                            view! {
                                <tr class="diff-hunk-row">
                                    <td colspan="4">{hunk.header}</td>
                                </tr>
                                {hunk
                                    .rows
                                    .into_iter()
                                    .map(|row: SplitRow| {
                                        let left_class: &str = cell_class(row.left.as_ref());
                                        let right_class: &str = cell_class(row.right.as_ref());
                                        let left_number: String = number_text(
                                            row.left.as_ref().map(|c: &Cell| c.number),
                                        );
                                        let right_number: String = number_text(
                                            row.right.as_ref().map(|c: &Cell| c.number),
                                        );
                                        let left_text: String = row
                                            .left
                                            .map(|c: Cell| c.text)
                                            .unwrap_or_default();
                                        let right_text: String = row
                                            .right
                                            .map(|c: Cell| c.text)
                                            .unwrap_or_default();
                                        view! {
                                            <tr>
                                                <td class="diff-num">{left_number}</td>
                                                <td class=left_class>{left_text}</td>
                                                <td class="diff-num">{right_number}</td>
                                                <td class=right_class>{right_text}</td>
                                            </tr>
                                        }
                                    })
                                    .collect_view()}
                            }
                        })
                        .collect_view()}
                </tbody>
            </table>
        </div>
    }
}

/// Single column with old and new line numbers and a +/- sign.
#[component]
fn UnifiedBody(hunks: Vec<Hunk>) -> impl IntoView {
    view! {
        <div class="diff-scroll">
            <table class="diff-table">
                <tbody>
                    {hunks
                        .into_iter()
                        .map(|hunk: Hunk| {
                            view! {
                                <tr class="diff-hunk-row">
                                    <td colspan="4">{hunk.header}</td>
                                </tr>
                                {hunk
                                    .lines
                                    .into_iter()
                                    .map(|line: UnifiedLine| {
                                        let class: &str = kind_class(line.kind);
                                        let sign: &str = kind_sign(line.kind);
                                        view! {
                                            <tr>
                                                <td class="diff-num">{number_text(line.old_number)}</td>
                                                <td class="diff-num">{number_text(line.new_number)}</td>
                                                <td class="diff-sign">{sign}</td>
                                                <td class=class>{line.text}</td>
                                            </tr>
                                        }
                                    })
                                    .collect_view()}
                            }
                        })
                        .collect_view()}
                </tbody>
            </table>
        </div>
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::presentation::state::diff_files::parse_patch;

    fn cell(kind: CellKind) -> Cell {
        Cell {
            number: 1,
            text: String::new(),
            kind,
        }
    }

    #[test]
    fn cell_classes_follow_kind_and_blank_side() {
        assert_eq!(cell_class(None), "diff-cell diff-cell-empty");
        assert_eq!(
            cell_class(Some(&cell(CellKind::Added))),
            "diff-cell diff-cell-add"
        );
        assert_eq!(
            cell_class(Some(&cell(CellKind::Removed))),
            "diff-cell diff-cell-del"
        );
        assert_eq!(cell_class(Some(&cell(CellKind::Context))), "diff-cell");
    }

    #[test]
    fn signs_match_kind() {
        assert_eq!(kind_sign(CellKind::Added), "+");
        assert_eq!(kind_sign(CellKind::Removed), "-");
        assert_eq!(kind_sign(CellKind::Context), " ");
    }

    #[test]
    fn rename_title_shows_both_paths() {
        let patch: &str =
            "diff --git a/a.rs b/b.rs\nsimilarity index 100%\nrename from a.rs\nrename to b.rs\n";
        let files: Vec<FileDiff> = parse_patch(patch);
        assert_eq!(file_title(&files[0]), "a.rs → b.rs");
    }

    #[test]
    fn missing_number_renders_empty() {
        assert_eq!(number_text(None), "");
        assert_eq!(number_text(Some(7)), "7");
    }
}
