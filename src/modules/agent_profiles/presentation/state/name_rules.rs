/// Longitud máxima de un nombre de perfil o de rol.
const MAX_NAME_LEN: usize = 64;

/// Nombres que el servidor rechaza por colisionar con propiedades de JavaScript.
const RESERVED_NAMES: [&str; 3] = ["__proto__", "constructor", "prototype"];

/// Pista de validación para un nombre slug (perfil o rol); `None` si es válido.
/// Refleja las reglas del servidor, que sigue siendo la autoridad.
pub fn slug_hint(raw: &str) -> Option<&'static str> {
    if raw.is_empty() {
        return Some("El nombre no puede estar vacío");
    }
    if raw.len() > MAX_NAME_LEN {
        return Some("Máximo 64 caracteres");
    }
    if !raw.starts_with(|c: char| c.is_ascii_alphanumeric()) {
        return Some("Debe empezar por una letra o un dígito");
    }
    if !raw
        .chars()
        .all(|c: char| c.is_ascii_alphanumeric() || matches!(c, '.' | '_' | '-'))
    {
        return Some("Solo letras, dígitos, «.», «_» y «-»");
    }
    if RESERVED_NAMES.contains(&raw) {
        return Some("Nombre reservado");
    }
    None
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn valid_names_have_no_hint() {
        assert_eq!(slug_hint("fast"), None);
        assert_eq!(slug_hint("gpt-5.1_test"), None);
        assert_eq!(slug_hint(&"a".repeat(64)), None);
    }

    #[test]
    fn empty_name_is_flagged() {
        assert_eq!(slug_hint(""), Some("El nombre no puede estar vacío"));
    }

    #[test]
    fn too_long_name_is_flagged() {
        assert_eq!(slug_hint(&"a".repeat(65)), Some("Máximo 64 caracteres"));
    }

    #[test]
    fn name_must_start_alphanumeric() {
        assert_eq!(
            slug_hint("-fast"),
            Some("Debe empezar por una letra o un dígito")
        );
    }

    #[test]
    fn invalid_characters_are_flagged() {
        assert_eq!(
            slug_hint("fast profile"),
            Some("Solo letras, dígitos, «.», «_» y «-»")
        );
        assert_eq!(
            slug_hint("clauдe"),
            Some("Solo letras, dígitos, «.», «_» y «-»")
        );
    }

    #[test]
    fn reserved_names_are_flagged() {
        assert_eq!(slug_hint("constructor"), Some("Nombre reservado"));
    }
}
