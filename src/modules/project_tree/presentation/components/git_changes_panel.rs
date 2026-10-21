use leptos::prelude::*;

use crate::core::components::layout::ErrorText;
use crate::core::components::tree::TreeIcon;
use crate::core::i18n::Msg;
use crate::modules::project_tree::infrastructure::adapters::inbound::dto::project_tree_dto::GitChangeDto;
use crate::modules::project_tree::presentation::state::git_changes_state::{
    ChangeGroups, GitChangesState, file_name, group_changes, kind_letter, parent_dir, paths_of,
};
use crate::modules::project_tree::presentation::state::git_status::status_class;

const CHANGES: Msg = Msg {
    en: "Changes",
    es: "Cambios",
    zh: "更改",
    de: "Änderungen",
    fr: "Modifications",
};
const STAGED: Msg = Msg {
    en: "Staged",
    es: "Con stage",
    zh: "已暂存",
    de: "Bereitgestellt",
    fr: "Indexées",
};
const UNTRACKED: Msg = Msg {
    en: "Untracked files",
    es: "Archivos sin seguimiento",
    zh: "未跟踪的文件",
    de: "Nicht verfolgte Dateien",
    fr: "Fichiers non suivis",
};
const STAGE_ALL: Msg = Msg {
    en: "Stage all",
    es: "Hacer stage de todo",
    zh: "全部暂存",
    de: "Alles bereitstellen",
    fr: "Tout indexer",
};
const UNSTAGE_ALL: Msg = Msg {
    en: "Unstage all",
    es: "Quitar todo del stage",
    zh: "全部取消暂存",
    de: "Alle entfernen",
    fr: "Tout désindexer",
};
const STAGE_FILE: Msg = Msg {
    en: "Stage",
    es: "Hacer stage de",
    zh: "暂存",
    de: "Bereitstellen:",
    fr: "Indexer",
};
const UNSTAGE_FILE: Msg = Msg {
    en: "Unstage",
    es: "Quitar del stage",
    zh: "取消暂存",
    de: "Entfernen:",
    fr: "Désindexer",
};
const NO_CHANGES: Msg = Msg {
    en: "No changes: the working tree is clean.",
    es: "Sin cambios: el árbol de trabajo está limpio.",
    zh: "没有更改：工作树是干净的。",
    de: "Keine Änderungen: Der Arbeitsbaum ist sauber.",
    fr: "Aucun changement : l'arbre de travail est propre.",
};
const LOADING: Msg = Msg {
    en: "Loading changes…",
    es: "Cargando cambios…",
    zh: "正在加载更改…",
    de: "Änderungen werden geladen…",
    fr: "Chargement des modifications…",
};
const RELOAD: Msg = Msg {
    en: "Reload",
    es: "Recargar",
    zh: "重新加载",
    de: "Neu laden",
    fr: "Recharger",
};

/// What a toggle does to the files it is attached to.
#[derive(Clone, Copy, PartialEq, Eq)]
enum Toggle {
    Stage,
    Unstage,
}

/// Runs `toggle` on `paths` through the matching action.
fn run(state: GitChangesState, toggle: Toggle, paths: Vec<String>) {
    match toggle {
        Toggle::Stage => state.stage.dispatch(paths),
        Toggle::Unstage => state.unstage.dispatch(paths),
    };
}

/// One file row: name, directory, status letter and the stage/unstage button.
#[component]
fn ChangeRow(change: GitChangeDto, toggle: Toggle, state: GitChangesState) -> impl IntoView {
    let GitChangeDto { path, kind, .. } = change;
    let (symbol, verb): (&str, Msg) = match toggle {
        Toggle::Stage => ("+", STAGE_FILE),
        Toggle::Unstage => ("−", UNSTAGE_FILE),
    };
    let label: String = format!("{} {path}", verb.get());
    let name: String = file_name(&path).to_string();
    let dir: String = parent_dir(&path).to_string();
    let class: String = format!("tree-row gap-2 pl-3 {}", status_class(Some(kind)));
    let target: Vec<String> = vec![path.clone()];
    view! {
        <li data-testid="git-row" data-path=path.clone()>
            <div class=class>
                <TreeIcon />
                <span class="truncate" title=path.clone()>
                    {name}
                </span>
                <span class="min-w-0 flex-1 truncate text-xs text-muted">{dir}</span>
                <span class="text-xs font-semibold" aria-hidden="true">
                    {kind_letter(kind)}
                </span>
                <button
                    type="button"
                    class="icon-btn"
                    data-testid="git-toggle"
                    title=label.clone()
                    aria-label=label
                    on:click=move |_| run(state, toggle, target.clone())
                >
                    {symbol}
                </button>
            </div>
        </li>
    }
}

