//! Puente con `dock.js` (solo navegador); fuera de él las funciones no hacen nada.

#[cfg(feature = "hydrate")]
mod bindings {
    use wasm_bindgen::JsValue;
    use wasm_bindgen::prelude::wasm_bindgen;

    #[wasm_bindgen(module = "/src/core/dock/dock.js")]
    extern "C" {
        pub fn pi_dock_resize(event: &JsValue, vertical: bool, on_done: &JsValue);
        pub fn pi_dock_drag_start(event: &JsValue, payload: &str);
        pub fn pi_dock_reveal_tab(element: &JsValue);
    }
}

/// Arrastra el divisor que originó `event` y entrega a `on_done` la fracción final (0..1).
pub fn start_resize(
    event: leptos::ev::PointerEvent,
    vertical: bool,
    on_done: impl FnOnce(f64) + 'static,
) {
    #[cfg(feature = "hydrate")]
    {
        let callback: wasm_bindgen::JsValue = wasm_bindgen::closure::Closure::once_into_js(on_done);
        bindings::pi_dock_resize(&wasm_bindgen::JsValue::from(event), vertical, &callback);
    }
    #[cfg(not(feature = "hydrate"))]
    {
        drop((event, vertical, on_done));
    }
}

/// Inicia el arrastre HTML5 de una pestaña con `payload` como dato.
pub fn start_drag(event: leptos::ev::DragEvent, payload: &str) {
    #[cfg(feature = "hydrate")]
    {
        bindings::pi_dock_drag_start(&wasm_bindgen::JsValue::from(event), payload);
    }
    #[cfg(not(feature = "hydrate"))]
    {
        drop((event, payload));
    }
}

/// Desplaza la barra de pestañas horizontalmente lo justo para que `element` quede visible.
pub fn reveal_tab(element: leptos::web_sys::HtmlDivElement) {
    #[cfg(feature = "hydrate")]
    {
        bindings::pi_dock_reveal_tab(&wasm_bindgen::JsValue::from(element));
    }
    #[cfg(not(feature = "hydrate"))]
    {
        drop(element);
    }
}
