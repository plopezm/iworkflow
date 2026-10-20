use leptos::prelude::*;
use serde::{Deserialize, Serialize};

use crate::core::browser;
use crate::core::i18n::Msg;
use crate::core::refresh::current_epoch;
use crate::modules::jira::infrastructure::adapters::inbound::dto::jira_dto::{
    IssueFilterDto, IssueLocalStateDto, JiraIssueListDto, JiraLaneDto, SavedLaneDto,
};
use crate::modules::jira::infrastructure::adapters::inbound::server_fns::{
    CreateJiraLane, DeleteJiraLane, SetJiraIssueIgnored, SetJiraLaneCollapsed, SyncJiraIssues,
    UpdateJiraLane, list_jira_issues,
};

/// What the lane form is doing: hidden, creating a lane or editing an existing one.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum LaneFormMode {
    Closed,
    Creating,
    Editing(String),
}

/// The form can be saved once it has a name and a query.
pub fn can_save_lane(name: &str, jql: &str) -> bool {
    !name.trim().is_empty() && !jql.trim().is_empty()
}

const LANE_SYNC_FAILED: Msg = Msg {
    en: "Lane “{lane}” was saved, but it could not be synced: {error}",
    es: "El carril «{lane}» se guardó, pero no se pudo sincronizar: {error}",
    zh: "泳道“{lane}”已保存，但无法同步：{error}",
    de: "Die Lane „{lane}“ wurde gespeichert, konnte aber nicht synchronisiert werden: {error}",
    fr: "La voie « {lane} » a été enregistrée, mais n'a pas pu être synchronisée : {error}",
};

/// Message for a lane that was stored but whose first sync failed.
pub fn lane_sync_notice(saved: &SavedLaneDto) -> Option<String> {
    saved.sync_error.as_ref().map(|error: &String| {
        LANE_SYNC_FAILED.fmt(&[("lane", saved.lane.name.as_str()), ("error", error)])
    })
}

/// `localStorage` key of the issue filters, so they survive switching tabs.
const FILTERS_STORAGE_KEY: &str = "iwf-jira-filters";

/// Raw filter form values as kept in `localStorage`.
#[derive(Debug, Default, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(default)]
pub struct StoredFilters {
    pub search: String,
    pub status: String,
    pub local_state: String,
}

impl StoredFilters {
    pub fn to_json(&self) -> String {
        serde_json::to_string(self).unwrap_or_default()
    }

    /// `None` when the stored text is not a valid filter document.
    pub fn from_json(json: &str) -> Option<Self> {
        serde_json::from_str(json).ok()
    }
}

/// Builds the server filter from the raw form values (blank = no filter).
pub fn build_filter(search: &str, status: &str, local_state: &str) -> IssueFilterDto {
    IssueFilterDto {
        search: search.trim().to_string(),
        status: Some(status.trim().to_string()).filter(|s: &String| !s.is_empty()),
        local_state: IssueLocalStateDto::parse(local_state),
    }
}