/// A titled list of changes with a bulk button.
#[component]
fn ChangeSection(
    testid: &'static str,
    title: Msg,
    changes: Vec<GitChangeDto>,
    toggle: Toggle,
    state: GitChangesState,
) -> impl IntoView {
    if changes.is_empty() {
        return ().into_any();
    }
    let count: usize = changes.len();
    let bulk: Vec<String> = paths_of(&changes);
    let (symbol, label): (&str, &str) = match toggle {
        Toggle::Stage => ("+", STAGE_ALL.get()),
        Toggle::Unstage => ("−", UNSTAGE_ALL.get()),
    };
    view! {
        <section data-testid=testid>
            <div class="flex h-8 items-center gap-2 px-3 text-xs font-semibold uppercase tracking-wide text-muted">
                <span class="flex-1 truncate">{format!("{} {count}", title.get())}</span>
                <button
                    type="button"
                    class="icon-btn"
                    data-testid="git-section-toggle"
                    title=label
                    aria-label=label
                    on:click=move |_| run(state, toggle, bulk.clone())
                >
                    {symbol}
                </button>
            </div>
            <ul>
                <For
                    each=move || changes.clone()
                    key=|c: &GitChangeDto| c.path.clone()
                    children=move |c: GitChangeDto| view! { <ChangeRow change=c toggle state /> }
                />
            </ul>
        </section>
    }
    .into_any()
}

/// Source-control style list of the git changes of a workspace or task worktree
/// (`task:<id>`): choose which files are staged for the next commit and which are not.
#[component]
pub fn GitChangesPanel(workspace_id: String) -> impl IntoView {
    let state: GitChangesState = GitChangesState::new(workspace_id);
    view! {
        <div class="flex h-full min-h-0 flex-col bg-surface" data-testid="git-changes" node_ref=state.root>
            <div class="tree-panel-header">
                <h1 class="min-w-0 flex-1 truncate text-sm font-semibold">{CHANGES.get()}</h1>
                <button
                    type="button"
                    class="icon-btn"
                    title=RELOAD.get()
                    aria-label=RELOAD.get()
                    on:click=move |_| state.refresh()
                >
                    "↻"
                </button>
            </div>
            {move || {
                state.error().map(|message: String| view! { <div class="px-3 py-2"><ErrorText message /></div> })
            }}
            <div class="min-h-0 flex-1 overflow-y-auto py-1">
                <Transition fallback=|| {
                    view! { <p class="px-3 py-2 text-xs text-muted">{LOADING.get()}</p> }
                }>
                    {move || Suspend::new(async move {
                        match state.changes.await {
                            Err(e) => {
                                view! {
                                    <div class="px-3 py-2">
                                        <ErrorText message=e.to_string() />
                                    </div>
                                }
                                    .into_any()
                            }
                            Ok(changes) => {
                                let groups: ChangeGroups = group_changes(&changes);
                                if groups.is_empty() {
                                    return view! {
                                        <p class="px-3 py-2 text-xs text-muted" data-testid="git-empty">
                                            {NO_CHANGES.get()}
                                        </p>
                                    }
                                        .into_any();
                                }
                                view! {
                                    <ChangeSection
                                        testid="git-section-staged"
                                        title=STAGED
                                        changes=groups.staged
                                        toggle=Toggle::Unstage
                                        state
                                    />
                                    <ChangeSection
                                        testid="git-section-unstaged"
                                        title=CHANGES
                                        changes=groups.unstaged
                                        toggle=Toggle::Stage
                                        state
                                    />
                                    <ChangeSection
                                        testid="git-section-untracked"
                                        title=UNTRACKED
                                        changes=groups.untracked
                                        toggle=Toggle::Stage
                                        state
                                    />
                                }
                                    .into_any()
                            }
                        }
                    })}
                </Transition>
            </div>
        </div>
    }
}
