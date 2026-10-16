use leptos::prelude::*;
use leptos_meta::{MetaTags, Stylesheet, Title, provide_meta_context};
use leptos_router::components::{Route, Router, Routes};
use leptos_router::hooks::use_location;
use leptos_router::params::ParamsMap;
use leptos_router::{StaticSegment, WildcardSegment};

use crate::core::browser;
use crate::core::components::shell::{AppShell, SidebarSection, SidebarToggle};
use crate::core::components::theme_toggle::{THEME_INIT_SCRIPT, ThemeToggle, provide_theme_mode};
use crate::core::dock::context::{DockContext, use_dock};
use crate::core::dock::host::{DockHooks, DockHost};
use crate::core::dock::link::TabLink;
use crate::core::refresh::DataRefresh;
use crate::core::url_path::decode_path;
use crate::modules::agent_profiles::presentation::pages::models_page::ModelsPage;
use crate::modules::agent_profiles::presentation::pages::profiles_page::ProfilesPage;
use crate::modules::agent_settings::presentation::components::agent_settings_bar::AgentSettingsBar;
use crate::modules::agent_settings::presentation::pages::settings_page::SettingsPage;
use crate::modules::agent_settings::presentation::state::agent_settings_state::AgentSettingsState;
use crate::modules::agents_files::presentation::pages::agents_files_page::AgentsFilesPage;
use crate::modules::gitlab::presentation::components::gitlab_settings_card::GitLabSettingsCard;
use crate::modules::gitlab::presentation::components::task_merge_request_panel::TaskMergeRequestPanel;
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
    view! {
        <!DOCTYPE html>
        <html lang="es">
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

/// Pantalla que muestra una pestaña, deducida de su ruta.
#[derive(Debug, Clone, PartialEq, Eq)]
enum TabRoute {
    Board,
    Workspaces,
    WorkspaceTree(String),
    /// Fichero del proyecto: id del workspace y ruta relativa (ya decodificada).
    WorkspaceFile(String, String),
    WorkspaceEdit(String),
    WorkspaceTerminal(String),
    Terminals,
    TerminalSession(String),
    AgentsFiles,
    Profiles,
    Models,
    Settings,
    Task(String),
    TaskEdit(String),
    TaskTerminal(String),
    TaskDiff(String),
    NotFound,
}

impl TabRoute {
    fn parse(path: &str) -> Self {
        let segments: Vec<&str> = path.split('/').filter(|s: &&str| !s.is_empty()).collect();
        match segments.as_slice() {
            [] => Self::Board,
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
            ["profiles"] => Self::Profiles,
            ["models"] => Self::Models,
            ["settings"] => Self::Settings,
            ["tasks", id] => Self::Task((*id).to_string()),
            ["tasks", id, "edit"] => Self::TaskEdit((*id).to_string()),
            ["tasks", id, "terminal"] => Self::TaskTerminal((*id).to_string()),
            ["tasks", id, "diff"] => Self::TaskDiff((*id).to_string()),
            _ => Self::NotFound,
        }
    }

    fn title(&self) -> String {
        match self {
            Self::Board => "Tablero".to_string(),
            Self::Workspaces => "Workspaces".to_string(),
            Self::WorkspaceTree(_) => "Proyecto".to_string(),
            Self::WorkspaceFile(_, path) => path.rsplit('/').next().unwrap_or(path).to_string(),
            Self::WorkspaceEdit(_) => "Editar workspace".to_string(),
            Self::WorkspaceTerminal(_) | Self::TaskTerminal(_) => "Terminal".to_string(),
            Self::Terminals => "Terminales".to_string(),
            Self::TerminalSession(id) => {
                format!("Terminal {}", id.chars().take(8).collect::<String>())
            }
            Self::AgentsFiles => "AGENTS.md".to_string(),
            Self::Profiles => "Perfiles".to_string(),
            Self::Models => "Modelos".to_string(),
            Self::Settings => "Configuración".to_string(),
            Self::Task(_) => "Tarea".to_string(),
            Self::TaskEdit(_) => "Editar tarea".to_string(),
            Self::TaskDiff(_) => "Diff".to_string(),
            Self::NotFound => "No encontrada".to_string(),
        }
    }

    fn icon(&self) -> &'static str {
        match self {
            Self::Board => "▦",
            Self::Workspaces | Self::WorkspaceTree(_) => "▤",
            Self::WorkspaceFile(..) => "◫",
            Self::WorkspaceEdit(_) | Self::TaskEdit(_) => "✎",
            Self::TaskDiff(_) => "±",
            Self::WorkspaceTerminal(_)
            | Self::Terminals
            | Self::TerminalSession(_)
            | Self::TaskTerminal(_) => "❯",
            Self::AgentsFiles => "§",
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
        if let Self::WorkspaceFile(id, path) = self {
            params.insert("id", id.clone());
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
            Self::TerminalSession(id) => params.insert("session_id", id.clone()),
            _ => {}
        }
        params
    }

    fn view(self) -> AnyView {
        match self {
            Self::Board => view! { <BoardPage /> }.into_any(),
            Self::Workspaces => view! { <WorkspacesPage /> }.into_any(),
            Self::WorkspaceTree(_) => view! { <ProjectTreePage /> }.into_any(),
            Self::WorkspaceFile(..) => view! { <ProjectFilePage /> }.into_any(),
            Self::WorkspaceEdit(_) => view! { <WorkspaceEditPage /> }.into_any(),
            Self::WorkspaceTerminal(_) => view! { <NewTerminalPage /> }.into_any(),
            Self::Terminals => view! { <TerminalsPage /> }.into_any(),
            Self::TerminalSession(_) => view! { <TerminalSessionPage /> }.into_any(),
            Self::AgentsFiles => view! { <AgentsFilesPage /> }.into_any(),
            Self::Profiles => view! { <ProfilesPage /> }.into_any(),
            Self::Models => view! { <ModelsPage /> }.into_any(),
            Self::Settings => view! {
                <SettingsPage>
                    <GitLabSettingsCard />
                </SettingsPage>
            }
            .into_any(),
            Self::Task(_) => view! { <TaskDetailPage extra_tab=gitlab_task_tab() /> }.into_any(),
            Self::TaskEdit(_) => view! { <TaskEditPage /> }.into_any(),
            Self::TaskDiff(_) => view! { <TaskDiffPage /> }.into_any(),
            Self::TaskTerminal(_) => view! { <NewTaskTerminalPage /> }.into_any(),
            Self::NotFound => view! { <p class="muted p-4">"Página no encontrada."</p> }.into_any(),
        }
    }
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

/// Destinos de nivel superior que ofrecen la barra lateral, el menú `+` y los grupos vacíos.
const TOP_LEVEL_DESTINATIONS: [(&str, &str, &str); 7] = [
    ("/", "▦", "Tablero"),
    ("/terminals", "❯", "Terminales"),
    ("/workspaces", "▤", "Workspaces"),
    ("/agents-files", "§", "AGENTS.md"),
    ("/profiles", "☰", "Perfiles"),
    ("/models", "◇", "Modelos"),
    ("/settings", "⚙", "Configuración"),
];

/// Navegación de la barra lateral, en una sección colapsable "Menú".
fn sidebar_nav() -> impl IntoView {
    let dock: DockContext = use_dock();
    view! {
        <SidebarSection title="Menú" count=Signal::stored(TOP_LEVEL_DESTINATIONS.len())>
            <nav class="flex flex-col gap-0.5" aria-label="Navegación">
                {TOP_LEVEL_DESTINATIONS
                    .into_iter()
                    .map(|(path, icon, label)| {
                        view! {
                            <TabLink
                                href=path
                                class=move || {
                                    if dock.is_path_visible(path) {
                                        "sidebar-nav sidebar-nav-active".to_string()
                                    } else {
                                        "sidebar-nav".to_string()
                                    }
                                }
                            >
                                <span class="sidebar-nav-icon" aria-hidden="true">{icon}</span>
                                {label}
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
            <span class="flex-1 text-sm font-semibold tracking-tight">"iworkflow"</span>
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
        .map(|(path, icon, label)| {
            view! {
                <TabLink href=path class="menu-item">
                    <span class="mr-2 inline-block w-4 text-center text-muted" aria-hidden="true">{icon}</span>
                    {label}
                </TabLink>
            }
        })
        .collect_view()
}

/// Contenido de un grupo sin pestañas.
fn empty_group() -> impl IntoView {
    view! {
        <div class="absolute inset-0 flex flex-col items-center justify-center gap-4 p-6 text-center">
            <p class="text-sm text-muted">"Este grupo no tiene pestañas. Abre una:"</p>
            <div class="flex max-w-md flex-wrap justify-center gap-2">
                {TOP_LEVEL_DESTINATIONS
                    .into_iter()
                    .map(|(path, icon, label)| {
                        view! {
                            <TabLink
                                href=path

                                class="btn-secondary min-h-8 px-3 py-1 text-xs"
                            >
                                <span class="mr-1.5" aria-hidden="true">{icon}</span>
                                {label}
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
                <Route path=StaticSegment("terminals") view=|| () />
                <Route path=StaticSegment("agents-files") view=|| () />
                <Route path=StaticSegment("profiles") view=|| () />
                <Route path=StaticSegment("models") view=|| () />
                <Route path=StaticSegment("settings") view=|| () />
                <Route path=(StaticSegment("tasks"), WildcardSegment("rest")) view=|| () />
                <Route path=(StaticSegment("workspaces"), WildcardSegment("rest")) view=|| () />
                <Route path=(StaticSegment("terminals"), WildcardSegment("rest")) view=|| () />
            </Routes>
            <TaskStoreProvider>
                <DocumentTitle />
                <DockBootstrap />
                <AppShell brand="iworkflow" sidebar=app_sidebar>
                    <DockHost hooks=hooks launcher=tab_launcher empty=empty_group />
                </AppShell>
            </TaskStoreProvider>
        </Router>
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parse_maps_every_known_path_to_its_screen() {
        assert_eq!(TabRoute::parse("/"), TabRoute::Board);
        assert_eq!(TabRoute::parse("/settings"), TabRoute::Settings);
        assert_eq!(TabRoute::Settings.title(), "Configuración");
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
        assert_eq!(TabRoute::Board.params().get("id"), None);
    }

    #[test]
    fn titles_fall_back_to_a_short_terminal_id() {
        assert_eq!(tab_title("/"), "Tablero");
        assert_eq!(tab_title("/terminals/0123456789"), "Terminal 01234567");
    }
}

/// Pestaña de GitLab del panel de tarea: se compone aquí para que `tasks` no importe `gitlab`.
fn gitlab_task_tab() -> TaskExtraTab {
    TaskExtraTab {
        id: "gitlab",
        label: "GitLab",
        render: Callback::new(|task_id: String| {
            view! { <TaskMergeRequestPanel task_id=task_id /> }.into_any()
        }),
    }
}
