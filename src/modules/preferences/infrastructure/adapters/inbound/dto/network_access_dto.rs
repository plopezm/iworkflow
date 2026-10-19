use serde::{Deserialize, Serialize};

/// One way to reach the server: an interface of this machine with its link and QR code.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct AccessTargetDto {
    /// Interface name, e.g. `eth0` or `tailscale0`.
    pub interface: String,
    pub address: String,
    /// `http://<address>:<port>`, with `/?token=...` appended when a token is demanded.
    pub access_url: String,
    /// Inline SVG of the QR code for `access_url`.
    pub qr_svg: Option<String>,
}

/// What the settings card needs to show and toggle network access.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct NetworkAccessDto {
    /// Saved choice: listen on every interface the next time the server starts.
    pub enabled: bool,
    /// The running server is reachable from other machines right now.
    pub active: bool,
    /// `LEPTOS_SITE_ADDR` pins the address, so the saved choice has no effect.
    pub locked_by_env: bool,
    /// Port the running server listens on.
    pub port: u16,
    /// Saved choice: port to listen on the next time the server starts.
    pub configured_port: u16,
    /// Interfaces other devices can use, preferred one first (empty if no network was detected
    /// or access is off).
    pub targets: Vec<AccessTargetDto>,
    /// Saved choice: demand the access token from network clients.
    pub require_token: bool,
    /// The running server demands the access token right now.
    pub token_active: bool,
    /// The secret itself, only while it is in play (reachable and required).
    pub token: Option<String>,
}

impl NetworkAccessDto {
    pub fn new(enabled: bool, active: bool, locked_by_env: bool, port: u16) -> Self {
        Self {
            enabled,
            active,
            locked_by_env,
            port,
            configured_port: port,
            targets: Vec::new(),
            require_token: true,
            token_active: false,
            token: None,
        }
    }

    pub fn with_token_state(mut self, require_token: bool, token_active: bool) -> Self {
        self.require_token = require_token;
        self.token_active = token_active;
        self
    }

    /// `targets` are only kept while access is on or active, so a private server shows no address.
    pub fn with_targets(mut self, targets: Vec<AccessTargetDto>) -> Self {
        if self.enabled || self.active {
            self.targets = targets;
        }
        self
    }

    pub fn with_configured_port(mut self, configured_port: u16) -> Self {
        self.configured_port = configured_port;
        self
    }

    pub fn with_token(mut self, token: Option<String>) -> Self {
        self.token = token;
        self
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn target(interface: &str, address: &str) -> AccessTargetDto {
        AccessTargetDto {
            interface: interface.to_string(),
            address: address.to_string(),
            access_url: format!("http://{address}:3000"),
            qr_svg: None,
        }
    }

    #[test]
    fn targets_are_hidden_while_access_is_off_and_inactive() {
        let dto: NetworkAccessDto = NetworkAccessDto::new(false, false, false, 3000)
            .with_targets(vec![target("eth0", "192.168.1.5")]);
        assert!(dto.targets.is_empty());
    }

    #[test]
    fn targets_are_shown_once_access_is_enabled_or_active() {
        let enabled: NetworkAccessDto = NetworkAccessDto::new(true, false, false, 3000)
            .with_targets(vec![target("eth0", "192.168.1.5")]);
        let active: NetworkAccessDto = NetworkAccessDto::new(false, true, true, 3000)
            .with_targets(vec![target("tailscale0", "100.64.0.7")]);
        assert_eq!(enabled.targets.len(), 1);
        assert_eq!(active.targets[0].interface, "tailscale0");
    }

    #[test]
    fn the_configured_port_defaults_to_the_running_one() {
        let dto: NetworkAccessDto = NetworkAccessDto::new(true, true, false, 9000);
        assert_eq!(dto.configured_port, 9000);
        assert_eq!(dto.with_configured_port(9100).configured_port, 9100);
    }

    #[test]
    fn a_new_dto_carries_no_secret() {
        let dto: NetworkAccessDto = NetworkAccessDto::new(true, true, false, 3000);
        assert_eq!(dto.token, None);
        assert!(dto.targets.is_empty());
    }
}
