use leptos::prelude::*;

use crate::modules::workspaces::infrastructure::adapters::inbound::dto::directory_dto::DirectoryListingDto;
use crate::modules::workspaces::infrastructure::adapters::inbound::server_fns::browse_directories;

/// Estado del selector de carpetas: solo pide datos al servidor mientras está abierto.
#[derive(Clone, Copy)]
pub struct DirectoryPickerState {
    pub open: RwSignal<bool>,
    pub current: RwSignal<Option<String>>,
    pub listing: Resource<Result<DirectoryListingDto, ServerFnError>>,
}

fn empty_listing() -> DirectoryListingDto {
    DirectoryListingDto {
        path: String::new(),
        parent: None,
        entries: Vec::new(),
    }
}

impl DirectoryPickerState {
    pub fn new() -> Self {
        let open: RwSignal<bool> = RwSignal::new(false);
        let current: RwSignal<Option<String>> = RwSignal::new(None);
        let listing: Resource<Result<DirectoryListingDto, ServerFnError>> = Resource::new(
            move || open.get().then(|| current.get()),
            |source: Option<Option<String>>| async move {
                match source {
                    // Cerrado: no se realiza ninguna llamada al servidor.
                    None => Ok(empty_listing()),
                    Some(path) => browse_directories(path).await,
                }
            },
        );
        Self {
            open,
            current,
            listing,
        }
    }

    /// Abre el selector partiendo de `path` (o de la carpeta home si está vacío).
    pub fn open_at(&self, path: String) {
        let start: Option<String> = Some(path).filter(|p: &String| !p.trim().is_empty());
        self.current.set(start);
        self.open.set(true);
    }

    pub fn navigate(&self, path: String) {
        self.current.set(Some(path));
    }

    pub fn close(&self) {
        self.open.set(false);
    }
}

impl Default for DirectoryPickerState {
    fn default() -> Self {
        Self::new()
    }
}
