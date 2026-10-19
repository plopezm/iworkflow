use serde::{Deserialize, Serialize};

/// What the settings card needs to show and toggle network access.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct NetworkAccessDto {
    /// Saved choice: listen on every interface the next time the server starts.
    pub enabled: bool,
    /// The running server is reachable from other machines right now.
    pub active: bool,
    /// `LEPTOS_SITE_ADDR` pins the address, so the saved choice has no effect.
    pub locked_by_env: bool,
    pub port: u16,
    /// Address other devices can use once the server is reachable (`None` if no network was
    /// detected or access is off).
    pub address: Option<String>,
    /// Saved choice: demand the access token from network clients.
    pub require_token: bool,
    /// The running server demands the access token right now.
    pub token_active: bool,
    /// The secret itself, only while it is in play (reachable and required).
    pub token: Option<String>,
    /// `http://<address>:<port>/?token=...`: opening it logs the device in.
    pub access_url: Option<String>,
    /// Inline SVG of the QR code for `access_url`.
    pub qr_svg: Option<String>,
}

impl NetworkAccessDto {
    /// `lan_ip` is only kept while access is on or active, so a private server shows no address.
    pub fn new(
        enabled: bool,
        active: bool,
        locked_by_env: bool,
        port: u16,
        lan_ip: Option<String>,
    ) -> Self {
        Self {
            enabled,
            active,
            locked_by_env,
            port,
            address: lan_ip.filter(|_: &String| enabled || active),
            require_token: true,
            token_active: false,
            token: None,
            access_url: None,
            qr_svg: None,
        }
    }

    pub fn with_token_state(mut self, require_token: bool, token_active: bool) -> Self {
        self.require_token = require_token;
        self.token_active = token_active;
        self
    }

    pub fn with_access(
        mut self,
        token: Option<String>,
        access_url: Option<String>,
        qr_svg: Option<String>,
    ) -> Self {
        self.token = token;
        self.access_url = access_url;
        self.qr_svg = qr_svg;
        self
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn address_is_hidden_while_access_is_off_and_inactive() {
        let dto: NetworkAccessDto =
            NetworkAccessDto::new(false, false, false, 3000, Some("192.168.1.5".to_string()));
        assert_eq!(dto.address, None);
    }

    #[test]
    fn address_is_shown_once_access_is_enabled_or_active() {
        let enabled: NetworkAccessDto =
            NetworkAccessDto::new(true, false, false, 3000, Some("192.168.1.5".to_string()));
        let active: NetworkAccessDto =
            NetworkAccessDto::new(false, true, true, 3000, Some("192.168.1.5".to_string()));
        assert_eq!(enabled.address.as_deref(), Some("192.168.1.5"));
        assert_eq!(active.address.as_deref(), Some("192.168.1.5"));
    }

    #[test]
    fn a_new_dto_carries_no_secret() {
        let dto: NetworkAccessDto = NetworkAccessDto::new(true, true, false, 3000, None);
        assert_eq!(dto.token, None);
        assert_eq!(dto.access_url, None);
        assert_eq!(dto.qr_svg, None);
    }
}
