/// Ruta que abre una terminal nueva para un workspace.
pub fn terminal_path(workspace_id: &str) -> String {
    format!("/workspaces/{workspace_id}/terminal")
}

/// Ruta de una sesión de terminal concreta.
pub fn terminal_session_path(session_id: &str) -> String {
    format!("/terminals/{session_id}")
}

/// Ruta del listado de sesiones de terminal.
pub fn terminals_path() -> &'static str {
    "/terminals"
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn builds_paths() {
        assert_eq!(terminal_path("abc"), "/workspaces/abc/terminal");
        assert_eq!(terminal_session_path("s1"), "/terminals/s1");
        assert_eq!(terminals_path(), "/terminals");
    }
}
