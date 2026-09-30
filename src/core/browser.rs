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
"#)]
    extern "C" {
        pub fn pi_theme_read(key: &str) -> Option<String>;
        pub fn pi_theme_write(key: &str, value: &str);
        pub fn pi_prefers_dark() -> bool;
        pub fn pi_set_document_theme(theme: &str);
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
