use crate::modules::preferences::domain::errors::preferences_error::PreferencesError;

/// Lowest port a regular user may bind: below it the OS reserves ports for privileged services.
pub const MIN_LISTEN_PORT: u16 = 1024;

/// TCP port the server listens on, restricted to the unprivileged range.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct ListenPort(u16);

impl ListenPort {
    pub fn new(port: u16) -> Result<Self, PreferencesError> {
        if port < MIN_LISTEN_PORT {
            return Err(PreferencesError::InvalidPort(port));
        }
        Ok(Self(port))
    }

    /// Parses a stored value; anything that is not a valid port is rejected.
    pub fn parse(raw: &str) -> Result<Self, PreferencesError> {
        let port: u16 = raw
            .trim()
            .parse()
            .map_err(|_| PreferencesError::InvalidPort(0))?;
        Self::new(port)
    }

    pub fn get(&self) -> u16 {
        self.0
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn accepts_unprivileged_ports() {
        assert_eq!(ListenPort::new(1024).expect("min").get(), 1024);
        assert_eq!(ListenPort::new(9000).expect("9000").get(), 9000);
        assert_eq!(ListenPort::new(65535).expect("max").get(), 65535);
    }

    #[test]
    fn rejects_privileged_ports_and_zero() {
        assert_eq!(ListenPort::new(0), Err(PreferencesError::InvalidPort(0)));
        assert_eq!(ListenPort::new(80), Err(PreferencesError::InvalidPort(80)));
    }

    #[test]
    fn parse_rejects_garbage_and_out_of_range_values() {
        assert_eq!(ListenPort::parse(" 9000 ").expect("trimmed").get(), 9000);
        assert!(ListenPort::parse("abc").is_err());
        assert!(ListenPort::parse("70000").is_err());
        assert!(ListenPort::parse("").is_err());
    }
}
