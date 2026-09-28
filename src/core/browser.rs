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
