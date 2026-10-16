use leptos::prelude::*;

use crate::modules::jira::infrastructure::adapters::inbound::dto::jira_dto::{
    JiraConnectionDto, SaveConnectionDto,
};
use crate::modules::jira::infrastructure::adapters::inbound::server_fns::{
    DeleteJiraConnection, SaveJiraConnection, get_jira_connection,
};

/// The form can be sent with a URL and either a typed token or one already stored.
pub fn can_submit(url: &str, token: &str, token_stored: bool) -> bool {
    !url.trim().is_empty() && (!token.trim().is_empty() || token_stored)
}

/// Estado del panel de conexión: lectura, alta/edición y desconexión.
#[derive(Clone, Copy)]
pub struct JiraConnectionState {
    pub connection: Resource<Result<Option<JiraConnectionDto>, ServerFnError>>,
    pub save: ServerAction<SaveJiraConnection>,
    pub delete: ServerAction<DeleteJiraConnection>,
    pub form_url: RwSignal<String>,
    pub form_token: RwSignal<String>,
    pub form_jql: RwSignal<String>,
    pub form_accept_invalid_certs: RwSignal<bool>,
    pub editing: RwSignal<bool>,
    pub confirming_delete: RwSignal<bool>,
}

impl JiraConnectionState {
    pub fn new() -> Self {
        let save: ServerAction<SaveJiraConnection> = ServerAction::<SaveJiraConnection>::new();
        let delete: ServerAction<DeleteJiraConnection> =
            ServerAction::<DeleteJiraConnection>::new();
        let connection: Resource<Result<Option<JiraConnectionDto>, ServerFnError>> = Resource::new(
            move || (save.version().get(), delete.version().get()),
            |_| get_jira_connection(),
        );
        let state: Self = Self {
            connection,
            save,
            delete,
            form_url: RwSignal::new(String::new()),
            form_token: RwSignal::new(String::new()),
            form_jql: RwSignal::new(String::new()),
            form_accept_invalid_certs: RwSignal::new(false),
            editing: RwSignal::new(false),
            confirming_delete: RwSignal::new(false),
        };
        Effect::new(move |_| {
            if let Some(Ok(_)) = save.value().get() {
                state.form_token.set(String::new());
                state.editing.set(false);
            }
        });
        Effect::new(move |_| {
            if let Some(Ok(_)) = delete.value().get() {
                state.confirming_delete.set(false);
                state.editing.set(false);
            }
        });
        state
    }

    /// Opens the form prefilled with the stored connection (the token is never shown).
    pub fn begin_edit(&self, current: Option<&JiraConnectionDto>) {
        self.form_url.set(
            current
                .map(|c: &JiraConnectionDto| c.base_url.clone())
                .unwrap_or_default(),
        );
        self.form_jql.set(
            current
                .map(|c: &JiraConnectionDto| c.jql.clone())
                .unwrap_or_default(),
        );
        self.form_accept_invalid_certs
            .set(current.is_some_and(|c: &JiraConnectionDto| c.accept_invalid_certs));
        self.form_token.set(String::new());
        self.editing.set(true);
    }

    pub fn cancel_edit(&self) {
        self.form_token.set(String::new());
        self.editing.set(false);
    }

    pub fn submit(&self) {
        self.save.dispatch(SaveJiraConnection {
            input: SaveConnectionDto {
                base_url: self.form_url.get_untracked(),
                token: self.form_token.get_untracked(),
                jql: self.form_jql.get_untracked(),
                accept_invalid_certs: self.form_accept_invalid_certs.get_untracked(),
            },
        });
    }

    pub fn disconnect(&self) {
        self.delete.dispatch(DeleteJiraConnection {});
    }

    /// Version that changes whenever the connection is saved or removed.
    pub fn version(&self) -> Signal<usize> {
        let save: ServerAction<SaveJiraConnection> = self.save;
        let delete: ServerAction<DeleteJiraConnection> = self.delete;
        Signal::derive(move || save.version().get() + delete.version().get())
    }
}

impl Default for JiraConnectionState {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn needs_a_url_and_a_token_unless_one_is_stored() {
        assert!(can_submit("https://j", "tok", false));
        assert!(!can_submit("https://j", "  ", false));
        assert!(can_submit("https://j", "", true));
        assert!(!can_submit("  ", "tok", true));
    }
}