/// Estado del listado de tareas importadas: filtros, sincronización, descartar/restaurar y
/// gestión de carriles (alta, edición, borrado y plegado).
#[derive(Clone, Copy)]
pub struct JiraIssuesState {
    pub search: RwSignal<String>,
    pub status: RwSignal<String>,
    pub local_state: RwSignal<String>,
    pub issues: Resource<Result<JiraIssueListDto, ServerFnError>>,
    pub sync: ServerAction<SyncJiraIssues>,
    pub ignore: ServerAction<SetJiraIssueIgnored>,
    pub create_lane: ServerAction<CreateJiraLane>,
    pub update_lane: ServerAction<UpdateJiraLane>,
    pub delete_lane: ServerAction<DeleteJiraLane>,
    pub set_collapsed: ServerAction<SetJiraLaneCollapsed>,
    pub lane_form: RwSignal<LaneFormMode>,
    pub lane_name: RwSignal<String>,
    pub lane_jql: RwSignal<String>,
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
        let create_lane: ServerAction<CreateJiraLane> = ServerAction::<CreateJiraLane>::new();
        let update_lane: ServerAction<UpdateJiraLane> = ServerAction::<UpdateJiraLane>::new();
        let delete_lane: ServerAction<DeleteJiraLane> = ServerAction::<DeleteJiraLane>::new();
        let set_collapsed: ServerAction<SetJiraLaneCollapsed> =
            ServerAction::<SetJiraLaneCollapsed>::new();
        let lane_form: RwSignal<LaneFormMode> = RwSignal::new(LaneFormMode::Closed);
        let lane_name: RwSignal<String> = RwSignal::new(String::new());
        let lane_jql: RwSignal<String> = RwSignal::new(String::new());
        // Collapsing is persisted but deliberately not part of the key: the lane already
        // folded locally, reloading the list would only make it flicker.
        let issues: Resource<Result<JiraIssueListDto, ServerFnError>> = Resource::new(
            move || {
                (
                    search.get(),
                    status.get(),
                    local_state.get(),
                    (
                        sync.version().get(),
                        ignore.version().get(),
                        create_lane.version().get(),
                        update_lane.version().get(),
                        delete_lane.version().get(),
                    ),
                    connection_version.get(),
                    current_epoch(),
                )
            },
            |(search, status, local_state, ..)| {
                list_jira_issues(build_filter(&search, &status, &local_state))
            },
        );
        // Client only, after hydration: restore the saved filters and save every change.
        let filters_restored: RwSignal<bool> = RwSignal::new(false);
        Effect::new(move |_| {
            if let Some(saved) = browser::read_local_storage(FILTERS_STORAGE_KEY)
                .and_then(|json: String| StoredFilters::from_json(&json))
            {
                search.set(saved.search);
                status.set(saved.status);
                local_state.set(saved.local_state);
            }
            filters_restored.set(true);
        });
        Effect::new(move |_| {
            let current: StoredFilters = StoredFilters {
                search: search.get(),
                status: status.get(),
                local_state: local_state.get(),
            };
            if filters_restored.get() {
                browser::write_local_storage(FILTERS_STORAGE_KEY, &current.to_json());
            }
        });
        let statuses: RwSignal<Vec<String>> = RwSignal::new(Vec::new());
        Effect::new(move |_| {
            if let Some(Ok(list)) = issues.get()
                && statuses.with_untracked(|known: &Vec<String>| *known != list.statuses)
            {
                statuses.set(list.statuses);
            }
        });
        Effect::new(move |_| {
            if let Some(Ok(_)) = create_lane.value().get() {
                lane_form.set(LaneFormMode::Closed);
            }
        });
        Effect::new(move |_| {
            if let Some(Ok(_)) = update_lane.value().get() {
                lane_form.set(LaneFormMode::Closed);
            }
        });
        Self {
            search,
            status,
            local_state,
            issues,
            sync,
            ignore,
            create_lane,
            update_lane,
            delete_lane,
            set_collapsed,
            lane_form,
            lane_name,
            lane_jql,
            statuses,
        }
    }

    pub fn begin_create_lane(&self) {
        self.lane_name.set(String::new());
        self.lane_jql.set(String::new());
        self.lane_form.set(LaneFormMode::Creating);
    }

    pub fn begin_edit_lane(&self, lane: &JiraLaneDto) {
        self.lane_name.set(lane.name.clone());
        self.lane_jql.set(lane.jql.clone());
        self.lane_form.set(LaneFormMode::Editing(lane.id.clone()));
    }

    pub fn cancel_lane_form(&self) {
        self.lane_form.set(LaneFormMode::Closed);
    }

    pub fn save_lane(&self) {
        let name: String = self.lane_name.get_untracked();
        let jql: String = self.lane_jql.get_untracked();
        if !can_save_lane(&name, &jql) {
            return;
        }
        match self.lane_form.get_untracked() {
            LaneFormMode::Creating => {
                self.create_lane.dispatch(CreateJiraLane { name, jql });
            }
            LaneFormMode::Editing(id) => {
                self.update_lane.dispatch(UpdateJiraLane { id, name, jql });
            }
            LaneFormMode::Closed => {}
        }
    }

    pub fn remove_lane(&self, id: String) {
        self.delete_lane.dispatch(DeleteJiraLane { id });
    }

    pub fn toggle_lane(&self, id: String, collapsed: bool) {
        self.set_collapsed
            .dispatch(SetJiraLaneCollapsed { id, collapsed });
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
    fn stored_filters_round_trip() {
        let filters: StoredFilters = StoredFilters {
            search: "login".into(),
            status: "Done".into(),
            local_state: "ignored".into(),
        };
        assert_eq!(StoredFilters::from_json(&filters.to_json()), Some(filters));
    }

    #[test]
    fn stored_filters_tolerate_missing_fields_and_garbage() {
        assert_eq!(
            StoredFilters::from_json(r#"{"status":"Done"}"#).map(|f: StoredFilters| f.status),
            Some("Done".to_string())
        );
        assert_eq!(StoredFilters::from_json("not json"), None);
    }

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

    #[test]
    fn a_lane_needs_a_name_and_a_query() {
        assert!(can_save_lane("Backend", "project = X"));
        assert!(!can_save_lane("  ", "project = X"));
        assert!(!can_save_lane("Backend", " "));
    }

    #[test]
    fn notice_only_appears_when_the_first_sync_failed() {
        let lane: JiraLaneDto = JiraLaneDto {
            id: "l1".to_string(),
            name: "Backend".to_string(),
            jql: "project = X".to_string(),
            is_default: false,
            collapsed: false,
        };
        let ok: SavedLaneDto = SavedLaneDto {
            lane: lane.clone(),
            sync_error: None,
        };
        assert_eq!(lane_sync_notice(&ok), None);
        let failed: SavedLaneDto = SavedLaneDto {
            lane,
            sync_error: Some("JQL inválido".to_string()),
        };
        let notice: String = lane_sync_notice(&failed).expect("notice");
        assert!(notice.contains("Backend") && notice.contains("JQL inválido"));
    }
}
