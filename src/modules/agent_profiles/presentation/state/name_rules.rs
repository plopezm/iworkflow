use crate::core::i18n::Msg;

const EMPTY_NAME: Msg = Msg {
    en: "The name cannot be empty",
    es: "El nombre no puede estar vacío",
    zh: "名称不能为空",
    de: "Der Name darf nicht leer sein",
    fr: "Le nom ne peut pas être vide",
};

const NAME_TOO_LONG: Msg = Msg {
    en: "Maximum 64 characters",
    es: "Máximo 64 caracteres",
    zh: "最多 64 个字符",
    de: "Maximal 64 Zeichen",
    fr: "64 caractères maximum",
};

const BAD_START: Msg = Msg {
    en: "Must start with a letter or a digit",
    es: "Debe empezar por una letra o un dígito",
    zh: "必须以字母或数字开头",
    de: "Muss mit einem Buchstaben oder einer Ziffer beginnen",
    fr: "Doit commencer par une lettre ou un chiffre",
};

const BAD_CHARS: Msg = Msg {
    en: "Only letters, digits, “.”, “_” and “-”",
    es: "Solo letras, dígitos, «.», «_» y «-»",
    zh: "仅限字母、数字、“.”、“_”和“-”",
    de: "Nur Buchstaben, Ziffern, „.“, „_“ und „-“",
    fr: "Uniquement des lettres, chiffres, « . », « _ » et « - »",
};

const RESERVED_NAME: Msg = Msg {
    en: "Reserved name",
    es: "Nombre reservado",
    zh: "保留名称",
    de: "Reservierter Name",
    fr: "Nom réservé",
};

/// Longitud máxima de un nombre de perfil o de rol.
const MAX_NAME_LEN: usize = 64;

/// Nombres que el servidor rechaza por colisionar con propiedades de JavaScript.
const RESERVED_NAMES: [&str; 3] = ["__proto__", "constructor", "prototype"];

/// Pista de validación para un nombre slug (perfil o rol); `None` si es válido.
/// Refleja las reglas del servidor, que sigue siendo la autoridad.
pub fn slug_hint(raw: &str) -> Option<&'static str> {
    if raw.is_empty() {
        return Some(EMPTY_NAME.get());
    }
    if raw.len() > MAX_NAME_LEN {
        return Some(NAME_TOO_LONG.get());
    }
    if !raw.starts_with(|c: char| c.is_ascii_alphanumeric()) {
        return Some(BAD_START.get());
    }
    if !raw
        .chars()
        .all(|c: char| c.is_ascii_alphanumeric() || matches!(c, '.' | '_' | '-'))
    {
        return Some(BAD_CHARS.get());
    }
    if RESERVED_NAMES.contains(&raw) {
        return Some(RESERVED_NAME.get());
    }
    None
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::core::i18n::Locale;

    #[test]
    fn valid_names_have_no_hint() {
        assert_eq!(slug_hint("fast"), None);
        assert_eq!(slug_hint("gpt-5.1_test"), None);
        assert_eq!(slug_hint(&"a".repeat(64)), None);
    }

    #[test]
    fn empty_name_is_flagged() {
        assert_eq!(slug_hint(""), Some("The name cannot be empty"));
    }

    #[test]
    fn too_long_name_is_flagged() {
        assert_eq!(slug_hint(&"a".repeat(65)), Some("Maximum 64 characters"));
    }

    #[test]
    fn name_must_start_alphanumeric() {
        assert_eq!(
            slug_hint("-fast"),
            Some("Must start with a letter or a digit")
        );
    }

    #[test]
    fn invalid_characters_are_flagged() {
        assert_eq!(
            slug_hint("fast profile"),
            Some("Only letters, digits, “.”, “_” and “-”")
        );
        assert_eq!(
            slug_hint("clauдe"),
            Some("Only letters, digits, “.”, “_” and “-”")
        );
    }

    #[test]
    fn reserved_names_are_flagged() {
        assert_eq!(slug_hint("constructor"), Some("Reserved name"));
    }

    #[test]
    fn hints_are_available_in_spanish() {
        assert_eq!(
            EMPTY_NAME.in_locale(Locale::Es),
            "El nombre no puede estar vacío"
        );
        assert_eq!(RESERVED_NAME.in_locale(Locale::Es), "Nombre reservado");
    }
}
