//! Codificación de rutas relativas (`a/b c/d.rs`) como segmentos de URL de pestaña.

/// Caracteres que no se codifican en un segmento (RFC 3986 «unreserved»).
fn is_unreserved(byte: u8) -> bool {
    byte.is_ascii_alphanumeric() || matches!(byte, b'-' | b'.' | b'_' | b'~')
}

/// Codifica cada segmento de `path` con porcentaje, conservando los `/` (pura).
pub fn encode_path(path: &str) -> String {
    let mut out: String = String::with_capacity(path.len());
    for byte in path.bytes() {
        if byte == b'/' || is_unreserved(byte) {
            out.push(byte as char);
        } else {
            out.push_str(&format!("%{byte:02X}"));
        }
    }
    out
}

/// Revierte [`encode_path`]; las secuencias `%` inválidas se dejan tal cual (pura).
pub fn decode_path(encoded: &str) -> String {
    let bytes: &[u8] = encoded.as_bytes();
    let mut out: Vec<u8> = Vec::with_capacity(bytes.len());
    let mut index: usize = 0;
    while index < bytes.len() {
        let decoded: Option<u8> = (bytes[index] == b'%')
            .then(|| bytes.get(index + 1..index + 3))
            .flatten()
            .and_then(|hex: &[u8]| std::str::from_utf8(hex).ok())
            .and_then(|hex: &str| u8::from_str_radix(hex, 16).ok());
        match decoded {
            Some(value) => {
                out.push(value);
                index += 3;
            }
            None => {
                out.push(bytes[index]);
                index += 1;
            }
        }
    }
    String::from_utf8_lossy(&out).into_owned()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn plain_paths_are_unchanged() {
        assert_eq!(encode_path("src/main.rs"), "src/main.rs");
    }

    #[test]
    fn special_characters_round_trip() {
        for path in ["a b/c#d?.md", "ñandú/é.txt", "100%/x", "a+b/[c].rs"] {
            assert_eq!(decode_path(&encode_path(path)), path);
        }
        assert_eq!(encode_path("a b#"), "a%20b%23");
    }

    #[test]
    fn invalid_escapes_are_kept() {
        assert_eq!(decode_path("100%"), "100%");
        assert_eq!(decode_path("%zz"), "%zz");
    }
}
