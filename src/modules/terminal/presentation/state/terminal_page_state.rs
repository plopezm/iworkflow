use leptos::prelude::*;
use leptos_router::hooks::use_params_map;

/// Ruta de la página de terminal de un workspace.
pub fn terminal_path(workspace_id: &str) -> String {
    format!("/workspaces/{workspace_id}/terminal")
}

/// Título de la página a partir del id del workspace.
pub fn terminal_title(workspace_id: &str) -> String {
    if workspace_id.is_empty() {
        "Terminal".to_string()
    } else {
        format!("Terminal · {workspace_id}")
    }
}

/// Estado de la pantalla de terminal: solo necesita el id de la ruta.
#[derive(Clone, Copy)]
pub struct TerminalPageState {
    pub workspace_id: Memo<String>,
    pub title: Memo<String>,
}

impl TerminalPageState {
    pub fn new() -> Self {
        let params = use_params_map();
        let workspace_id: Memo<String> =
            Memo::new(move |_| params.with(|p| p.get("id").unwrap_or_default()));
        let title: Memo<String> = Memo::new(move |_| terminal_title(&workspace_id.get()));
        Self {
            workspace_id,
            title,
        }
    }
}

impl Default for TerminalPageState {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn builds_terminal_path() {
        assert_eq!(terminal_path("abc"), "/workspaces/abc/terminal");
    }

    #[test]
    fn builds_title_with_and_without_id() {
        assert_eq!(terminal_title("abc"), "Terminal · abc");
        assert_eq!(terminal_title(""), "Terminal");
    }
}
