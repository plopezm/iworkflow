use leptos::prelude::*;

use crate::core::refresh::current_epoch;
use crate::modules::jira::infrastructure::adapters::inbound::dto::jira_dto::{
    JiraCommentDto, JiraIssueDto, JiraTransitionDto,
};
use crate::modules::jira::infrastructure::adapters::inbound::server_fns::{
    AddJiraComment, SetJiraIssueIgnored, TransitionJiraIssue, get_jira_issue, list_jira_comments,
    list_jira_transitions,
};

/// Estado del detalle de una tarea de Jira: datos locales, comentarios y transiciones de Jira.
#[derive(Clone, Copy)]
pub struct JiraIssueState {
    pub key: StoredValue<String>,
    pub issue: Resource<Result<JiraIssueDto, ServerFnError>>,
    /// Read live from Jira: failing here must not hide the imported issue.
    pub comments: Resource<Result<Vec<JiraCommentDto>, ServerFnError>>,
    pub transitions: Resource<Result<Vec<JiraTransitionDto>, ServerFnError>>,
    pub add_comment: ServerAction<AddJiraComment>,
    pub transition: ServerAction<TransitionJiraIssue>,
    pub ignore: ServerAction<SetJiraIssueIgnored>,
    pub comment_text: RwSignal<String>,
    pub selected_transition: RwSignal<String>,
}

impl JiraIssueState {
    pub fn new(key: String) -> Self {
        let add_comment: ServerAction<AddJiraComment> = ServerAction::<AddJiraComment>::new();
        let transition: ServerAction<TransitionJiraIssue> =
            ServerAction::<TransitionJiraIssue>::new();
        let ignore: ServerAction<SetJiraIssueIgnored> = ServerAction::<SetJiraIssueIgnored>::new();
        let stored_key: StoredValue<String> = StoredValue::new(key);
        let issue: Resource<Result<JiraIssueDto, ServerFnError>> = Resource::new(
            move || {
                (
                    transition.version().get(),
                    ignore.version().get(),
                    current_epoch(),
                )
            },
            move |_| get_jira_issue(stored_key.get_value()),
        );
        let comments: Resource<Result<Vec<JiraCommentDto>, ServerFnError>> = Resource::new(
            move || add_comment.version().get(),
            move |_| list_jira_comments(stored_key.get_value()),
        );
        let transitions: Resource<Result<Vec<JiraTransitionDto>, ServerFnError>> = Resource::new(
            move || transition.version().get(),
            move |_| list_jira_transitions(stored_key.get_value()),
        );
        let state: Self = Self {
            key: stored_key,
            issue,
            comments,
            transitions,
            add_comment,
            transition,
            ignore,
            comment_text: RwSignal::new(String::new()),
            selected_transition: RwSignal::new(String::new()),
        };
        Effect::new(move |_| {
            if let Some(Ok(())) = add_comment.value().get() {
                state.comment_text.set(String::new());
            }
        });
        Effect::new(move |_| {
            if let Some(Ok(_)) = transition.value().get() {
                state.selected_transition.set(String::new());
            }
        });
        state
    }

    pub fn send_comment(&self) {
        self.add_comment.dispatch(AddJiraComment {
            key: self.key.get_value(),
            body: self.comment_text.get_untracked(),
        });
    }

    pub fn apply_transition(&self) {
        let transition_id: String = self.selected_transition.get_untracked();
        if transition_id.is_empty() {
            return;
        }
        self.transition.dispatch(TransitionJiraIssue {
            key: self.key.get_value(),
            transition_id,
        });
    }

    pub fn set_ignored(&self, ignored: bool) {
        self.ignore.dispatch(SetJiraIssueIgnored {
            key: self.key.get_value(),
            ignored,
        });
    }
}
