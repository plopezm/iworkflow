use leptos::prelude::*;

use crate::core::components::layout::Badge;
use crate::core::components::modal::Modal;
use crate::modules::workspaces::infrastructure::adapters::inbound::dto::directory_dto::{
    DirectoryEntryDto, DirectoryListingDto,
};
use crate::modules::workspaces::presentation::state::directory_picker_state::{
    DirectoryPickerState, filter_entries,
};

const MODAL_TITLE: &str = "Elegir carpeta";

/// Selector de carpetas del servidor: navega ancestros/hijos y confirma con `on_select`.
#[component]
pub fn DirectoryPicker(state: DirectoryPickerState, on_select: Callback<String>) -> impl IntoView {
    let on_close: Callback<()> = Callback::new(move |_: ()| state.close());

    view! {
        <Modal
            title=Signal::derive(|| MODAL_TITLE.to_string())
            open=state.open
            on_close=on_close
        >
            <Transition fallback=|| view! { <p class="muted">"Cargando…"</p> }>
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
                    aria-label="Ruta"
                    placeholder="/ruta/a/la/carpeta"
                    value=path
                />
                <button type="submit" class="btn-ghost">
                    "Ir"
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
                    "↑ Subir"
                </button>
                <button
                    type="button"
                    class="btn-primary"
                    on:click=move |_| on_select.run(select_path.clone())
                >
                    "Usar esta carpeta"
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
                    aria-label="Nombre de la nueva carpeta"
                    placeholder="Nueva carpeta aquí"
                />
                <button type="submit" class="btn-ghost">
                    "Crear"
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
                aria-label="Filtrar carpetas"
                placeholder="Filtrar carpetas…"
                prop:value=move || state.filter.get()
                on:input=move |ev| state.filter.set(event_target_value(&ev))
            />
            <ul class="stack max-h-72 gap-1 overflow-y-auto">
                {move || {
                    let items: Vec<DirectoryEntryDto> = visible.get();
                    if items.is_empty() {
                        return view! { <li class="muted px-3 py-2 text-sm">"Sin subcarpetas"</li> }
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
