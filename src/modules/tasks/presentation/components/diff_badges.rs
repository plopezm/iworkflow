use leptos::prelude::*;

use crate::modules::tasks::presentation::state::diff_files::{FileStatus, status_letter};

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
        FileStatus::Added => "Añadido",
        FileStatus::Modified => "Modificado",
        FileStatus::Deleted => "Eliminado",
        FileStatus::Renamed => "Renombrado",
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

    #[test]
    fn each_status_has_its_own_badge_class() {
        assert!(status_badge_class(FileStatus::Added).ends_with("added"));
        assert!(status_badge_class(FileStatus::Modified).ends_with("modified"));
        assert!(status_badge_class(FileStatus::Deleted).ends_with("deleted"));
        assert!(status_badge_class(FileStatus::Renamed).ends_with("renamed"));
    }

    #[test]
    fn status_titles_are_spanish() {
        assert_eq!(status_title(FileStatus::Added), "Añadido");
        assert_eq!(status_title(FileStatus::Deleted), "Eliminado");
    }
}
