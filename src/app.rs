use leptos::prelude::*;
use leptos_meta::{MetaTags, Stylesheet, Title, provide_meta_context};
use leptos_router::components::{Route, Router, Routes};
use leptos_router::hooks::use_location;
use leptos_router::params::ParamsMap;
use leptos_router::{StaticSegment, WildcardSegment};

use crate::core::browser;
use crate::core::components::language_picker::{LanguageCard, LanguagePrompt};
use crate::core::components::shell::{AppShell, SidebarSection, SidebarToggle};
use crate::core::components::theme_toggle::{THEME_INIT_SCRIPT, ThemeToggle, provide_theme_mode};
use crate::core::dock::context::{DockContext, use_dock};
use crate::core::dock::host::{DockHooks, DockHost};
use crate::core::dock::link::TabLink;
use crate::core::i18n::{Msg, detect_locale, provide_locale};
use crate::core::refresh::DataRefresh;
use crate::core::url_path::decode_path;
use crate::modules::agent_profiles::presentation::pages::models_page::ModelsPage;
use crate::modules::agent_profiles::presentation::pages::profiles_page::ProfilesPage;
use crate::modules::agent_settings::presentation::components::agent_settings_bar::AgentSettingsBar;
use crate::modules::agent_settings::presentation::pages::settings_page::SettingsPage;
use crate::modules::agent_settings::presentation::state::agent_settings_state::AgentSettingsState;
use crate::modules::agents_files::presentation::pages::agents_files_page::AgentsFilesPage;
use crate::modules::code_hosting::presentation::components::code_host_settings_card::CodeHostSettingsCard;
use crate::modules::code_hosting::presentation::components::task_merge_request_panel::TaskMergeRequestPanel;
use crate::modules::jira::presentation::components::connection_card::JiraSettingsSection;
use crate::modules::jira::presentation::pages::jira_convert_page::JiraConvertPage;
use crate::modules::jira::presentation::pages::jira_issue_page::JiraIssuePage;
use crate::modules::jira::presentation::pages::jira_page::JiraPage;
use crate::modules::preferences::presentation::choose_language::choose_language;
use crate::modules::project_tree::presentation::pages::project_file_page::ProjectFilePage;
use crate::modules::project_tree::presentation::pages::project_tree_page::ProjectTreePage;
use crate::modules::tasks::presentation::components::app_sidebar::AppSidebar;
use crate::modules::tasks::presentation::pages::board_page::BoardPage;
use crate::modules::tasks::presentation::pages::task_detail_page::{TaskDetailPage, TaskExtraTab};
use crate::modules::tasks::presentation::pages::task_diff_page::TaskDiffPage;
use crate::modules::tasks::presentation::pages::task_edit_page::TaskEditPage;
use crate::modules::tasks::presentation::state::task_store::{TaskStore, use_task_store};
use crate::modules::terminal::presentation::pages::new_task_terminal_page::NewTaskTerminalPage;
use crate::modules::terminal::presentation::pages::new_terminal_page::NewTerminalPage;
use crate::modules::terminal::presentation::pages::terminal_session_page::TerminalSessionPage;
use crate::modules::terminal::presentation::pages::terminals_page::TerminalsPage;
use crate::modules::workspaces::presentation::pages::workspace_edit_page::WorkspaceEditPage;
use crate::modules::workspaces::presentation::pages::workspaces_page::WorkspacesPage;

pub fn shell(options: LeptosOptions) -> impl IntoView {
    let lang: &'static str = detect_locale().locale.code();
    view! {
        <!DOCTYPE html>
        <html lang=lang>
            <head>
                <meta charset="utf-8" />
                <script inner_html=THEME_INIT_SCRIPT></script>
                <meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=1, user-scalable=no" />
                <AutoReload options=options.clone() />
                <HydrationScripts options />
                <MetaTags />
            </head>
            <body>
                <App />
            </body>
        </html>
    }
}

