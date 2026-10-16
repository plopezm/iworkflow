use leptos::prelude::*;

use crate::core::refresh::current_epoch;
use crate::modules::jira::infrastructure::adapters::inbound::dto::jira_dto::{
    IssueFilterDto, IssueLocalStateDto, JiraIssueListDto,
};
use crate::modules::jira::infrastructure::adapters::inbound::server_fns::{
    SetJiraIssueIgnored, SyncJiraIssues, list_jira_issues,
};

/// Builds the server filter from the raw form values (blank = no filter).
pub fn build_filter(search: &str, status: &str, local_state: &str) -> IssueFilterDto {
    IssueFilterDto {
        search: search.trim().to_string(),
        status: Some(status.trim().to_string()).filter(|s: &String| !s.is_empty()),
        local_state: IssueLocalStateDto::parse(local_state),
    }
}

/// Estado del listado de tareas importadas: filtros, sincronización y descartar/restaurar.
#[derive(Clone, Copy)]
pub struct JiraIssuesState {
    pub search: RwSignal<String>,
    pub status: RwSignal<String>,
    pub local_state: RwSignal<String>,
    pub issues: Resource<Result<JiraIssueListDto, ServerFnError>>,
    pub sync: ServerAction<SyncJiraIssues>,
    pub ignore: ServerAction<SetJiraIssueIgnored>,
    /// Statuses seen so far, kept outside the list so the filter bar does not rebuild.
    pub statuses: RwSignal<Vec<String>>,
}

impl JiraIssuesState {
    /// `connection_version` changes when the connection is saved or removed.
    pub fn new(connection_version: Signal<usize>) -> Self {
        let search: RwSignal<String> = RwSignal::new(String::new());
        let status: RwSignal<String> = RwSignal::new(String::new());
        let local_state: RwSignal<String> = RwSignal::new(String::new());
        let sync: ServerAction<SyncJiraIssues> = ServerAction::<SyncJiraIssues>::new();
        let ignore: ServerAction<SetJiraIssueIgnored> = ServerAction::<SetJiraIssueIgnored>::new();
        let issues: Resource<Result<JiraIssueListDto, ServerFnError>> = Resource::new(
            move || {
                (
                    search.get(),
                    status.get(),
                    local_state.get(),
                    sync.version().get(),
                    ignore.version().get(),
                    connection_version.get(),
                    current_epoch(),
                )
            },
            |(search, status, local_state, ..)| {
                list_jira_issues(build_filter(&search, &status, &local_state))
            },
        );
        let statuses: RwSignal<Vec<String>> = RwSignal::new(Vec::new());
        Effect::new(move |_| {
            if let Some(Ok(list)) = issues.get()
                && statuses.with_untracked(|known: &Vec<String>| *known != list.statuses)
            {
                statuses.set(list.statuses);
            }
        });
        Self {
            search,
            status,
            local_state,
            issues,
            sync,
            ignore,
            statuses,
        }
    }

    pub fn start_sync(&self) {
        self.sync.dispatch(SyncJiraIssues {});
    }

    pub fn set_ignored(&self, key: String, ignored: bool) {
        self.ignore.dispatch(SetJiraIssueIgnored { key, ignored });
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn blank_values_mean_no_filter() {
        assert_eq!(build_filter("  ", "", ""), IssueFilterDto::default());
    }

    #[test]
    fn values_are_trimmed_and_local_state_parsed() {
        let filter: IssueFilterDto = build_filter(" login ", " Done ", "ignored");
        assert_eq!(filter.search, "login");
        assert_eq!(filter.status.as_deref(), Some("Done"));
        assert_eq!(filter.local_state, Some(IssueLocalStateDto::Ignored));
    }

    #[test]
    fn unknown_local_state_is_ignored() {
        assert_eq!(build_filter("", "", "bogus").local_state, None);
    }
}
