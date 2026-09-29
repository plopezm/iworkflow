use leptos::prelude::*;

use crate::core::components::layout::Badge;
use crate::core::components::modal::Modal;
use crate::modules::workspaces::infrastructure::adapters::inbound::dto::directory_dto::{
    DirectoryEntryDto, DirectoryListingDto,
};
use crate::modules::workspaces::presentation::state::directory_picker_state::DirectoryPickerState;

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

    view! {
        <div class="stack">
            <p class="break-all text-sm text-fg">{path}</p>
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
                    "Subir"
                </button>
                <button
                    type="button"
                    class="btn-primary"
                    on:click=move |_| on_select.run(select_path.clone())
                >
                    "Usar esta carpeta"
                </button>
            </div>
            <ul class="stack gap-1">
                {listing
                    .entries
                    .into_iter()
                    .map(|entry: DirectoryEntryDto| {
                        let target: String = entry.path.clone();
                        view! {
                            <li>
                                <button
                                    type="button"
                                    class="btn-ghost row w-full justify-start gap-2"
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
                    .collect_view()}
            </ul>
        </div>
    }
}