const AGENTS_FILES: Msg = Msg {
    en: "AGENTS.md",
    es: "AGENTS.md",
    zh: "AGENTS.md",
    de: "AGENTS.md",
    fr: "AGENTS.md",
};

const JIRA: Msg = Msg {
    en: "Jira",
    es: "Jira",
    zh: "Jira",
    de: "Jira",
    fr: "Jira",
};

const TITLE_BOARD: Msg = Msg {
    en: "Board",
    es: "Tablero",
    zh: "看板",
    de: "Board",
    fr: "Tableau",
};

const TITLE_WORKSPACES: Msg = Msg {
    en: "Workspaces",
    es: "Workspaces",
    zh: "工作区",
    de: "Workspaces",
    fr: "Espaces de travail",
};

const TITLE_PROJECT: Msg = Msg {
    en: "Project",
    es: "Proyecto",
    zh: "项目",
    de: "Projekt",
    fr: "Projet",
};

const TITLE_FILES: Msg = Msg {
    en: "Files",
    es: "Archivos",
    zh: "文件",
    de: "Dateien",
    fr: "Fichiers",
};

const TITLE_EDIT_WORKSPACE: Msg = Msg {
    en: "Edit workspace",
    es: "Editar workspace",
    zh: "编辑工作区",
    de: "Workspace bearbeiten",
    fr: "Modifier l'espace de travail",
};

const TITLE_TERMINAL: Msg = Msg {
    en: "Terminal",
    es: "Terminal",
    zh: "终端",
    de: "Terminal",
    fr: "Terminal",
};

const TITLE_TERMINALS: Msg = Msg {
    en: "Terminals",
    es: "Terminales",
    zh: "终端",
    de: "Terminals",
    fr: "Terminaux",
};

const TITLE_CONVERT: Msg = Msg {
    en: "Convert {key}",
    es: "Convertir {key}",
    zh: "转换 {key}",
    de: "{key} konvertieren",
    fr: "Convertir {key}",
};

const TITLE_PROFILES: Msg = Msg {
    en: "Profiles",
    es: "Perfiles",
    zh: "配置文件",
    de: "Profile",
    fr: "Profils",
};

const TITLE_MODELS: Msg = Msg {
    en: "Models",
    es: "Modelos",
    zh: "模型",
    de: "Modelle",
    fr: "Modèles",
};

const TITLE_SETTINGS: Msg = Msg {
    en: "Settings",
    es: "Configuración",
    zh: "设置",
    de: "Einstellungen",
    fr: "Paramètres",
};

const TITLE_TASK: Msg = Msg {
    en: "Task",
    es: "Tarea",
    zh: "任务",
    de: "Aufgabe",
    fr: "Tâche",
};

const TITLE_EDIT_TASK: Msg = Msg {
    en: "Edit task",
    es: "Editar tarea",
    zh: "编辑任务",
    de: "Aufgabe bearbeiten",
    fr: "Modifier la tâche",
};

const TITLE_NOT_FOUND: Msg = Msg {
    en: "Not found",
    es: "No encontrada",
    zh: "未找到",
    de: "Nicht gefunden",
    fr: "Introuvable",
};

const PAGE_NOT_FOUND: Msg = Msg {
    en: "Page not found.",
    es: "Página no encontrada.",
    zh: "未找到页面。",
    de: "Seite nicht gefunden.",
    fr: "Page introuvable.",
};

const MENU: Msg = Msg {
    en: "Menu",
    es: "Menú",
    zh: "菜单",
    de: "Menü",
    fr: "Menu",
};

const NAVIGATION: Msg = Msg {
    en: "Navigation",
    es: "Navegación",
    zh: "导航",
    de: "Navigation",
    fr: "Navigation",
};

const EMPTY_GROUP: Msg = Msg {
    en: "This group has no tabs. Open one:",
    es: "Este grupo no tiene pestañas. Abre una:",
    zh: "此分组没有标签页。请打开一个：",
    de: "Diese Gruppe hat keine Tabs. Öffne einen:",
    fr: "Ce groupe n'a aucun onglet. Ouvrez-en un :",
};

