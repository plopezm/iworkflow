use leptos::prelude::*;

/// Clases del panel según esté plegado o no.
pub fn panel_class(collapsed: bool) -> &'static str {
    if collapsed {
        "side-panel side-panel-collapsed"
    } else {
        "side-panel"
    }
}

/// Texto accesible del botón de plegado según el estado actual.
pub fn toggle_label<'a>(collapsed: bool, expand: &'a str, collapse: &'a str) -> &'a str {
    if collapsed { expand } else { collapse }
}

/// Panel lateral plegable con cabecera (título + contador) y cuerpo desplazable. Genérico: los
/// textos y el contenido los aporta quien lo usa. Plegado, solo queda la cabecera compacta.
#[component]
pub fn CollapsiblePanel(
    #[prop(into)] title: String,
    #[prop(into)] count: Signal<usize>,
    collapsed: RwSignal<bool>,
    #[prop(into)] expand_label: String,
    #[prop(into)] collapse_label: String,
    children: Children,
) -> impl IntoView {
    let toggle_title: String = title.clone();
    let label: Signal<String> = Signal::derive(move || {
        toggle_label(collapsed.get(), &expand_label, &collapse_label).to_string()
    });
    view! {
        <aside class=move || panel_class(collapsed.get())>
            <header class="side-panel-header">
                <button
                    type="button"
                    class="side-panel-toggle"
                    aria-expanded=move || (!collapsed.get()).to_string()
                    title=move || label.get()
                    aria-label=move || format!("{}: {toggle_title}", label.get())
                    on:click=move |_| collapsed.update(|c: &mut bool| *c = !*c)
                >
                    <span aria-hidden="true">
                        {move || if collapsed.get() { "▸" } else { "▾" }}
                    </span>
                </button>
                <h2 class="side-panel-title">{title}</h2>
                <span class="badge badge-muted" aria-live="polite">
                    {move || count.get()}
                </span>
            </header>
            <div class="side-panel-body" hidden=move || collapsed.get()>
                {children()}
            </div>
        </aside>
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn panel_class_marks_the_collapsed_state() {
        assert_eq!(panel_class(false), "side-panel");
        assert_eq!(panel_class(true), "side-panel side-panel-collapsed");
    }

    #[test]
    fn toggle_label_offers_the_opposite_action() {
        assert_eq!(toggle_label(true, "Expand", "Collapse"), "Expand");
        assert_eq!(toggle_label(false, "Expand", "Collapse"), "Collapse");
    }
}
