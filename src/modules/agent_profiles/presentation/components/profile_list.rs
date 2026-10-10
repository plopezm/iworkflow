use leptos::prelude::*;

use crate::core::components::layout::Badge;

/// Elemento de la lista de selección: nombre y etiqueta opcional (p. ej. "Activo").
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ProfileListItem {
    pub name: String,
    pub badge: Option<String>,
}

/// Lista de elementos seleccionables con el seleccionado resaltado.
#[component]
pub fn ProfileList(
    items: Vec<ProfileListItem>,
    #[prop(into)] selected: Signal<Option<String>>,
    on_select: Callback<String>,
    #[prop(into, default = "No hay elementos.".to_string())] empty_text: String,
) -> impl IntoView {
    if items.is_empty() {
        return view! { <p class="muted">{empty_text}</p> }.into_any();
    }
    view! {
        <ul class="stack gap-1.5">
            {items
                .into_iter()
                .map(|item: ProfileListItem| {
                    let name: String = item.name.clone();
                    let compare: String = item.name.clone();
                    let pick: String = item.name.clone();
                    view! {
                        <li>
                            <button
                                type="button"
                                class=move || {
                                    if selected.get().as_deref() == Some(compare.as_str()) {
                                        "chip chip-selected w-full justify-between"
                                    } else {
                                        "chip w-full justify-between"
                                    }
                                }
                                on:click=move |_| on_select.run(pick.clone())
                            >
                                <span class="truncate">{name}</span>
                                {item
                                    .badge
                                    .map(|text: String| view! { <Badge text variant="ok" /> })}
                            </button>
                        </li>
                    }
                })
                .collect_view()}
        </ul>
    }
    .into_any()
}