/// Pantalla que muestra una pestaña, deducida de su ruta.
#[derive(Debug, Clone, PartialEq, Eq)]
enum TabRoute {
    /// Tablero de un workspace; sin id (`/`) muestra el primero por nombre.
    Board(Option<String>),
    Workspaces,
    WorkspaceTree(String),
    /// Fichero del proyecto: id del workspace y ruta relativa (ya decodificada).
    WorkspaceFile(String, String),
    WorkspaceEdit(String),
    WorkspaceTerminal(String),
    Terminals,
    TerminalSession(String),
    AgentsFiles,
    Jira,
    JiraIssue(String),
    JiraConvert(String),
    Profiles,
    Models,
    Settings,
    Task(String),
    TaskEdit(String),
    TaskTerminal(String),
    TaskDiff(String),
    /// Explorador de ficheros del worktree de una tarea.
    TaskTree(String),
    /// Fichero del worktree de una tarea: id de la tarea y ruta relativa (ya decodificada).
    TaskFile(String, String),
    NotFound,
}

impl TabRoute {
    fn parse(path: &str) -> Self {
        let segments: Vec<&str> = path.split('/').filter(|s: &&str| !s.is_empty()).collect();
        match segments.as_slice() {
            [] | ["board"] => Self::Board(None),
            ["board", id] => Self::Board(Some((*id).to_string())),
            ["workspaces"] => Self::Workspaces,
            ["workspaces", id] => Self::WorkspaceTree((*id).to_string()),
            ["workspaces", id, "files", rest @ ..] if !rest.is_empty() => {
                Self::WorkspaceFile((*id).to_string(), decode_path(&rest.join("/")))
            }
            ["workspaces", id, "edit"] => Self::WorkspaceEdit((*id).to_string()),
            ["workspaces", id, "terminal"] => Self::WorkspaceTerminal((*id).to_string()),
            ["terminals"] => Self::Terminals,
            ["terminals", id] => Self::TerminalSession((*id).to_string()),
            ["agents-files"] => Self::AgentsFiles,
            ["jira"] => Self::Jira,
            ["jira", key] => Self::JiraIssue((*key).to_string()),
            ["jira", key, "convert"] => Self::JiraConvert((*key).to_string()),
            ["profiles"] => Self::Profiles,
            ["models"] => Self::Models,
            ["settings"] => Self::Settings,
            ["tasks", id] => Self::Task((*id).to_string()),
            ["tasks", id, "edit"] => Self::TaskEdit((*id).to_string()),
            ["tasks", id, "terminal"] => Self::TaskTerminal((*id).to_string()),
            ["tasks", id, "diff"] => Self::TaskDiff((*id).to_string()),
            ["tasks", id, "files"] => Self::TaskTree((*id).to_string()),
            ["tasks", id, "files", rest @ ..] => {
                Self::TaskFile((*id).to_string(), decode_path(&rest.join("/")))
            }
            _ => Self::NotFound,
        }
    }

    fn title(&self) -> String {
        match self {
            Self::Board(_) => TITLE_BOARD.get().to_string(),
            Self::Workspaces => TITLE_WORKSPACES.get().to_string(),
            Self::WorkspaceTree(_) => TITLE_PROJECT.get().to_string(),
            Self::TaskTree(_) => TITLE_FILES.get().to_string(),
            Self::WorkspaceFile(_, path) | Self::TaskFile(_, path) => {
                path.rsplit('/').next().unwrap_or(path).to_string()
            }
            Self::WorkspaceEdit(_) => TITLE_EDIT_WORKSPACE.get().to_string(),
            Self::WorkspaceTerminal(_) | Self::TaskTerminal(_) => TITLE_TERMINAL.get().to_string(),
            Self::Terminals => TITLE_TERMINALS.get().to_string(),
            Self::TerminalSession(id) => {
                format!(
                    "{} {}",
                    TITLE_TERMINAL.get(),
                    id.chars().take(8).collect::<String>()
                )
            }
            Self::AgentsFiles => "AGENTS.md".to_string(),
            Self::Jira => "Jira".to_string(),
            Self::JiraIssue(key) => key.clone(),
            Self::JiraConvert(key) => TITLE_CONVERT.fmt(&[("key", key)]),
            Self::Profiles => TITLE_PROFILES.get().to_string(),
            Self::Models => TITLE_MODELS.get().to_string(),
            Self::Settings => TITLE_SETTINGS.get().to_string(),
            Self::Task(_) => TITLE_TASK.get().to_string(),
            Self::TaskEdit(_) => TITLE_EDIT_TASK.get().to_string(),
            Self::TaskDiff(_) => "Diff".to_string(),
            Self::NotFound => TITLE_NOT_FOUND.get().to_string(),
        }
    }

