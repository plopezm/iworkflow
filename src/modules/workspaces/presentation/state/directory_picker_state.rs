use leptos::prelude::*;

use crate::modules::workspaces::infrastructure::adapters::inbound::dto::directory_dto::{
    DirectoryEntryDto, DirectoryListingDto,
};
use crate::modules::workspaces::infrastructure::adapters::inbound::server_fns::{
    browse_directories, create_directory,
};

/// Estado del selector de carpetas: solo pide datos al servidor mientras está abierto.
#[derive(Clone, Copy)]
pub struct DirectoryPickerState {
    pub open: RwSignal<bool>,
    pub current: RwSignal<Option<String>>,
    /// Texto del filtro por nombre sobre las subcarpetas listadas.
    pub filter: RwSignal<String>,
    /// Crea una carpeta `(padre, nombre)` y, si tiene éxito, navega a ella.
    pub create_folder: Action<(String, String), Result<String, ServerFnError>>,
    pub listing: Resource<Result<DirectoryListingDto, ServerFnError>>,
}

/// Entradas cuyo nombre contiene `needle` (sin distinguir mayúsculas); vacío = todas.
pub fn filter_entries(entries: &[DirectoryEntryDto], needle: &str) -> Vec<DirectoryEntryDto> {
    let needle: String = needle.trim().to_lowercase();
    entries
        .iter()
        .filter(|e: &&DirectoryEntryDto| e.name.to_lowercase().contains(&needle))
        .cloned()
        .collect()
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
        let filter: RwSignal<String> = RwSignal::new(String::new());
        let create_folder: Action<(String, String), Result<String, ServerFnError>> =
            Action::new(move |(parent, name): &(String, String)| {
                let parent: String = parent.clone();
                let name: String = name.clone();
                async move {
                    let created: Result<String, ServerFnError> =
                        create_directory(parent, name).await;
                    if let Ok(path) = &created {
                        filter.set(String::new());
                        current.set(Some(path.clone()));
                    }
                    created
                }
            });
        Self {
            open,
            current,
            filter,
            create_folder,
            listing,
        }
    }

    /// Abre el selector partiendo de `path` (o de la carpeta home si está vacío).
    pub fn open_at(&self, path: String) {
        let start: Option<String> = Some(path).filter(|p: &String| !p.trim().is_empty());
        self.current.set(start);
        self.filter.set(String::new());
        self.create_folder.value().set(None);
        self.open.set(true);
    }

    pub fn navigate(&self, path: String) {
        self.filter.set(String::new());
        self.create_folder.value().set(None);
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

#[cfg(test)]
mod tests {
    use super::*;

    fn entry(name: &str) -> DirectoryEntryDto {
        DirectoryEntryDto {
            name: name.to_string(),
            path: format!("/x/{name}"),
            is_git_repo: false,
        }
    }

    #[test]
    fn filter_is_case_insensitive_and_trims() {
        let entries: Vec<DirectoryEntryDto> = vec![entry("Alpha"), entry("beta")];
        let result: Vec<DirectoryEntryDto> = filter_entries(&entries, "  ALP ");
        assert_eq!(result, vec![entry("Alpha")]);
    }

    #[test]
    fn empty_filter_returns_everything() {
        let entries: Vec<DirectoryEntryDto> = vec![entry("a"), entry("b")];
        assert_eq!(filter_entries(&entries, "").len(), 2);
    }
}
