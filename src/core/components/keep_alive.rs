//! Panel de pestaña que conserva su estado: se monta la primera vez que se muestra y después
//! solo se oculta, en vez de desmontarse como hace `<Show>`.

use leptos::prelude::*;

/// Clase del contenedor según esté activo o no (pure).
pub fn pane_class(active: bool) -> &'static str {
    if active { "contents" } else { "hidden" }
}

/// Monta `children` en la primera activación y los mantiene montados (ocultos) después.
#[component]
pub fn KeepAlive(#[prop(into)] active: Signal<bool>, children: ChildrenFn) -> impl IntoView {
    // Un `Memo` solo notifica cuando cambia: reactivar la pestaña no remonta el contenido.
    let seen: Memo<bool> =
        Memo::new(move |prev: Option<&bool>| prev.copied().unwrap_or(false) || active.get());
    view! {
        <div class=move || pane_class(active.get())>
            {move || seen.get().then(|| untrack(|| children()))}
        </div>
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn inactive_pane_is_hidden_and_active_pane_is_shown() {
        assert_eq!(pane_class(true), "contents");
        assert_eq!(pane_class(false), "hidden");
    }
}
