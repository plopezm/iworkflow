use serde::Deserialize;

/// Mensaje cliente → servidor del WebSocket de terminal.
///
/// Un frame binario son bytes de entrada; un frame de texto es JSON
/// `{"type":"resize","cols":N,"rows":M}`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ClientMessage {
    Input(Vec<u8>),
    Resize { cols: u16, rows: u16 },
}

#[derive(Deserialize)]
#[serde(tag = "type", rename_all = "lowercase")]
enum ControlWire {
    Resize { cols: u16, rows: u16 },
}

impl ClientMessage {
    pub fn from_binary(bytes: Vec<u8>) -> Self {
        Self::Input(bytes)
    }

    pub fn from_text(text: &str) -> Result<Self, serde_json::Error> {
        match serde_json::from_str::<ControlWire>(text)? {
            ControlWire::Resize { cols, rows } => Ok(Self::Resize { cols, rows }),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn binary_frame_is_raw_input() {
        assert_eq!(
            ClientMessage::from_binary(vec![1, 2, 3]),
            ClientMessage::Input(vec![1, 2, 3])
        );
    }

    #[test]
    fn text_frame_parses_resize() {
        assert_eq!(
            ClientMessage::from_text(r#"{"type":"resize","cols":100,"rows":30}"#).unwrap(),
            ClientMessage::Resize {
                cols: 100,
                rows: 30
            }
        );
    }

    #[test]
    fn text_frame_rejects_unknown_type_garbage_and_overflow() {
        assert!(ClientMessage::from_text(r#"{"type":"nope"}"#).is_err());
        assert!(ClientMessage::from_text("hola").is_err());
        assert!(ClientMessage::from_text(r#"{"type":"resize","cols":99999,"rows":1}"#).is_err());
        assert!(ClientMessage::from_text(r#"{"type":"resize","cols":1}"#).is_err());
    }
}