    fn icon(&self) -> &'static str {
        match self {
            Self::Board(_) => "▦",
            Self::Workspaces | Self::WorkspaceTree(_) | Self::TaskTree(_) => "▤",
            Self::WorkspaceFile(..) | Self::TaskFile(..) => "◫",
            Self::WorkspaceEdit(_) | Self::TaskEdit(_) => "✎",
            Self::TaskDiff(_) => "±",
            Self::WorkspaceTerminal(_)
            | Self::Terminals
            | Self::TerminalSession(_)
            | Self::TaskTerminal(_) => "❯",
            Self::AgentsFiles => "§",
            Self::Jira | Self::JiraIssue(_) => "◈",
            Self::JiraConvert(_) => "⇄",
            Self::Profiles => "☰",
            Self::Models => "◇",
            Self::Settings => "⚙",
            Self::Task(_) => "●",
            Self::NotFound => "?",
        }
    }

    /// Parámetros de ruta que esperan las páginas (`id` o `session_id`).
    fn params(&self) -> ParamsMap {
        let mut params: ParamsMap = ParamsMap::new();
        if let Self::Board(Some(workspace_id)) = self {
            params.insert("workspace_id", workspace_id.clone());
        }
        if let Self::WorkspaceFile(id, path) = self {
            params.insert("id", id.clone());
            params.insert("path", path.clone());
        }
        if let Self::TaskFile(id, path) = self {
            params.insert("id", task_scope(id));
            params.insert("path", path.clone());
        }
        match self {
            Self::WorkspaceTree(id)
            | Self::WorkspaceEdit(id)
            | Self::WorkspaceTerminal(id)
            | Self::Task(id)
            | Self::TaskEdit(id)
            | Self::TaskTerminal(id)
            | Self::TaskDiff(id) => params.insert("id", id.clone()),
            Self::TaskTree(id) => params.insert("id", task_scope(id)),
            Self::JiraIssue(key) | Self::JiraConvert(key) => params.insert("key", key.clone()),
            Self::TerminalSession(id) => params.insert("session_id", id.clone()),
            _ => {}
        }
        params
    }

    fn view(self) -> AnyView {
        match self {
            Self::Board(_) => view! { <BoardPage /> }.into_any(),
            Self::Workspaces => view! { <WorkspacesPage /> }.into_any(),
            Self::WorkspaceTree(_) | Self::TaskTree(_) => view! { <ProjectTreePage /> }.into_any(),
            Self::WorkspaceFile(..) | Self::TaskFile(..) => {
                view! { <ProjectFilePage /> }.into_any()
            }
            Self::WorkspaceEdit(_) => view! { <WorkspaceEditPage /> }.into_any(),
            Self::WorkspaceTerminal(_) => view! { <NewTerminalPage /> }.into_any(),
            Self::Terminals => view! { <TerminalsPage /> }.into_any(),
            Self::TerminalSession(_) => view! { <TerminalSessionPage /> }.into_any(),
            Self::AgentsFiles => view! { <AgentsFilesPage /> }.into_any(),
            Self::Jira => view! { <JiraPage /> }.into_any(),
            Self::JiraIssue(_) => view! { <JiraIssuePage /> }.into_any(),
            Self::JiraConvert(_) => view! { <JiraConvertPage /> }.into_any(),
            Self::Profiles => view! { <ProfilesPage /> }.into_any(),
            Self::Models => view! { <ModelsPage /> }.into_any(),
            Self::Settings => view! {
                <SettingsPage leading=Box::new(|| view! { <LanguageCard on_choose=Callback::new(choose_language) /> }.into_any())>
                    <JiraSettingsSection />
                    <CodeHostSettingsCard provider="gitlab" />
                    <CodeHostSettingsCard provider="github" />
                </SettingsPage>
            }
            .into_any(),
            Self::Task(_) => view! { <TaskDetailPage extra_tab=code_host_task_tab() /> }.into_any(),
            Self::TaskEdit(_) => view! { <TaskEditPage /> }.into_any(),
            Self::TaskDiff(_) => view! { <TaskDiffPage /> }.into_any(),
            Self::TaskTerminal(_) => view! { <NewTaskTerminalPage /> }.into_any(),
            Self::NotFound => view! { <p class="muted p-4">{PAGE_NOT_FOUND.get()}</p> }.into_any(),
        }
    }
}

