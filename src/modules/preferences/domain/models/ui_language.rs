use crate::modules::preferences::domain::errors::preferences_error::PreferencesError;

/// Language codes the interface is translated into (mirrors `core::i18n::Locale`).
pub const SUPPORTED_LANGUAGES: [&str; 5] = ["en", "es", "zh", "de", "fr"];

/// Interface language the user picked; only supported codes can be built.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct UiLanguage(&'static str);

impl UiLanguage {
    pub fn parse(raw: &str) -> Result<Self, PreferencesError> {
        let code: &str = raw.trim();
        SUPPORTED_LANGUAGES
            .into_iter()
            .find(|supported: &&str| *supported == code)
            .map(UiLanguage)
            .ok_or_else(|| PreferencesError::UnsupportedLanguage(code.to_string()))
    }

    pub fn as_str(&self) -> &'static str {
        self.0
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::core::i18n::Locale;

    #[test]
    fn parses_every_supported_code_and_trims() {
        for code in SUPPORTED_LANGUAGES {
            assert_eq!(UiLanguage::parse(code).expect("parse").as_str(), code);
        }
        assert_eq!(UiLanguage::parse(" de ").expect("parse").as_str(), "de");
    }

    #[test]
    fn rejects_unknown_codes() {
        assert_eq!(
            UiLanguage::parse("pt"),
            Err(PreferencesError::UnsupportedLanguage("pt".to_string()))
        );
        assert!(UiLanguage::parse("").is_err());
    }

    #[test]
    fn supported_languages_match_the_interface_locales() {
        let locales: Vec<&str> = Locale::ALL.into_iter().map(Locale::code).collect();
        assert_eq!(locales, SUPPORTED_LANGUAGES);
    }
}
