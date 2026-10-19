use leptos::prelude::*;

#[cfg(feature = "ssr")]
mod support {
    use leptos::prelude::*;

    use crate::modules::preferences::domain::errors::preferences_error::PreferencesError;
    use crate::modules::preferences::infrastructure::di::preferences_context::PreferencesContext;

    pub fn context() -> Result<PreferencesContext, ServerFnError> {
        use_context::<PreferencesContext>()
            .ok_or_else(|| ServerFnError::new("PreferencesContext no disponible"))
    }

    pub fn to_server_error(error: PreferencesError) -> ServerFnError {
        ServerFnError::new(error.to_string())
    }

    #[cfg(test)]
    mod tests {
        use super::*;

        #[test]
        fn domain_error_message_is_preserved() {
            let error: ServerFnError =
                to_server_error(PreferencesError::UnsupportedLanguage("pt".to_string()));
            assert_eq!(
                error,
                ServerFnError::ServerError("idioma no soportado: pt".to_string())
            );
        }
    }
}

/// Stores the interface language (`en`, `es`, ...) so every browser opens in it.
#[server]
pub async fn save_language(code: String) -> Result<(), ServerFnError> {
    support::context()?
        .save_language
        .execute(&code)
        .await
        .map(|_| ())
        .map_err(support::to_server_error)
}
