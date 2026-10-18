use crate::core::i18n::Msg;
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

const SYNC_EMPTY: Msg = Msg {
    en: "Synced: Jira returned no issues.",
    es: "Sincronizado: Jira no devolvió ninguna tarea.",
    zh: "已同步：Jira 没有返回任何任务。",
    de: "Synchronisiert: Jira hat keine Aufgaben zurückgegeben.",
    fr: "Synchronisé : Jira n'a renvoyé aucune tâche.",
};
const SYNC_UNCHANGED: Msg = Msg {
    en: "Synced: {total} issues (nothing new).",
    es: "Sincronizado: {total} tareas (sin novedades).",
    zh: "已同步：{total} 个任务（无新增）。",
    de: "Synchronisiert: {total} Aufgaben (keine Neuigkeiten).",
    fr: "Synchronisé : {total} tâches (rien de nouveau).",
};
const SYNC_WITH_NEW: Msg = Msg {
    en: "Synced: {total} issues ({added} new).",
    es: "Sincronizado: {total} tareas ({added} nuevas).",
    zh: "已同步：{total} 个任务（{added} 个新增）。",
    de: "Synchronisiert: {total} Aufgaben ({added} neu).",
    fr: "Synchronisé : {total} tâches ({added} nouvelles).",
};
const SYNC_ONE_LANE_FAILED: Msg = Msg {
    en: "{base} One lane failed.",
    es: "{base} Un carril falló.",
    zh: "{base} 有 1 个泳道失败。",
    de: "{base} Eine Lane ist fehlgeschlagen.",
    fr: "{base} Une voie a échoué.",
};
const SYNC_LANES_FAILED: Msg = Msg {
    en: "{base} {n} lanes failed.",
    es: "{base} {n} carriles fallaron.",
    zh: "{base} 有 {n} 个泳道失败。",
    de: "{base} {n} Lanes sind fehlgeschlagen.",
    fr: "{base} {n} voies ont échoué.",
};

const STATE_NEW: Msg = Msg {
    en: "New",
    es: "Nueva",
    zh: "新建",
    de: "Neu",
    fr: "Nouvelle",
};
const STATE_CONVERTED: Msg = Msg {
    en: "Converted",
    es: "Convertida",
    zh: "已转换",
    de: "Konvertiert",
    fr: "Convertie",
};
const STATE_IGNORED: Msg = Msg {
    en: "Ignored",
    es: "Ignorada",
    zh: "已忽略",
    de: "Ignoriert",
    fr: "Ignorée",
};

/// Visible name of a local state, in the current language.
pub fn local_state_label(state: IssueLocalStateDto) -> &'static str {
    match state {
        IssueLocalStateDto::New => STATE_NEW.get(),
        IssueLocalStateDto::Converted => STATE_CONVERTED.get(),
        IssueLocalStateDto::Ignored => STATE_IGNORED.get(),
    }
}

pub fn sync_message(outcome: &SyncOutcomeDto) -> String {
    let base: String = match (outcome.total, outcome.added) {
        (0, _) => SYNC_EMPTY.get().to_string(),
        (total, 0) => SYNC_UNCHANGED.fmt(&[("total", &total.to_string())]),
        (total, added) => {
            SYNC_WITH_NEW.fmt(&[("total", &total.to_string()), ("added", &added.to_string())])
        }
    };
    match outcome.failed_lanes.len() {
        0 => base,
        1 => SYNC_ONE_LANE_FAILED.fmt(&[("base", &base)]),
        n => SYNC_LANES_FAILED.fmt(&[("base", &base), ("n", &n.to_string())]),
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
    use crate::core::i18n::Locale;

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
        let msg = |total, added| {
            sync_message(&SyncOutcomeDto {
                total,
                added,
                failed_lanes: vec![],
            })
        };
        assert_eq!(msg(0, 0), "Synced: Jira returned no issues.");
        assert_eq!(msg(4, 0), "Synced: 4 issues (nothing new).");
        assert_eq!(msg(4, 2), "Synced: 4 issues (2 new).");
    }

    #[test]
    fn sync_message_is_available_in_spanish() {
        assert_eq!(
            SYNC_WITH_NEW.in_locale(Locale::Es),
            "Sincronizado: {total} tareas ({added} nuevas)."
        );
        assert_eq!(STATE_NEW.in_locale(Locale::Es), "Nueva");
    }

    #[test]
    fn sync_message_mentions_failed_lanes() {
        let msg = |failed: usize| {
            sync_message(&SyncOutcomeDto {
                total: 2,
                added: 0,
                failed_lanes: vec!["x".to_string(); failed],
            })
        };
        assert!(msg(1).ends_with("One lane failed."));
        assert!(msg(3).ends_with("3 lanes failed."));
    }

    #[test]
    fn local_state_labels_default_to_english() {
        assert_eq!(local_state_label(IssueLocalStateDto::New), "New");
        assert_eq!(
            local_state_label(IssueLocalStateDto::Converted),
            "Converted"
        );
        assert_eq!(local_state_label(IssueLocalStateDto::Ignored), "Ignored");
    }

    #[test]
    fn every_local_state_has_a_variant() {
        assert_eq!(local_state_variant(IssueLocalStateDto::New), "info");
        assert_eq!(local_state_variant(IssueLocalStateDto::Converted), "ok");
        assert_eq!(local_state_variant(IssueLocalStateDto::Ignored), "muted");
    }
}
