use leptos::prelude::*;

use crate::modules::preferences::infrastructure::adapters::inbound::dto::network_access_dto::NetworkAccessDto;

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

/// Whether the server is (or will be, after a restart) reachable from other devices, and how
/// to open it from one: access URL, QR code and token.
#[server]
pub async fn get_network_access() -> Result<NetworkAccessDto, ServerFnError> {
    use crate::core::listen::{ListenInfo, local_network_ip};
    use crate::modules::preferences::infrastructure::adapters::inbound::access_link::{
        build_access_url, render_qr_svg,
    };
    use crate::modules::preferences::infrastructure::di::preferences_context::PreferencesContext;

    let context: PreferencesContext = support::context()?;
    let enabled: bool = context
        .get_network_access
        .execute()
        .await
        .map_err(support::to_server_error)?;
    let require_token: bool = context
        .get_require_access_token
        .execute()
        .await
        .map_err(support::to_server_error)?;
    let listen: ListenInfo = use_context::<ListenInfo>()
        .ok_or_else(|| ServerFnError::new("ListenInfo no disponible"))?;
    let active: bool = listen.is_exposed();
    let lan_ip: Option<String> = local_network_ip().map(|ip: std::net::IpAddr| ip.to_string());
    let dto: NetworkAccessDto = NetworkAccessDto::new(
        enabled,
        active,
        listen.locked_by_env,
        listen.addr.port(),
        lan_ip,
    )
    .with_token_state(require_token, listen.token_required);

    // The secret only leaves the server while the app is (or is about to be) reachable and a
    // token is demanded, now or after the next restart.
    let token_in_play: bool = (enabled || active) && (require_token || listen.token_required);
    if !token_in_play {
        return Ok(dto);
    }
    let token: String = context
        .get_access_token
        .execute()
        .await
        .map_err(support::to_server_error)?
        .as_str()
        .to_string();
    let access_url: Option<String> = dto
        .address
        .as_deref()
        .map(|host: &str| build_access_url(host, dto.port, Some(&token)));
    let qr_svg: Option<String> = access_url.as_deref().and_then(render_qr_svg);
    Ok(dto.with_access(Some(token), access_url, qr_svg))
}

/// Stores whether to listen on every network interface; it applies on the next server start.
#[server]
pub async fn save_network_access(enabled: bool) -> Result<(), ServerFnError> {
    support::context()?
        .save_network_access
        .execute(enabled)
        .await
        .map(|_| ())
        .map_err(support::to_server_error)
}

/// Stores whether network clients must present the access token; it applies on the next start.
#[server]
pub async fn save_require_access_token(required: bool) -> Result<(), ServerFnError> {
    support::context()?
        .save_require_access_token
        .execute(required)
        .await
        .map(|_| ())
        .map_err(support::to_server_error)
}

/// Replaces the access token. The old one stops working at once; this browser gets the new one
/// in its cookie so it stays logged in.
#[server]
pub async fn regenerate_access_token() -> Result<(), ServerFnError> {
    use axum::http::{HeaderValue, header::SET_COOKIE};

    use crate::modules::preferences::domain::models::access_token::AccessToken;
    use crate::modules::preferences::infrastructure::adapters::inbound::access_guard::AccessGuard;

    let token: AccessToken = support::context()?
        .regenerate_access_token
        .execute()
        .await
        .map_err(support::to_server_error)?;
    let guard: AccessGuard = use_context::<AccessGuard>()
        .ok_or_else(|| ServerFnError::new("AccessGuard no disponible"))?;
    guard.set_token(token);
    if guard.is_enforcing()
        && let Some(response) = use_context::<leptos_axum::ResponseOptions>()
    {
        let cookie: HeaderValue = HeaderValue::from_str(&guard.session_cookie()).map_err(
            |e: axum::http::header::InvalidHeaderValue| ServerFnError::new(e.to_string()),
        )?;
        response.insert_header(SET_COOKIE, cookie);
    }
    Ok(())
}
