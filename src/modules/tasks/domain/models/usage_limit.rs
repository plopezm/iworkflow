use chrono::{DateTime, Duration, TimeZone, Utc};

/// Espera inicial entre intentos cuando el proveedor no indica cuándo se renueva el límite.
const PROBE_BASE_SECS: i64 = 5 * 60;
/// Tope de la espera entre intentos (el backoff se duplica hasta aquí).
const PROBE_MAX_SECS: i64 = 60 * 60;
/// Margen tras el instante de renovación que indica el proveedor, para no reanudar justo antes.
const RESET_MARGIN_SECS: i64 = 60;
/// Exponente máximo del backoff (2^6 · 5 min ya supera el tope).
const MAX_BACKOFF_EXPONENT: u32 = 6;

/// Fragmentos (en minúsculas) de los mensajes de error con los que los proveedores
/// avisan de que se agotó la cuota, el crédito o la ventana de la suscripción.
const LIMIT_MARKERS: [&str; 14] = [
    "usage limit",
    "out of extra usage",
    "rate limit",
    "rate_limit",
    "too many requests",
    "insufficient_quota",
    "exceeded your current quota",
    "credit balance is too low",
    "upgrade your plan",
    "resource_exhausted",
    // Límites duros que pi no reintenta.
    "available balance",
    "quota exceeded",
    "out of budget",
    "subscription_sharing_usage_limit",
];

/// Indica si `text` (un mensaje de error del agente) describe un límite de uso agotado:
/// «You've hit your session limit · resets 3:45pm», «usage limit reached», un 429, etc.
pub fn looks_like_usage_limit(text: &str) -> bool {
    let lowered: String = text.to_lowercase();
    (lowered.contains("hit your") && lowered.contains("limit"))
        || LIMIT_MARKERS
            .iter()
            .any(|marker: &&str| lowered.contains(marker))
}

/// Instante (epoch en segundos) → fecha; `None` si está fuera de rango.
pub fn from_epoch_secs(secs: i64) -> Option<DateTime<Utc>> {
    Utc.timestamp_opt(secs, 0).single()
}

/// Planificación de la reanudación de una tarea parada por un límite de uso.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub struct UsageLimitState {
    /// Cuándo reintentar la reanudación; `None` si la tarea no está parada por un límite.
    pub resume_at: Option<DateTime<Utc>>,
    /// Veces seguidas que se ha topado con el límite sin completar un turno (alimenta el backoff).
    pub attempts: u32,
}

impl UsageLimitState {
    /// Estado tras un nuevo choque con el límite: usa `resets_at` si el proveedor lo indicó
    /// y es futuro; si no, sondea con backoff exponencial (5 min → 1 h).
    pub fn after_hit(&self, resets_at: Option<DateTime<Utc>>, now: DateTime<Utc>) -> Self {
        let resume_at: DateTime<Utc> = match resets_at.filter(|at: &DateTime<Utc>| *at > now) {
            Some(at) => at + Duration::seconds(RESET_MARGIN_SECS),
            None => {
                let exponent: u32 = self.attempts.min(MAX_BACKOFF_EXPONENT);
                let secs: i64 = (PROBE_BASE_SECS << exponent).min(PROBE_MAX_SECS);
                now + Duration::seconds(secs)
            }
        };
        Self {
            resume_at: Some(resume_at),
            attempts: self.attempts.saturating_add(1),
        }
    }

    /// Ha llegado la hora de reintentar.
    pub fn is_due(&self, now: DateTime<Utc>) -> bool {
        self.resume_at.is_some_and(|at: DateTime<Utc>| at <= now)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn now() -> DateTime<Utc> {
        Utc.with_ymd_and_hms(2026, 10, 18, 12, 0, 0)
            .single()
            .expect("fecha")
    }

    #[test]
    fn detects_provider_limit_messages() {
        for text in [
            "You've hit your session limit · resets 3:45pm",
            "You've hit your weekly limit · resets Mon 12:00am",
            "claude -p: You're out of extra usage · resets 6:40pm",
            "Claude AI usage limit reached|1784283600",
            r#"429 {"error":{"type":"rate_limit_error"}}"#,
            "You exceeded your current quota",
            "Error: quota exceeded for this account",
            "your available balance is too low",
        ] {
            assert!(looks_like_usage_limit(text), "{text}");
        }
    }

    #[test]
    fn ignores_unrelated_errors() {
        for text in ["error_max_turns", "boom", "command not found", "limit"] {
            assert!(!looks_like_usage_limit(text), "{text}");
        }
    }

    #[test]
    fn known_future_reset_is_used_with_margin() {
        let reset: DateTime<Utc> = now() + Duration::hours(3);
        let state: UsageLimitState = UsageLimitState::default().after_hit(Some(reset), now());
        assert_eq!(state.resume_at, Some(reset + Duration::seconds(60)));
        assert_eq!(state.attempts, 1);
    }

    #[test]
    fn unknown_or_past_reset_backs_off_exponentially_up_to_the_cap() {
        let mut state: UsageLimitState = UsageLimitState::default();
        let mut waits: Vec<i64> = Vec::new();
        for _ in 0..6 {
            state = state.after_hit(None, now());
            waits.push((state.resume_at.expect("programada") - now()).num_minutes());
        }
        assert_eq!(waits, vec![5, 10, 20, 40, 60, 60]);
        let past: UsageLimitState =
            UsageLimitState::default().after_hit(Some(now() - Duration::minutes(1)), now());
        assert_eq!(past.resume_at, Some(now() + Duration::minutes(5)));
    }

    #[test]
    fn is_due_only_once_the_resume_time_has_passed() {
        let state: UsageLimitState = UsageLimitState::default().after_hit(None, now());
        assert!(!state.is_due(now()));
        assert!(state.is_due(now() + Duration::minutes(5)));
        assert!(!UsageLimitState::default().is_due(now()));
    }

    #[test]
    fn epoch_seconds_convert_to_dates() {
        assert_eq!(from_epoch_secs(0), Some(Utc.timestamp_opt(0, 0).unwrap()));
    }
}
