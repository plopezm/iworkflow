use leptos::prelude::*;

/// Chevron that rotates when its row is expanded (sin dominio: sirve a cualquier árbol).
#[component]
pub fn TreeChevron(#[prop(into)] open: Signal<bool>) -> impl IntoView {
    view! {
        <svg
            class=move || if open.get() { "tree-chevron tree-chevron-open" } else { "tree-chevron" }
            viewBox="0 0 16 16"
            fill="none"
            aria-hidden="true"
        >
            <path
                d="M6 4l4 4-4 4"
                stroke="currentColor"
                stroke-width="1.5"
                stroke-linecap="round"
                stroke-linejoin="round"
            />
        </svg>
    }
}

/// Icono de fichero (hoja con esquina doblada) o de carpeta.
#[component]
pub fn TreeIcon(
    #[prop(optional)] is_dir: bool,
    #[prop(optional, into)] open: Signal<bool>,
) -> impl IntoView {
    if is_dir {
        view! {
            <svg class="tree-icon" viewBox="0 0 16 16" fill="none" aria-hidden="true">
                <path
                    d=move || {
                        if open.get() {
                            "M2 5.5A1.5 1.5 0 013.5 4h2.6l1.3 1.5h5.1A1.5 1.5 0 0114 7v4.5a1.5 1.5 0 01-1.5 1.5h-9A1.5 1.5 0 012 11.5v-6z"
                        } else {
                            "M2 4.5A1.5 1.5 0 013.5 3h2.6l1.3 1.5h5.1A1.5 1.5 0 0114 6v5.5a1.5 1.5 0 01-1.5 1.5h-9A1.5 1.5 0 012 11.5v-7z"
                        }
                    }
                    stroke="currentColor"
                    stroke-width="1.2"
                    stroke-linejoin="round"
                />
            </svg>
        }
        .into_any()
    } else {
        view! {
            <svg class="tree-icon" viewBox="0 0 16 16" fill="none" aria-hidden="true">
                <path
                    d="M4 2.5h5l3 3v7a1 1 0 01-1 1H4a1 1 0 01-1-1v-9a1 1 0 011-1z"
                    stroke="currentColor"
                    stroke-width="1.2"
                    stroke-linejoin="round"
                />
                <path d="M9 2.5v3h3" stroke="currentColor" stroke-width="1.2" stroke-linejoin="round" />
            </svg>
        }
        .into_any()
    }
}
