use uuid::Uuid;

use crate::modules::preferences::domain::errors::preferences_error::PreferencesError;

/// Length of a token: two UUIDv4 values written as hex without dashes (64 hex characters).
const TOKEN_LEN: usize = 64;

/// Secret that other devices present to use the app when it is reachable from the network.
#[derive(Clone, PartialEq, Eq)]
pub struct AccessToken(String);

// Never print the secret by accident (logs, panics, test failures).
impl std::fmt::Debug for AccessToken {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str("AccessToken(…)")
    }
}

impl AccessToken {
    /// About 244 random bits (two UUIDv4 values) taken from the operating system.
    pub fn generate() -> Self {
        Self(format!(
            "{}{}",
            Uuid::new_v4().simple(),
            Uuid::new_v4().simple()
        ))
    }

    /// Accepts only well-formed tokens, so a damaged stored value is treated as missing.
    pub fn parse(raw: &str) -> Result<Self, PreferencesError> {
        let valid: bool = raw.len() == TOKEN_LEN && raw.bytes().all(|b: u8| b.is_ascii_hexdigit());
        if valid {
            Ok(Self(raw.to_ascii_lowercase()))
        } else {
            Err(PreferencesError::InvalidAccessToken)
        }
    }

    pub fn as_str(&self) -> &str {
        &self.0
    }

    /// Compares without stopping at the first differing byte, so timing leaks nothing.
    pub fn matches(&self, candidate: &str) -> bool {
        let expected: &[u8] = self.0.as_bytes();
        let given: &[u8] = candidate.as_bytes();
        let same_length: bool = expected.len() == given.len();
        let differences: u8 = expected
            .iter()
            .zip(given.iter())
            .fold(0u8, |acc: u8, (a, b): (&u8, &u8)| acc | (a ^ b));
        same_length && differences == 0
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn generated_tokens_are_well_formed_and_unique() {
        let first: AccessToken = AccessToken::generate();
        let second: AccessToken = AccessToken::generate();
        assert_eq!(first.as_str().len(), TOKEN_LEN);
        assert!(AccessToken::parse(first.as_str()).is_ok());
        assert_ne!(first, second);
    }

    #[test]
    fn parse_rejects_malformed_values() {
        assert_eq!(
            AccessToken::parse("abc"),
            Err(PreferencesError::InvalidAccessToken)
        );
        let non_hex: String = "z".repeat(TOKEN_LEN);
        assert_eq!(
            AccessToken::parse(&non_hex),
            Err(PreferencesError::InvalidAccessToken)
        );
    }

    #[test]
    fn matches_only_the_exact_token() {
        let token: AccessToken = AccessToken::generate();
        assert!(token.matches(token.as_str()));
        assert!(!token.matches(""));
        assert!(!token.matches(&token.as_str()[1..]));
        let other: AccessToken = AccessToken::generate();
        assert!(!token.matches(other.as_str()));
    }

    #[test]
    fn debug_output_hides_the_secret() {
        let token: AccessToken = AccessToken::generate();
        assert!(!format!("{token:?}").contains(token.as_str()));
    }
}
