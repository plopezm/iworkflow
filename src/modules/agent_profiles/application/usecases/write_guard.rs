use std::sync::Arc;

/// Cerrojo compartido que serializa el ciclo cargar → mutar → guardar de `profiles.json`.
pub type WriteGuard = Arc<tokio::sync::Mutex<()>>;

pub fn new_write_guard() -> WriteGuard {
    Arc::new(tokio::sync::Mutex::new(()))
}
