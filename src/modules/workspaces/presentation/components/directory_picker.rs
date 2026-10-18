use leptos::prelude::*;

use crate::core::components::layout::Badge;
use crate::core::components::modal::Modal;
use crate::core::i18n::Msg;
use crate::modules::workspaces::infrastructure::adapters::inbound::dto::directory_dto::{
    DirectoryEntryDto, DirectoryListingDto,
};
use crate::modules::workspaces::presentation::state::directory_picker_state::{
    DirectoryPickerState, filter_entries,
};

const MODAL_TITLE: Msg = Msg {
    en: "Choose folder",
    es: "Elegir carpeta",
    zh: "选择文件夹",
    de: "Ordner auswählen",
    fr: "Choisir un dossier",
};
const LOADING: Msg = Msg {
    en: "Loading…",
    es: "Cargando…",
    zh: "加载中…",
    de: "Wird geladen…",
    fr: "Chargement…",
};
const PATH: Msg = Msg {
    en: "Path",
    es: "Ruta",
    zh: "路径",
    de: "Pfad",
    fr: "Chemin",
};
const PATH_PLACEHOLDER: Msg = Msg {
    en: "/path/to/folder",
    es: "/ruta/a/la/carpeta",
    zh: "/路径/到/文件夹",
    de: "/pfad/zum/ordner",
    fr: "/chemin/vers/le/dossier",
};
const GO: Msg = Msg {
    en: "Go",
    es: "Ir",
    zh: "前往",
    de: "Los",
    fr: "Aller",
};
const UP: Msg = Msg {
    en: "↑ Up",
    es: "↑ Subir",
    zh: "↑ 上一级",
    de: "↑ Hoch",
    fr: "↑ Remonter",
};
const USE_THIS_FOLDER: Msg = Msg {
    en: "Use this folder",
    es: "Usar esta carpeta",
    zh: "使用此文件夹",
    de: "Diesen Ordner verwenden",
    fr: "Utiliser ce dossier",
};
const NEW_FOLDER_NAME: Msg = Msg {
    en: "New folder name",
    es: "Nombre de la nueva carpeta",
    zh: "新文件夹名称",
    de: "Name des neuen Ordners",
    fr: "Nom du nouveau dossier",
};
const NEW_FOLDER_HERE: Msg = Msg {
    en: "New folder here",
    es: "Nueva carpeta aquí",
    zh: "在此新建文件夹",
    de: "Neuer Ordner hier",
    fr: "Nouveau dossier ici",
};
const CREATE: Msg = Msg {
    en: "Create",
    es: "Crear",
    zh: "创建",
    de: "Erstellen",
    fr: "Créer",
};
const FILTER_FOLDERS_LABEL: Msg = Msg {
    en: "Filter folders",
    es: "Filtrar carpetas",
    zh: "筛选文件夹",
    de: "Ordner filtern",
    fr: "Filtrer les dossiers",
};
const FILTER_FOLDERS: Msg = Msg {
    en: "Filter folders…",
    es: "Filtrar carpetas…",
    zh: "筛选文件夹…",
    de: "Ordner filtern…",
    fr: "Filtrer les dossiers…",
};
const NO_SUBFOLDERS: Msg = Msg {
    en: "No subfolders",
    es: "Sin subcarpetas",
    zh: "没有子文件夹",
    de: "Keine Unterordner",
    fr: "Aucun sous-dossier",
};

/// Selector de carpetas del servidor: navega ancestros/hijos y confirma con `on_select`.
#[component]
pub fn DirectoryPicker(state: DirectoryPickerState, on_select: Callback<String>) -> impl IntoView {
    let on_close: Callback<()> = Callback::new(move |_: ()| state.close());
    let modal_title: String = MODAL_TITLE.get().to_string();

    view! {
        <Modal
            title=Signal::derive(move || modal_title.clone())
            open=state.open
            on_close=on_close
        >
            <Transition fallback=|| view! { <p class="muted">{LOADING.get()}</p> }>
                {move || Suspend::new(async move {
                    match state.listing.await {
                        Ok(listing) => render_listing(listing, state, on_select).into_any(),
                        Err(e) => {
                            view! { <p class="text-sm text-error">{e.to_string()}</p> }.into_any()
                        }
                    }
                })}
            </Transition>
        </Modal>
    }
}

