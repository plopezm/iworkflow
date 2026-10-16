use leptos::prelude::*;

use crate::modules::jira::infrastructure::adapters::inbound::dto::jira_dto::{
    ConversionBranchDto, ConversionPreparationDto, ConvertIssueDto, ConvertedIssueDto,
};
use crate::modules::jira::infrastructure::adapters::inbound::server_fns::{
    ConvertJiraIssue, list_jira_conversion_branches, prepare_jira_conversion,
};

/// Builds the request from the form values (blank branch = workspace default).
pub fn build_convert_dto(
    key: &str,
    workspace_id: &str,
    base_branch: &str,
    title: &str,
    prompt: &str,
    priority: &str,
    start_now: bool,
) -> ConvertIssueDto {
    ConvertIssueDto {
        key: key.to_string(),
        workspace_id: workspace_id.to_string(),
        base_branch: Some(base_branch.trim().to_string()).filter(|b: &String| !b.is_empty()),
        title: title.to_string(),
        prompt: prompt.to_string(),
        priority: priority.to_string(),
        start_now,
    }
}

/// Estado de la pantalla de conversión: borrador editable y envío.
#[derive(Clone, Copy)]
pub struct JiraConvertState {
    pub key: StoredValue<String>,
    pub preparation: Resource<Result<ConversionPreparationDto, ServerFnError>>,
    pub branches: Resource<Result<Vec<ConversionBranchDto>, ServerFnError>>,
    pub convert: ServerAction<ConvertJiraIssue>,
    pub form_workspace_id: RwSignal<String>,
    pub form_base_branch: RwSignal<String>,
    pub form_title: RwSignal<String>,
    pub form_prompt: RwSignal<String>,
    pub form_priority: RwSignal<String>,
    pub form_start_now: RwSignal<bool>,
    prefilled: RwSignal<bool>,
}

impl JiraConvertState {
    pub fn new(key: String) -> Self {
        let stored_key: StoredValue<String> = StoredValue::new(key);
        let form_workspace_id: RwSignal<String> = RwSignal::new(String::new());
        let preparation: Resource<Result<ConversionPreparationDto, ServerFnError>> = Resource::new(
            || (),
            move |_| prepare_jira_conversion(stored_key.get_value()),
        );
        let branches: Resource<Result<Vec<ConversionBranchDto>, ServerFnError>> = Resource::new(
            move || form_workspace_id.get(),
            list_jira_conversion_branches,
        );
        let state: Self = Self {
            key: stored_key,
            preparation,
            branches,
            convert: ServerAction::<ConvertJiraIssue>::new(),
            form_workspace_id,
            form_base_branch: RwSignal::new(String::new()),
            form_title: RwSignal::new(String::new()),
            form_prompt: RwSignal::new(String::new()),
            form_priority: RwSignal::new("normal".to_string()),
            form_start_now: RwSignal::new(false),
            prefilled: RwSignal::new(false),
        };
        Effect::new(move |_| {
            if let Some(Ok(prepared)) = preparation.get()
                && !state.prefilled.get_untracked()
            {
                state.prefill(&prepared);
                state.prefilled.set(true);
            }
        });
        // A branch from another workspace would be invalid for the new one.
        Effect::new(move |previous: Option<String>| {
            let current: String = form_workspace_id.get();
            if previous.is_some_and(|p: String| p != current) {
                state.form_base_branch.set(String::new());
            }
            current
        });
        state
    }

    /// Loads the suggested content once; a single workspace is preselected.
    pub fn prefill(&self, prepared: &ConversionPreparationDto) {
        self.form_title.set(prepared.title.clone());
        self.form_prompt.set(prepared.prompt.clone());
        self.form_priority.set(prepared.priority.clone());
        if let [only] = prepared.workspaces.as_slice() {
            self.form_workspace_id.set(only.id.clone());
        }
    }

    pub fn submit(&self) {
        self.convert.dispatch(ConvertJiraIssue {
            input: build_convert_dto(
                &self.key.get_value(),
                &self.form_workspace_id.get_untracked(),
                &self.form_base_branch.get_untracked(),
                &self.form_title.get_untracked(),
                &self.form_prompt.get_untracked(),
                &self.form_priority.get_untracked(),
                self.form_start_now.get_untracked(),
            ),
        });
    }

    /// The created task, once the conversion succeeded.
    pub fn converted(&self) -> Option<ConvertedIssueDto> {
        self.convert.value().get().and_then(Result::ok)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn blank_branch_becomes_none_and_values_pass_through() {
        let dto: ConvertIssueDto =
            build_convert_dto("A-1", "w1", "  ", "Título", "Prompt", "high", true);
        assert_eq!(dto.base_branch, None);
        assert_eq!(dto.key, "A-1");
        assert_eq!(dto.workspace_id, "w1");
        assert_eq!(dto.title, "Título");
        assert_eq!(dto.prompt, "Prompt");
        assert_eq!(dto.priority, "high");
        assert!(dto.start_now);
    }

    #[test]
    fn explicit_branch_is_kept_trimmed() {
        let dto: ConvertIssueDto =
            build_convert_dto("A-1", "w1", " origin/dev ", "t", "p", "normal", false);
        assert_eq!(dto.base_branch.as_deref(), Some("origin/dev"));
    }
}