/// Scope id that the project tree server functions expect for a task worktree.
fn task_scope(task_id: &str) -> String {
    format!("task:{task_id}")
}

fn tab_title(path: &str) -> String {
    TabRoute::parse(path).title()
}

fn tab_icon(path: &str) -> &'static str {
    TabRoute::parse(path).icon()
}

fn tab_params(path: &str) -> ParamsMap {
    TabRoute::parse(path).params()
}

/// `true` si la ruta es la de un tablero, de cualquier workspace.
fn is_board_path(path: &str) -> bool {
    matches!(TabRoute::parse(path), TabRoute::Board(_))
}

/// Predicado de reutilización para el destino `/` (tablero): si ya hay uno abierto se
/// activa en lugar de abrir otro con el primer workspace.
fn board_reuse(path: &str) -> Option<fn(&str) -> bool> {
    (path == "/").then_some(is_board_path as fn(&str) -> bool)
}

/// Destinos de nivel superior que ofrecen la barra lateral, el menú `+` y los grupos vacíos.
const TOP_LEVEL_DESTINATIONS: [(&str, &str, Msg); 8] = [
    ("/", "▦", TITLE_BOARD),
    ("/terminals", "❯", TITLE_TERMINALS),
    ("/workspaces", "▤", TITLE_WORKSPACES),
    ("/agents-files", "§", AGENTS_FILES),
    ("/jira", "◈", JIRA),
    ("/profiles", "☰", TITLE_PROFILES),
    ("/models", "◇", TITLE_MODELS),
    ("/settings", "⚙", TITLE_SETTINGS),
];

/// Navegación de la barra lateral, en una sección colapsable "Menú".
fn sidebar_nav() -> impl IntoView {
    let dock: DockContext = use_dock();
    view! {
        <SidebarSection title=MENU.get() count=Signal::stored(TOP_LEVEL_DESTINATIONS.len())>
            <nav class="flex flex-col gap-0.5" aria-label=NAVIGATION.get()>
                {TOP_LEVEL_DESTINATIONS
                    .into_iter()
                    .map(|(path, icon, label): (&str, &str, Msg)| {
                        view! {
                            <TabLink
                                href=path
                                reuse=board_reuse(path)
                                class=move || {
                                    if dock.is_path_visible(path) {
                                        "sidebar-nav sidebar-nav-active".to_string()
                                    } else {
                                        "sidebar-nav".to_string()
                                    }
                                }
                            >
                                <span class="sidebar-nav-icon" aria-hidden="true">{icon}</span>
                                {label.get()}
                            </TabLink>
                        }
                    })
                    .collect_view()}
            </nav>
        </SidebarSection>
    }
}