fn render_listing(
    listing: DirectoryListingDto,
    state: DirectoryPickerState,
    on_select: Callback<String>,
) -> impl IntoView {
    let path: String = listing.path;
    let parent: Option<String> = listing.parent;
    let select_path: String = path.clone();
    let create_parent: String = path.clone();
    let entries: Vec<DirectoryEntryDto> = listing.entries;
    let path_input: NodeRef<leptos::html::Input> = NodeRef::new();
    let folder_input: NodeRef<leptos::html::Input> = NodeRef::new();

    let visible: Memo<Vec<DirectoryEntryDto>> =
        Memo::new(move |_| filter_entries(&entries, &state.filter.get()));

    view! {
        <div class="stack">
            <form
                class="row gap-2"
                on:submit=move |ev: leptos::ev::SubmitEvent| {
                    ev.prevent_default();
                    if let Some(input) = path_input.get() {
                        let typed: String = input.value();
                        if !typed.trim().is_empty() {
                            state.navigate(typed.trim().to_string());
                        }
                    }
                }
            >
                <input
                    node_ref=path_input
                    class="input min-w-0 flex-1"
                    type="text"
                    aria-label=PATH.get()
                    placeholder=PATH_PLACEHOLDER.get()
                    value=path
                />
                <button type="submit" class="btn-ghost">
                    {GO.get()}
                </button>
            </form>
            <div class="row gap-2">
                <button
                    type="button"
                    class="btn-ghost"
                    disabled=parent.is_none()
                    on:click=move |_| {
                        if let Some(p) = parent.clone() {
                            state.navigate(p);
                        }
                    }
                >
                    {UP.get()}
                </button>
                <button
                    type="button"
                    class="btn-primary"
                    on:click=move |_| on_select.run(select_path.clone())
                >
                    {USE_THIS_FOLDER.get()}
                </button>
            </div>
            <form
                class="row gap-2"
                on:submit=move |ev: leptos::ev::SubmitEvent| {
                    ev.prevent_default();
                    if let Some(input) = folder_input.get() {
                        let name: String = input.value();
                        if !name.trim().is_empty() {
                            state.create_folder.dispatch((create_parent.clone(), name));
                            input.set_value("");
                        }
                    }
                }
            >
                <input
                    node_ref=folder_input
                    class="input min-w-0 flex-1"
                    type="text"
                    aria-label=NEW_FOLDER_NAME.get()
                    placeholder=NEW_FOLDER_HERE.get()
                />
                <button type="submit" class="btn-ghost">
                    {CREATE.get()}
                </button>
            </form>
            {move || {
                state
                    .create_folder
                    .value()
                    .get()
                    .and_then(|r: Result<String, ServerFnError>| r.err())
                    .map(|e: ServerFnError| {
                        view! { <p class="text-sm text-error">{e.to_string()}</p> }
                    })
            }}
            <input
                class="input"
                type="search"
                aria-label=FILTER_FOLDERS_LABEL.get()
                placeholder=FILTER_FOLDERS.get()
                prop:value=move || state.filter.get()
                on:input=move |ev| state.filter.set(event_target_value(&ev))
            />
            <ul class="stack max-h-72 gap-1 overflow-y-auto">
                {move || {
                    let items: Vec<DirectoryEntryDto> = visible.get();
                    if items.is_empty() {
                        return view! { <li class="muted px-3 py-2 text-sm">{NO_SUBFOLDERS.get()}</li> }
                            .into_any();
                    }
                    items
                        .into_iter()
                        .map(|entry: DirectoryEntryDto| {
                            let target: String = entry.path.clone();
                            view! {
                                <li>
                                    <button
                                        type="button"
                                        class="btn-ghost row w-full justify-start gap-2 rounded-lg px-3 py-2 transition-colors hover:bg-surface-2"
                                        on:click=move |_| state.navigate(target.clone())
                                    >
                                        <span>{entry.name}</span>
                                        {entry
                                            .is_git_repo
                                            .then(|| view! { <Badge text="git" variant="info" /> })}
                                    </button>
                                </li>
                            }
                        })
                        .collect_view()
                        .into_any()
                }}
            </ul>
        </div>
    }
}
