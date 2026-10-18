use leptos::prelude::*;
use leptos::web_sys::FormData;

use crate::core::components::form::ActionError;
use crate::core::components::layout::{Card, ErrorText};
use crate::core::i18n::Msg;
use crate::modules::agents_files::infrastructure::adapters::inbound::dto::agents_file_dto::{
    AgentsFileDto, ProjectOptionDto,
};
use crate::modules::agents_files::presentation::components::agents_file_list::AgentsFileList;
use crate::modules::agents_files::presentation::components::agents_file_upload_form::AgentsFileUploadForm;
use crate::modules::agents_files::presentation::state::agents_files_state::{
    AgentsFilesState, ApplyFeedback,
};

const UPLOAD_FILE: Msg = Msg {
    en: "Upload file",
    es: "Subir fichero",
    zh: "上传文件",
    de: "Datei hochladen",
    fr: "Envoyer un fichier",
};
const FILES: Msg = Msg {
    en: "Files",
    es: "Ficheros",
    zh: "文件",
    de: "Dateien",
    fr: "Fichiers",
};
const LOADING: Msg = Msg {
    en: "Loading…",
    es: "Cargando…",
    zh: "加载中…",
    de: "Wird geladen…",
    fr: "Chargement…",
};

#[component]
pub fn AgentsFilesPage() -> impl IntoView {
    let state: AgentsFilesState = AgentsFilesState::new();

    Effect::new(move |_| {
        if let Some(Ok(_)) = state.upload.value().get() {
            state.reset_form();
        }
    });

    let on_submit: Callback<FormData> =
        Callback::new(move |data: FormData| state.submit_upload(data));
    let on_delete: Callback<String> = Callback::new(move |id: String| state.remove(id));
    let on_apply: Callback<(String, String)> =
        Callback::new(move |(file_id, project_id): (String, String)| {
            state.apply_to_project(file_id, project_id)
        });
    let on_confirm_overwrite: Callback<()> = Callback::new(move |_| state.confirm_overwrite());
    let on_dismiss: Callback<()> = Callback::new(move |_| state.dismiss_feedback());
    let feedback_for: Callback<String, Signal<Option<ApplyFeedback>>> =
        Callback::new(move |id: String| state.feedback_for(id));
    let applying: Signal<bool> = state.apply.pending().into();

    view! {
        <div class="flex h-full min-h-0 flex-col">
            <div class="flex shrink-0 items-center border-b border-border bg-surface px-4 py-2.5">
                <h1 class="page-title">"AGENTS.md"</h1>
            </div>
            <div class="stack min-h-0 flex-1 overflow-y-auto p-4">
                <Card title=UPLOAD_FILE.get()>
                    <AgentsFileUploadForm
                        description=state.description
                        form_ref=state.form_ref
                        pending=state.upload.pending()
                        on_submit
                    />
                    <ActionError result=state.upload.value().into() />
                </Card>
                <Card title=FILES.get()>
                    <ActionError result=state.apply.value().into() />
                    <ActionError result=state.delete.value().into() />
                    <Transition fallback=|| view! { <p class="muted">{LOADING.get()}</p> }>
                        {move || Suspend::new(async move {
                            let files: Result<Vec<AgentsFileDto>, ServerFnError> =
                                state.agents_files.await;
                            let projects: Result<Vec<ProjectOptionDto>, ServerFnError> =
                                state.projects.await;
                            match (files, projects) {
                                (Ok(files), Ok(projects)) => {
                                    view! {
                                        <AgentsFileList
                                            files
                                            projects
                                            feedback_for
                                            applying
                                            on_apply
                                            on_confirm_overwrite
                                            on_dismiss
                                            on_delete
                                        />
                                    }
                                        .into_any()
                                }
                                (Err(e), _) | (_, Err(e)) => {
                                    view! { <ErrorText message=e.to_string() /> }.into_any()
                                }
                            }
                        })}
                    </Transition>
                </Card>
            </div>
        </div>
    }
}