/// Contenido de la barra lateral: marca, navegación, listados y ajustes del agente.
fn app_sidebar() -> impl IntoView {
    view! {
        <div class="flex shrink-0 items-center gap-2 px-3 pt-3 pb-1">
            <span class="live-dot text-accent" aria-hidden="true"></span>
            <TabLink
                href="/"
                reuse=Some(is_board_path as fn(&str) -> bool)
                class="flex-1 text-sm font-semibold tracking-tight"
            >
                "iworkflow"
            </TabLink>
            <SidebarToggle class="hidden md:flex" />
        </div>
        {sidebar_nav()}
        <div class="min-h-0 flex-1 overflow-y-auto">
            <AppSidebar />
        </div>
        <div class="stack shrink-0 gap-1.5 border-t border-border p-3">
            <ThemeToggle />
            <AgentSettingsBar />
        </div>
    }
}

/// Entradas del menú `+` de cada grupo.
fn tab_launcher() -> impl IntoView {
    TOP_LEVEL_DESTINATIONS
        .into_iter()
        .map(|(path, icon, label): (&str, &str, Msg)| {
            view! {
                <TabLink href=path reuse=board_reuse(path) class="menu-item">
                    <span class="mr-2 inline-block w-4 text-center text-muted" aria-hidden="true">{icon}</span>
                    {label.get()}
                </TabLink>
            }
        })
        .collect_view()
}

/// Contenido de un grupo sin pestañas.
fn empty_group() -> impl IntoView {
    view! {
        <div class="absolute inset-0 flex flex-col items-center justify-center gap-4 p-6 text-center">
            <p class="text-sm text-muted">{EMPTY_GROUP.get()}</p>
            <div class="flex max-w-md flex-wrap justify-center gap-2">
                {TOP_LEVEL_DESTINATIONS
                    .into_iter()
                    .map(|(path, icon, label): (&str, &str, Msg)| {
                        view! {
                            <TabLink
                                href=path
                                reuse=board_reuse(path)
                                class="btn-secondary min-h-8 px-3 py-1 text-xs"
                            >
                                <span class="mr-1.5" aria-hidden="true">{icon}</span>
                                {label.get()}
                            </TabLink>
                        }
                    })
                    .collect_view()}
            </div>
        </div>
    }
}

/// Crea una sola vez el almacén de tareas (con su única suscripción SSE) y lo provee por
/// contexto a la barra lateral y a las páginas. Debe ir dentro del `<Router>`.
#[component]
fn TaskStoreProvider(children: Children) -> impl IntoView {
    provide_context(TaskStore::new());
    children()
}

/// Único título del documento: el contador de tareas que requieren atención. Las pestañas
/// ponen su propio título en la barra del dock, no en `<title>`.
#[component]
fn DocumentTitle() -> impl IntoView {
    let store: TaskStore = use_task_store();
    let text: Memo<String> = Memo::new(move |_| {
        let count: usize = store
            .active
            .with(|tasks| tasks.iter().filter(|t| t.needs_attention).count());
        if count > 0 {
            format!("({count}) iworkflow")
        } else {
            "iworkflow".to_string()
        }
    });
    view! { <Title text=move || text.get() /> }
}

/// Tras restaurar el layout guardado abre (o activa) la pestaña de la ruta del navegador:
/// enlace directo, recarga o visita a `/` (tablero).
#[component]
fn DockBootstrap() -> impl IntoView {
    let dock: DockContext = use_dock();
    let pathname: Memo<String> = use_location().pathname;
    Effect::new(move |_| {
        if !dock.restored.get() {
            return;
        }
        let path: String = pathname.get_untracked();
        // Abrir una ruta ya abierta solo activa su pestaña.
        dock.open(&path, None, None);
    });
    // La barra de direcciones refleja la pestaña con foco (enlaces directos y recarga).
    Effect::new(move |_| {
        let focused: Option<String> = dock.layout.with(|l| l.focused_path());
        if dock.restored.get()
            && let Some(path) = focused
        {
            browser::replace_url(&path);
        }
    });
}

