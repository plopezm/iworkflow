use leptos::prelude::*;

use crate::core::i18n::Msg;

use crate::modules::tasks::presentation::state::diff_files::{FileStatus, status_letter};

const ADDED: Msg = Msg {
    en: "Added",
    es: "Añadido",
    zh: "已添加",
    de: "Hinzugefügt",
    fr: "Ajouté",
};
const MODIFIED: Msg = Msg {
    en: "Modified",
    es: "Modificado",
    zh: "已修改",
    de: "Geändert",
    fr: "Modifié",
};
const DELETED: Msg = Msg {
    en: "Deleted",
    es: "Eliminado",
    zh: "已删除",
    de: "Gelöscht",
    fr: "Supprimé",
};
const RENAMED: Msg = Msg {
    en: "Renamed",
    es: "Renombrado",
    zh: "已重命名",
    de: "Umbenannt",
    fr: "Renommé",
};

/// CSS class of the status letter badge (A/M/D/R).
pub fn status_badge_class(status: FileStatus) -> &'static str {
    match status {
        FileStatus::Added => "diff-status diff-status-added",
        FileStatus::Modified => "diff-status diff-status-modified",
        FileStatus::Deleted => "diff-status diff-status-deleted",
        FileStatus::Renamed => "diff-status diff-status-renamed",
    }
}

/// Human readable name of a status, used as tooltip.
pub fn status_title(status: FileStatus) -> &'static str {
    match status {
        FileStatus::Added => ADDED.get(),
        FileStatus::Modified => MODIFIED.get(),
        FileStatus::Deleted => DELETED.get(),
        FileStatus::Renamed => RENAMED.get(),
    }
}

/// Single-letter badge with the status of a changed file.
#[component]
pub fn DiffStatusBadge(status: FileStatus) -> impl IntoView {
    view! {
        <span class=status_badge_class(status) title=status_title(status)>
            {status_letter(status)}
        </span>
    }
}

/// `+added -removed` counters.
#[component]
pub fn DiffCounts(additions: usize, deletions: usize) -> impl IntoView {
    view! {
        <span class="diff-counts">
            <span class="text-ok">{format!("+{additions}")}</span>
            <span class="text-error">{format!("-{deletions}")}</span>
        </span>
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::core::i18n::Locale;

    #[test]
    fn each_status_has_its_own_badge_class() {
        assert!(status_badge_class(FileStatus::Added).ends_with("added"));
        assert!(status_badge_class(FileStatus::Modified).ends_with("modified"));
        assert!(status_badge_class(FileStatus::Deleted).ends_with("deleted"));
        assert!(status_badge_class(FileStatus::Renamed).ends_with("renamed"));
    }

    #[test]
    fn status_titles_default_to_english() {
        assert_eq!(status_title(FileStatus::Added), "Added");
        assert_eq!(status_title(FileStatus::Deleted), "Deleted");
        assert_eq!(ADDED.in_locale(Locale::Es), "Añadido");
        assert_eq!(DELETED.in_locale(Locale::Es), "Eliminado");
    }
}
