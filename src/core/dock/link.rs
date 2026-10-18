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
    #[prop(optional, into)] class: Signal<String>,
    #[prop(optional, into)] style: Option<String>,
    /// Abre la pestaña en el grupo de la derecha en lugar de en el actual.
    #[prop(optional)]
    beside: bool,
    /// Si ya hay una pestaña abierta cuya ruta cumple el predicado, la activa en lugar de
    /// abrir `href` (p. ej. volver al tablero abierto, sea cual sea su workspace).
    #[prop(optional_no_strip)]
    reuse: Option<fn(&str) -> bool>,
    children: Children,
) -> impl IntoView {
    let nav: TabNav = use_tab_nav();
    let target: String = path_without_fragment(&href).to_string();
    view! {
        <a
            href=href
            class=move || class.get()
            style=style
            on:click=move |ev: leptos::ev::MouseEvent| {
                if ev.button() != 0 || ev.ctrl_key() || ev.meta_key() || ev.shift_key() {
                    return;
                }
                ev.prevent_default();
                if beside {
                    nav.open_beside(&target);
                    return;
                }
                if let Some(reuse) = reuse {
                    nav.open_or_reuse(&target, reuse);
                    return;
                }
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

/// Sustituye la pestaña actual por `path` en cuanto se monta (equivale a una redirección).
#[component]
pub fn TabRedirect(#[prop(into)] path: String) -> impl IntoView {
    let nav: TabNav = use_tab_nav();
    Effect::new(move |_| nav.replace(&path));
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