#[component]
pub fn App() -> impl IntoView {
    provide_meta_context();
    provide_locale();
    provide_theme_mode();
    DockContext::provide(tab_title);
    DataRefresh::provide();
    provide_context(AgentSettingsState::new());
    let hooks: DockHooks = DockHooks {
        render: Callback::new(|path: String| TabRoute::parse(&path).view()),
        params_for: tab_params,
        icon_for: tab_icon,
    };

    view! {
        <Stylesheet id="leptos" href="/pkg/iworkflow.css" />
        <Router>
            // Sin pantallas enrutadas: las pestañas del dock las sustituyen. Estas rutas solo
            // hacen que el servidor entregue el shell en los enlaces directos conocidos (un comodín
            // global taparía los ficheros estáticos de `/pkg`).
            <Routes fallback=|| ()>
                <Route path=StaticSegment("") view=|| () />
                <Route path=StaticSegment("workspaces") view=|| () />
                <Route path=StaticSegment("board") view=|| () />
                <Route path=(StaticSegment("board"), WildcardSegment("rest")) view=|| () />
                <Route path=StaticSegment("terminals") view=|| () />
                <Route path=StaticSegment("agents-files") view=|| () />
                <Route path=StaticSegment("jira") view=|| () />
                <Route path=(StaticSegment("jira"), WildcardSegment("rest")) view=|| () />
                <Route path=StaticSegment("profiles") view=|| () />
                <Route path=StaticSegment("models") view=|| () />
                <Route path=StaticSegment("settings") view=|| () />
                <Route path=(StaticSegment("tasks"), WildcardSegment("rest")) view=|| () />
                <Route path=(StaticSegment("workspaces"), WildcardSegment("rest")) view=|| () />
                <Route path=(StaticSegment("terminals"), WildcardSegment("rest")) view=|| () />
            </Routes>
            <TaskStoreProvider>
                <DocumentTitle />
                <LanguagePrompt on_choose=Callback::new(choose_language) />
                <DockBootstrap />
                <AppShell brand="iworkflow" home_reuse=is_board_path sidebar=app_sidebar>
                    <DockHost hooks=hooks launcher=tab_launcher empty=empty_group />
                </AppShell>
            </TaskStoreProvider>
        </Router>
    }
}

