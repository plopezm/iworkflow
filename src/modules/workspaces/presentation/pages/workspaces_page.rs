use leptos::prelude::*;

use crate::core::components::form::ActionError;
use crate::core::components::layout::{Card, ErrorText};
use crate::core::i18n::Msg;
use crate::modules::workspaces::infrastructure::adapters::inbound::dto::workspace_dto::{
    InitializationReasonDto, RegisterWorkspaceOutcomeDto, WorkspaceDto,
};
use crate::modules::workspaces::presentation::components::confirm_panel::ConfirmPanel;
use crate::modules::workspaces::presentation::components::directory_picker::DirectoryPicker;
use crate::modules::workspaces::presentation::components::workspace_form::WorkspaceForm;
use crate::modules::workspaces::presentation::components::workspace_table::WorkspaceTable;
use crate::modules::workspaces::presentation::state::workspaces_state::WorkspacesState;

/// Mensaje inline explicando por qué hace falta confirmar la inicialización.
fn initialization_message(path: &str, reason: InitializationReasonDto) -> String {
    let message: Msg = match reason {
        InitializationReasonDto::MissingFolder => MISSING_FOLDER,
        InitializationReasonDto::NotAGitRepository => NOT_A_GIT_REPOSITORY,
        InitializationReasonDto::NoCommits => NO_COMMITS,
    };
    message.fmt(&[("path", path)])
}

fn initialization_confirm_label(reason: InitializationReasonDto) -> &'static str {
    match reason {
        InitializationReasonDto::MissingFolder => CREATE_AND_INITIALIZE.get(),
        InitializationReasonDto::NotAGitRepository => INITIALIZE.get(),
        InitializationReasonDto::NoCommits => CREATE_COMMIT.get(),
    }
}

const MISSING_FOLDER: Msg = Msg {
    en: "The folder {path} does not exist. Create it and initialize a git repository?",
    es: "La carpeta {path} no existe. ¿Crearla e inicializar un repositorio git?",
    zh: "文件夹 {path} 不存在。是否创建并初始化 git 仓库？",
    de: "Der Ordner {path} existiert nicht. Erstellen und ein Git-Repository initialisieren?",
    fr: "Le dossier {path} n'existe pas. Le créer et initialiser un dépôt git ?",
};
const NOT_A_GIT_REPOSITORY: Msg = Msg {
    en: "{path} is not a git repository. Initialize it?",
    es: "{path} no es un repositorio git. ¿Inicializarlo?",
    zh: "{path} 不是 git 仓库。是否初始化？",
    de: "{path} ist kein Git-Repository. Initialisieren?",
    fr: "{path} n'est pas un dépôt git. L'initialiser ?",
};
const NO_COMMITS: Msg = Msg {
    en: "The repository {path} has no commits. Create an empty initial commit?",
    es: "El repositorio {path} no tiene commits. ¿Crear un commit inicial vacío?",
    zh: "仓库 {path} 没有提交。是否创建一个空的初始提交？",
    de: "Das Repository {path} hat keine Commits. Einen leeren initialen Commit erstellen?",
    fr: "Le dépôt {path} n'a aucun commit. Créer un commit initial vide ?",
};
const CREATE_AND_INITIALIZE: Msg = Msg {
    en: "Create and initialize",
    es: "Crear e inicializar",
    zh: "创建并初始化",
    de: "Erstellen und initialisieren",
    fr: "Créer et initialiser",
};
const INITIALIZE: Msg = Msg {
    en: "Initialize",
    es: "Inicializar",
    zh: "初始化",
    de: "Initialisieren",
    fr: "Initialiser",
};
const CREATE_COMMIT: Msg = Msg {
    en: "Create commit",
    es: "Crear commit",
    zh: "创建提交",
    de: "Commit erstellen",
    fr: "Créer un commit",
};
const WORKSPACES: Msg = Msg {
    en: "Workspaces",
    es: "Workspaces",
    zh: "工作区",
    de: "Workspaces",
    fr: "Espaces de travail",
};
const REGISTER_WORKSPACE: Msg = Msg {
    en: "Register workspace",
    es: "Registrar workspace",
    zh: "注册工作区",
    de: "Workspace registrieren",
    fr: "Enregistrer un espace de travail",
};
const REGISTERED: Msg = Msg {
    en: "Registered",
    es: "Registrados",
    zh: "已注册",
    de: "Registriert",
    fr: "Enregistrés",
};
const LOADING: Msg = Msg {
    en: "Loading…",
    es: "Cargando…",
    zh: "加载中…",
    de: "Wird geladen…",
    fr: "Chargement…",
};

#[component]
pub fn WorkspacesPage() -> impl IntoView {
    let state: WorkspacesState = WorkspacesState::new();

    Effect::new(move |_| {
        if let Some(Ok(RegisterWorkspaceOutcomeDto::Registered(_))) = state.register.value().get() {
            state.reset_form();
        }
    });

    let on_submit: Callback<()> = Callback::new(move |_| state.submit());
    let on_delete: Callback<String> = Callback::new(move |id: String| state.remove(id));
    let on_browse: Callback<()> = Callback::new(move |_| state.open_directory_picker());
    let on_select_directory: Callback<String> =
        Callback::new(move |path: String| state.select_directory(path));
    let on_confirm_initialization: Callback<()> =
        Callback::new(move |_| state.confirm_initialization());
    let on_cancel_initialization: Callback<()> =
        Callback::new(move |_| state.dismiss_initialization());

    view! {
        <div class="flex h-full min-h-0 flex-col">
            <div class="flex shrink-0 items-center border-b border-border bg-surface px-4 py-2.5">
                <h1 class="page-title">{WORKSPACES.get()}</h1>
            </div>
            <div class="stack min-h-0 flex-1 overflow-y-auto p-4">
            <Card title=REGISTER_WORKSPACE.get()>
                <WorkspaceForm
                    name=state.name
                    path=state.path
                    base_branch=state.base_branch
                    agents_file_id=state.agents_file_id
                    agents_file_options=state.agents_file_options
                    pending=state.register.pending()
                    on_submit
                    on_browse
                />
                <ActionError result=state.register.value().into() />
                {move || {
                    state.pending_initialization.get().map(|(path, reason): (String, InitializationReasonDto)| {
                        let message: Signal<String> =
                            Signal::derive(move || initialization_message(&path, reason));
                        let confirm_label: Signal<String> = Signal::derive(move || {
                            initialization_confirm_label(reason).to_string()
                        });
                        view! {
                            <ConfirmPanel
                                message=message
                                confirm_label=confirm_label
                                pending=state.register.pending()
                                on_confirm=on_confirm_initialization
                                on_cancel=on_cancel_initialization
                            />
                        }
                    })
                }}
            </Card>
            <DirectoryPicker state=state.directory_picker on_select=on_select_directory />
            <Card title=REGISTERED.get()>
                <ActionError result=state.delete.value().into() />
                <Transition fallback=|| view! { <p class="muted">{LOADING.get()}</p> }>
                    {move || Suspend::new(async move {
                        match state.workspaces.await {
                            Ok(list) => {
                                let list: Vec<WorkspaceDto> = list;
                                view! { <WorkspaceTable workspaces=list on_delete /> }.into_any()
                            }
                            Err(e) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                        }
                    })}
                </Transition>
            </Card>
            </div>
        </div>
    }
}
