//! Utilidades del navegador sin conocimiento de dominio.

/// Pide confirmación al usuario con el diálogo nativo. Fuera del navegador devuelve `false`.
pub fn confirm(message: &str) -> bool {
    #[cfg(feature = "hydrate")]
    {
        web_sys::window()
            .and_then(|window: web_sys::Window| window.confirm_with_message(message).ok())
            .unwrap_or(false)
    }
    #[cfg(not(feature = "hydrate"))]
    {
        let _message: &str = message;
        false
    }
}

/// Clave de `localStorage` donde se guarda el modo de tema.
pub const THEME_STORAGE_KEY: &str = "pi-theme";

#[cfg(feature = "hydrate")]
mod theme_js {
    use wasm_bindgen::prelude::wasm_bindgen;

    #[wasm_bindgen(inline_js = r#"
export function pi_theme_read(key) {
  try { return localStorage.getItem(key); } catch (_) { return null; }
}
export function pi_theme_write(key, value) {
  try { localStorage.setItem(key, value); } catch (_) {}
}
export function pi_prefers_dark() {
  return window.matchMedia('(prefers-color-scheme: dark)').matches;
}
export function pi_set_document_theme(theme) {
  document.documentElement.setAttribute('data-theme', theme);
}
export function pi_history_back() {
  if (window.history.length <= 1) return false;
  window.history.back();
  return true;
}
"#)]
    extern "C" {
        pub fn pi_theme_read(key: &str) -> Option<String>;
        pub fn pi_theme_write(key: &str, value: &str);
        pub fn pi_prefers_dark() -> bool;
        pub fn pi_set_document_theme(theme: &str);
        pub fn pi_history_back() -> bool;
    }
}

#[cfg(feature = "hydrate")]
mod follow_js {
    use wasm_bindgen::prelude::wasm_bindgen;

    #[wasm_bindgen(module = "/src/core/follow_bottom.js")]
    extern "C" {
        pub fn pi_follow_bottom(id: &str, auto: bool);
        pub fn pi_scroll_to_bottom(id: &str);
    }
}

/// Lee el modo de tema guardado. Fuera del navegador devuelve `None`.
pub fn read_stored_theme() -> Option<String> {
    #[cfg(feature = "hydrate")]
    {
        theme_js::pi_theme_read(THEME_STORAGE_KEY)
    }
    #[cfg(not(feature = "hydrate"))]
    {
        None
    }
}

/// Guarda el modo de tema; ignora errores de almacenamiento (p. ej. modo privado).
pub fn write_stored_theme(mode: &str) {
    #[cfg(feature = "hydrate")]
    {
        theme_js::pi_theme_write(THEME_STORAGE_KEY, mode);
    }
    #[cfg(not(feature = "hydrate"))]
    {
        let _mode: &str = mode;
    }
}

/// Indica si el sistema prefiere esquema oscuro. Fuera del navegador devuelve `false`.
pub fn system_prefers_dark() -> bool {
    #[cfg(feature = "hydrate")]
    {
        theme_js::pi_prefers_dark()
    }
    #[cfg(not(feature = "hydrate"))]
    {
        false
    }
}

/// Fija `data-theme` en `<html>` (`"light"` o `"dark"`).
pub fn set_document_theme(theme: &str) {
    #[cfg(feature = "hydrate")]
    {
        theme_js::pi_set_document_theme(theme);
    }
    #[cfg(not(feature = "hydrate"))]
    {
        let _theme: &str = theme;
    }
}

/// Con `auto` activo, mantiene el scroll del contenedor `id` en su final ante cualquier
/// cambio de contenido; con `auto` inactivo no lo mueve.
pub fn follow_bottom(id: &str, auto: bool) {
    #[cfg(feature = "hydrate")]
    {
        follow_js::pi_follow_bottom(id, auto);
    }
    #[cfg(not(feature = "hydrate"))]
    {
        let _id: &str = id;
        let _auto: bool = auto;
    }
}

/// Desplaza el contenedor `id` hasta su final y reactiva el seguimiento automático.
pub fn scroll_to_bottom(id: &str) {
    #[cfg(feature = "hydrate")]
    {
        follow_js::pi_scroll_to_bottom(id);
    }
    #[cfg(not(feature = "hydrate"))]
    {
        let _id: &str = id;
    }
}

/// Vuelve a la pantalla anterior del historial. Devuelve `false` si no hay a dónde volver
/// (o fuera del navegador), para que el llamador navegue a un destino por defecto.
pub fn history_back() -> bool {
    #[cfg(feature = "hydrate")]
    {
        theme_js::pi_history_back()
    }
    #[cfg(not(feature = "hydrate"))]
    {
        false
    }
}

#[cfg(feature = "hydrate")]
mod file_js {
    use wasm_bindgen::JsValue;
    use wasm_bindgen::prelude::wasm_bindgen;

    #[wasm_bindgen(inline_js = r#"
export function pi_download_text(filename, content, mime) {
  const blob = new Blob([content], { type: mime });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = filename;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(url);
}
export function pi_copy_text(text, on_done) {
  if (!navigator.clipboard) { on_done(false); return; }
  navigator.clipboard.writeText(text).then(() => on_done(true), () => on_done(false));
}
export function pi_read_first_file(input, on_done) {
  const file = input && input.files && input.files[0];
  if (!file) { on_done(false, ''); return; }
  file.text().then((text) => on_done(true, text), (error) => on_done(false, String(error)));
}
"#)]
    extern "C" {
        pub fn pi_download_text(filename: &str, content: &str, mime: &str);
        pub fn pi_copy_text(text: &str, on_done: &JsValue);
        pub fn pi_read_first_file(input: &JsValue, on_done: &JsValue);
    }
}

/// Descarga `content` como fichero de texto `filename`. Fuera del navegador no hace nada.
pub fn download_text_file(filename: &str, content: &str, mime: &str) {
    #[cfg(feature = "hydrate")]
    {
        file_js::pi_download_text(filename, content, mime);
    }
    #[cfg(not(feature = "hydrate"))]
    {
        let _args: (&str, &str, &str) = (filename, content, mime);
    }
}

/// Copia `text` al portapapeles y avisa del resultado (`true` = copiado) a `on_done`.
/// Fuera del navegador avisa con `false`.
pub fn copy_text_to_clipboard(text: &str, on_done: impl FnOnce(bool) + 'static) {
    #[cfg(feature = "hydrate")]
    {
        let callback: wasm_bindgen::JsValue = wasm_bindgen::closure::Closure::once_into_js(on_done);
        file_js::pi_copy_text(text, &callback);
    }
    #[cfg(not(feature = "hydrate"))]
    {
        let _text: &str = text;
        on_done(false);
    }
}

/// Lee como texto el primer fichero del `<input type=file>` que originó `event` y entrega
/// `Ok(texto)` o `Err(motivo)` a `on_done`. Fuera del navegador no hace nada.
pub fn read_event_file_text(
    event: leptos::ev::Event,
    on_done: impl FnOnce(Result<String, String>) + 'static,
) {
    #[cfg(feature = "hydrate")]
    {
        let target: wasm_bindgen::JsValue = event
            .target()
            .map(wasm_bindgen::JsValue::from)
            .unwrap_or(wasm_bindgen::JsValue::NULL);
        let callback: wasm_bindgen::JsValue =
            wasm_bindgen::closure::Closure::once_into_js(move |ok: bool, payload: String| {
                on_done(if ok { Ok(payload) } else { Err(payload) });
            });
        file_js::pi_read_first_file(&target, &callback);
    }
    #[cfg(not(feature = "hydrate"))]
    {
        drop((event, on_done));
    }
}