/// Pestaña de PR/MR (GitLab o GitHub) del panel de tarea: se compone aquí para que `tasks` no importe `code_hosting`.
fn code_host_task_tab() -> TaskExtraTab {
    TaskExtraTab {
        id: "code-host",
        label: "PR / MR",
        render: Callback::new(|task_id: String| {
            view! { <TaskMergeRequestPanel task_id=task_id /> }.into_any()
        }),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::core::i18n::{Locale, LocaleState};

    #[test]
    fn parse_maps_every_known_path_to_its_screen() {
        assert!(is_board_path("/"));
        assert!(is_board_path("/board/w2"));
        assert!(!is_board_path("/workspaces"));
        assert_eq!(board_reuse("/").map(|f| f("/board/w2")), Some(true));
        assert!(board_reuse("/jira").is_none());
        assert_eq!(TabRoute::parse("/"), TabRoute::Board(None));
        assert_eq!(TabRoute::parse("/board"), TabRoute::Board(None));
        assert_eq!(
            TabRoute::parse("/board/w1"),
            TabRoute::Board(Some("w1".into()))
        );
        assert_eq!(TabRoute::parse("/board/w1/x"), TabRoute::NotFound);
        assert_eq!(TabRoute::parse("/settings"), TabRoute::Settings);
        assert_eq!(TabRoute::Settings.title(), "Settings");
        assert_eq!(TabRoute::parse("/tasks/a1"), TabRoute::Task("a1".into()));
        assert_eq!(
            TabRoute::parse("/tasks/a1/edit"),
            TabRoute::TaskEdit("a1".into())
        );
        assert_eq!(
            TabRoute::parse("/tasks/a1/diff"),
            TabRoute::TaskDiff("a1".into())
        );
        assert_eq!(
            TabRoute::parse("/tasks/a1/terminal"),
            TabRoute::TaskTerminal("a1".into())
        );
        assert_eq!(
            TabRoute::parse("/workspaces/w"),
            TabRoute::WorkspaceTree("w".into())
        );
        assert_eq!(
            TabRoute::parse("/workspaces/w/files/src/a%20b/main.rs"),
            TabRoute::WorkspaceFile("w".into(), "src/a b/main.rs".into())
        );
        assert_eq!(TabRoute::parse("/workspaces/w/files"), TabRoute::NotFound);
        assert_eq!(
            TabRoute::parse("/tasks/t/files"),
            TabRoute::TaskTree("t".into())
        );
        assert_eq!(
            TabRoute::parse("/tasks/t/files/src/a%20b/main.rs"),
            TabRoute::TaskFile("t".into(), "src/a b/main.rs".into())
        );
        assert_eq!(
            TabRoute::TaskTree("t".into()).params().get("id"),
            Some("task:t".to_string())
        );
        assert_eq!(
            TabRoute::WorkspaceFile("w".into(), "src/main.rs".into()).title(),
            "main.rs"
        );
        assert_eq!(
            TabRoute::parse("/workspaces/w/edit"),
            TabRoute::WorkspaceEdit("w".into())
        );
        assert_eq!(
            TabRoute::parse("/workspaces/w/terminal"),
            TabRoute::WorkspaceTerminal("w".into())
        );
        assert_eq!(
            TabRoute::parse("/terminals/s1"),
            TabRoute::TerminalSession("s1".into())
        );
        assert_eq!(TabRoute::parse("/models"), TabRoute::Models);
        assert_eq!(TabRoute::parse("/jira"), TabRoute::Jira);
        assert_eq!(
            TabRoute::parse("/jira/ABC-12"),
            TabRoute::JiraIssue("ABC-12".into())
        );
        assert_eq!(
            TabRoute::parse("/jira/ABC-12/convert"),
            TabRoute::JiraConvert("ABC-12".into())
        );
        assert_eq!(TabRoute::JiraIssue("ABC-12".into()).title(), "ABC-12");
        assert_eq!(
            TabRoute::JiraConvert("ABC-12".into()).title(),
            "Convert ABC-12"
        );
        assert_eq!(TabRoute::parse("/nope/x/y/z"), TabRoute::NotFound);
    }

    #[test]
    fn params_expose_the_ids_pages_read() {
        assert_eq!(
            TabRoute::parse("/tasks/a1").params().get("id").as_deref(),
            Some("a1")
        );
        assert_eq!(
            TabRoute::parse("/terminals/s1")
                .params()
                .get("session_id")
                .as_deref(),
            Some("s1")
        );
        assert_eq!(
            TabRoute::parse("/jira/ABC-12/convert")
                .params()
                .get("key")
                .as_deref(),
            Some("ABC-12")
        );
        assert_eq!(TabRoute::Board(None).params().get("workspace_id"), None);
        assert_eq!(
            TabRoute::parse("/board/w1")
                .params()
                .get("workspace_id")
                .as_deref(),
            Some("w1")
        );
    }

    #[test]
    fn titles_fall_back_to_a_short_terminal_id() {
        assert_eq!(tab_title("/"), "Board");
        assert_eq!(tab_title("/board/w1"), "Board");
        assert_eq!(tab_title("/terminals/0123456789"), "Terminal 01234567");
    }

    #[test]
    fn titles_follow_the_active_language() {
        let owner: Owner = Owner::new();
        owner.with(|| {
            provide_context(LocaleState {
                locale: Locale::Es,
                chosen: true,
            });
            assert_eq!(tab_title("/settings"), "Configuración");
            assert_eq!(tab_title("/jira/ABC-1/convert"), "Convertir ABC-1");
        });
        Owner::new().with(|| {
            provide_context(LocaleState {
                locale: Locale::De,
                chosen: true,
            });
            assert_eq!(tab_title("/settings"), "Einstellungen");
        });
    }
}
