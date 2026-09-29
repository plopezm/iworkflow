use chrono::{DateTime, Utc};

/// Diálogo abierto por el agente y aún sin responder.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct PendingDialog {
    pub id: String,
    /// Momento a partir del cual el agente lo da por caducado, si tenía `timeout_ms`.
    pub expires_at: Option<DateTime<Utc>>,
}

impl PendingDialog {
    pub fn new(id: String, expires_at: Option<DateTime<Utc>>) -> Self {
        Self { id, expires_at }
    }

    pub fn is_expired_at(&self, now: DateTime<Utc>) -> bool {
        self.expires_at
            .is_some_and(|expires_at: DateTime<Utc>| expires_at <= now)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn dialog_without_timeout_never_expires() {
        let dialog: PendingDialog = PendingDialog::new("d1".to_string(), None);
        assert!(!dialog.is_expired_at(Utc::now() + chrono::Duration::days(1)));
    }

    #[test]
    fn dialog_expires_at_or_after_its_deadline() {
        let now: DateTime<Utc> = Utc::now();
        let dialog: PendingDialog = PendingDialog::new("d1".to_string(), Some(now));
        assert!(!dialog.is_expired_at(now - chrono::Duration::milliseconds(1)));
        assert!(dialog.is_expired_at(now));
        assert!(dialog.is_expired_at(now + chrono::Duration::milliseconds(1)));
    }
}
