use leptos::prelude::*;

#[cfg(feature = "hydrate")]
const FOCUS_DELAY: std::time::Duration = std::time::Duration::from_millis(0);

#[cfg(feature = "hydrate")]
fn focus_first_field() {
    use wasm_bindgen::JsCast;

    let Some(document) = web_sys::window().and_then(|w: web_sys::Window| w.document()) else {
        return;
    };
    let Ok(Some(element)) = document.query_selector(
        "#modal-panel input, #modal-panel select, #modal-panel textarea, #modal-panel button",
    ) else {
        return;
    };
    if let Some(html_element) = element.dyn_ref::<web_sys::HtmlElement>() {
        let _ = html_element.focus();
    }
}

/// Ventana modal generica: bottom sheet en movil, dialogo centrado en escritorio.
/// Se muestra mientras `open` sea `true` y notifica el cierre con `on_close`
/// (clic en el fondo, boton de cierre, tecla Escape, o quien controle la senal desde fuera).
#[component]
pub fn Modal(
    #[prop(into)] title: Signal<String>,
    open: RwSignal<bool>,
    on_close: Callback<()>,
    children: ChildrenFn,
) -> impl IntoView {
    #[cfg(feature = "hydrate")]
    {
        use wasm_bindgen::JsCast;

        let previous_focus: StoredValue<Option<web_sys::HtmlElement>, LocalStorage> =
            StoredValue::new_local(None);
        Effect::new(move |prev_open: Option<bool>| {
            let is_open: bool = open.get();
            if is_open && prev_open != Some(true) {
                let active: Option<web_sys::HtmlElement> = web_sys::window()
                    .and_then(|w: web_sys::Window| w.document())
                    .and_then(|d: web_sys::Document| d.active_element())
                    .and_then(|el: web_sys::Element| el.dyn_into::<web_sys::HtmlElement>().ok());
                previous_focus.set_value(active);
                set_timeout(focus_first_field, FOCUS_DELAY);
            } else if !is_open && prev_open == Some(true) {
                previous_focus.update_value(|el: &mut Option<web_sys::HtmlElement>| {
                    if let Some(element) = el.take() {
                        let _ = element.focus();
                    }
                });
            }
            is_open
        });
    }

    view! {
        <Show when=move || open.get()>
            <div
                class="fixed inset-0 z-50 flex items-end justify-center bg-black/60 backdrop-blur-sm sm:items-center"
                on:click=move |_| on_close.run(())
                on:keydown=move |ev: leptos::ev::KeyboardEvent| {
                    if ev.key() == "Escape" {
                        on_close.run(());
                    }
                }
            >
                <div
                    id="modal-panel"
                    class="stack max-h-[90dvh] w-full overflow-y-auto rounded-t-2xl border border-border bg-surface p-4 sm:max-w-lg sm:rounded-2xl sm:p-6"
                    role="dialog"
                    aria-modal="true"
                    aria-labelledby="modal-title"
                    on:click=move |ev| ev.stop_propagation()
                >
                    <div class="row justify-between">
                        <h2 id="modal-title" class="text-lg font-semibold text-fg">
                            {move || title.get()}
                        </h2>
                        <button
                            class="btn-ghost min-h-10 min-w-10 px-2"
                            aria-label="Cerrar"
                            on:click=move |_| on_close.run(())
                        >
                            "x"
                        </button>
                    </div>
                    {children()}
                </div>
            </div>
        </Show>
    }
}
