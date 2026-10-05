use leptos::prelude::*;

use crate::modules::terminal::infrastructure::adapters::inbound::dto::terminal_session_dto::TerminalSessionDto;

#[cfg(feature = "ssr")]
mod support {
    use leptos::prelude::*;
    use uuid::Uuid;

    use crate::modules::terminal::domain::errors::terminal_error::TerminalError;
    use crate::modules::terminal::domain::models::terminal_session_id::TerminalSessionId;
    use crate::modules::terminal::infrastructure::di::terminal_context::TerminalContext;

    pub fn context() -> Result<TerminalContext, ServerFnError> {
        use_context::<TerminalContext>()
            .ok_or_else(|| ServerFnError::new("TerminalContext no disponible"))
    }

    pub fn to_server_error(error: TerminalError) -> ServerFnError {
        ServerFnError::new(error.to_string())
    }

    pub fn workspace_id(id: &str) -> Result<Uuid, ServerFnError> {
        Uuid::parse_str(id).map_err(|_| ServerFnError::new("id de workspace inválido"))
    }

    pub fn session_id(id: &str) -> Result<TerminalSessionId, ServerFnError> {
        Uuid::parse_str(id)
            .map(TerminalSessionId::from_uuid)
            .map_err(|_| ServerFnError::new("id de sesión inválido"))
    }

    #[cfg(test)]
    mod tests {
        use super::*;

        #[test]
        fn domain_error_message_is_preserved() {
            assert_eq!(
                to_server_error(TerminalError::SessionNotFound),
                ServerFnError::ServerError("la sesión de terminal no existe".to_string())
            );
        }

        #[test]
        fn invalid_ids_are_rejected() {
            assert_eq!(
                workspace_id("nope"),
                Err(ServerFnError::ServerError(
                    "id de workspace inválido".to_string()
                ))
            );
            assert_eq!(
                session_id("nope"),
                Err(ServerFnError::ServerError(
                    "id de sesión inválido".to_string()
                ))
            );
        }

        #[test]
        fn valid_ids_are_parsed() {
            let id: Uuid = Uuid::from_u128(7);
            assert_eq!(workspace_id(&id.to_string()), Ok(id));
            assert_eq!(
                session_id(&id.to_string()),
                Ok(TerminalSessionId::from_uuid(id))
            );
        }
    }
}

#[server]
pub async fn open_terminal(
    workspace_id: String,
    cols: u16,
    rows: u16,
) -> Result<TerminalSessionDto, ServerFnError> {
    use crate::modules::terminal::domain::models::terminal_session::TerminalSession;
    use crate::modules::terminal::domain::models::terminal_size::TerminalSize;

    let size: TerminalSize = TerminalSize::new(cols, rows).map_err(support::to_server_error)?;
    let session: TerminalSession = support::context()?
        .open
        .execute(support::workspace_id(&workspace_id)?, size)
        .await
        .map_err(support::to_server_error)?;
    Ok(TerminalSessionDto::from(session))
}

#[server]
pub async fn list_terminals() -> Result<Vec<TerminalSessionDto>, ServerFnError> {
    use crate::modules::terminal::domain::models::terminal_session::TerminalSession;

    let sessions: Vec<TerminalSession> = support::context()?
        .list
        .execute()
        .await
        .map_err(support::to_server_error)?;
    Ok(sessions.into_iter().map(TerminalSessionDto::from).collect())
}

#[server]
pub async fn close_terminal(session_id: String) -> Result<(), ServerFnError> {
    support::context()?
        .close
        .execute(support::session_id(&session_id)?)
        .await
        .map_err(support::to_server_error)
}
