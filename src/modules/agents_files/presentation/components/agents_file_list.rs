use leptos::prelude::*;

use crate::modules::agents_files::infrastructure::adapters::inbound::dto::agents_file_dto::{
    AgentsFileDto, ProjectOptionDto,
};
use crate::modules::agents_files::presentation::components::agents_file_item::AgentsFileItem;
use crate::modules::agents_files::presentation::state::agents_files_state::ApplyFeedback;

/// Listado de ficheros AGENTS.md. `feedback_for` resuelve el aviso de cada fichero.
#[component]
pub fn AgentsFileList(
    files: Vec<AgentsFileDto>,
    projects: Vec<ProjectOptionDto>,
    feedback_for: Callback<String, Signal<Option<ApplyFeedback>>>,
    #[prop(into)] applying: Signal<bool>,
    on_apply: Callback<(String, String)>,
    on_confirm_overwrite: Callback<()>,
    on_dismiss: Callback<()>,
    on_delete: Callback<String>,
) -> impl IntoView {
    if files.is_empty() {
        return view! { <p class="muted">"Aún no hay ficheros AGENTS.md."</p> }.into_any();
    }

    view! {
        <div class="stack">
            {files
                .into_iter()
                .map(|file: AgentsFileDto| {
                    let feedback: Signal<Option<ApplyFeedback>> = feedback_for.run(file.id.clone());
                    view! {
                        <AgentsFileItem
                            file
                            projects=projects.clone()
                            feedback
                            applying
                            on_apply
                            on_confirm_overwrite
                            on_dismiss
                            on_delete
                        />
                    }
                })
                .collect_view()}
        </div>
    }
    .into_any()
}
