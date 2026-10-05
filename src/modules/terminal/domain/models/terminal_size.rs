use crate::modules::terminal::domain::errors::terminal_error::TerminalError;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct TerminalSize {
    pub cols: u16,
    pub rows: u16,
}

impl TerminalSize {
    pub fn new(cols: u16, rows: u16) -> Result<Self, TerminalError> {
        if cols == 0 || rows == 0 {
            return Err(TerminalError::InvalidSize);
        }
        Ok(Self { cols, rows })
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn rejects_zero_dimensions() {
        assert_eq!(TerminalSize::new(0, 24), Err(TerminalError::InvalidSize));
        assert_eq!(TerminalSize::new(80, 0), Err(TerminalError::InvalidSize));
    }

    #[test]
    fn accepts_positive_dimensions() {
        assert_eq!(
            TerminalSize::new(80, 24),
            Ok(TerminalSize { cols: 80, rows: 24 })
        );
    }
}
