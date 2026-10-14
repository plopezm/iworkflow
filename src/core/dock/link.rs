//! Enlace que abre su destino como pestaña del grupo actual en lugar de navegar.

use leptos::prelude::*;

use crate::core::dock::context::{TabNav, use_tab_nav};

/// Ruta sin el fragmento (`/#lane-1` → `/`): las pestañas se identifican solo por ruta.
pub fn path_without_fragment(href: &str) -> &str {
    href.split('#').next().unwrap_or(href)
}

/// Enlace a otra pantalla: un clic normal abre (o activa) su pestaña; con modificadores
/// (Ctrl/Cmd/Shift o botón central) conserva el comportamiento nativo del navegador.
#[component]
pub fn TabLink(
    #[prop(into)] href: String,
    #[prop(optional, into)] title: Option<String>,
    #[prop(optional, into)] class: String,
    children: Children,
) -> impl IntoView {
    let nav: TabNav = use_tab_nav();
    let target: String = path_without_fragment(&href).to_string();
    view! {
        <a
            href=href
            class=class
            on:click=move |ev: leptos::ev::MouseEvent| {
                if ev.button() != 0 || ev.ctrl_key() || ev.meta_key() || ev.shift_key() {
                    return;
                }
                ev.prevent_default();
                match &title {
                    Some(t) => nav.open_titled(&target, t),
                    None => nav.open(&target),
                }
            }
        >
            {children()}
        </a>
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn fragment_is_dropped_from_the_tab_path() {
        assert_eq!(path_without_fragment("/#lane-1"), "/");
        assert_eq!(path_without_fragment("/tasks/a"), "/tasks/a");
    }
}
