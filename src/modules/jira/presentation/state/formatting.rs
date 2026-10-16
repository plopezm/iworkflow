use crate::modules::jira::infrastructure::adapters::inbound::dto::jira_dto::{
    IssueLocalStateDto, SyncOutcomeDto,
};

/// `2024-05-01T10:00:00+00:00` → `2024-05-01 10:00` (UTC, as sent by the server).
pub fn short_timestamp(rfc3339: &str) -> String {
    rfc3339
        .chars()
        .take(16)
        .collect::<String>()
        .replace('T', " ")
}

pub fn sync_message(outcome: &SyncOutcomeDto) -> String {
    match (outcome.total, outcome.added) {
        (0, _) => "Sincronizado: Jira no devolvió ninguna tarea.".to_string(),
        (total, 0) => format!("Sincronizado: {total} tareas (sin novedades)."),
        (total, added) => format!("Sincronizado: {total} tareas ({added} nuevas)."),
    }
}

/// Badge variant (see `Badge`) of a local state.
pub fn local_state_variant(state: IssueLocalStateDto) -> &'static str {
    match state {
        IssueLocalStateDto::New => "info",
        IssueLocalStateDto::Converted => "ok",
        IssueLocalStateDto::Ignored => "muted",
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn short_timestamp_keeps_date_and_minutes() {
        assert_eq!(
            short_timestamp("2024-05-01T10:00:00+00:00"),
            "2024-05-01 10:00"
        );
        assert_eq!(short_timestamp("hoy"), "hoy");
    }

    #[test]
    fn sync_message_covers_empty_unchanged_and_new() {
        let msg = |total, added| sync_message(&SyncOutcomeDto { total, added });
        assert_eq!(msg(0, 0), "Sincronizado: Jira no devolvió ninguna tarea.");
        assert_eq!(msg(4, 0), "Sincronizado: 4 tareas (sin novedades).");
        assert_eq!(msg(4, 2), "Sincronizado: 4 tareas (2 nuevas).");
    }

    #[test]
    fn every_local_state_has_a_variant() {
        assert_eq!(local_state_variant(IssueLocalStateDto::New), "info");
        assert_eq!(local_state_variant(IssueLocalStateDto::Converted), "ok");
        assert_eq!(local_state_variant(IssueLocalStateDto::Ignored), "muted");
    }
}
