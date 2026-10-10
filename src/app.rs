use leptos::prelude::*;
use leptos_meta::{MetaTags, Stylesheet, Title, provide_meta_context};
use leptos_router::components::{A, Route, Router, Routes};
use leptos_router::{ParamSegment, StaticSegment};

use crate::core::components::shell::{AppShell, DropdownMenu};
use crate::core::components::theme_toggle::{THEME_INIT_SCRIPT, ThemeToggle, provide_theme_mode};
use crate::modules::agent_profiles::presentation::pages::profiles_page::ProfilesPage;
use crate::modules::agent_settings::presentation::components::agent_settings_bar::AgentSettingsBar;
use crate::modules::agents_files::presentation::pages::agents_files_page::AgentsFilesPage;
use crate::modules::project_tree::presentation::pages::project_tree_page::ProjectTreePage;
use crate::modules::tasks::presentation::components::app_sidebar::AppSidebar;
use crate::modules::tasks::presentation::pages::board_page::BoardPage;
use crate::modules::tasks::presentation::pages::task_detail_page::TaskDetailPage;
use crate::modules::tasks::presentation::pages::task_edit_page::TaskEditPage;
use crate::modules::tasks::presentation::state::task_store::TaskStore;
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

/// Contenido de la barra lateral: marca y menú de navegación, listados y ajustes del agente.
fn app_sidebar() -> impl IntoView {
    view! {
        <div class="flex items-center justify-between gap-2 border-b border-border px-3 py-3">
            <A href="/" attr:class="flex items-baseline gap-1.5 leading-none">
                <span class="font-display text-2xl">"iworkflow"</span>
                <span class="live-dot text-accent" aria-hidden="true"></span>
            </A>
            <DropdownMenu
                label="Actions"
                aria_label="Navegación"
                trigger_class="btn-secondary min-h-8 px-2.5 py-1 text-xs"
            >
                <A href="/" exact=true attr:class="menu-item">
                    "Tablero"
                </A>
                <A href="/workspaces" attr:class="menu-item">
                    "Workspaces"
                </A>
                <A href="/terminals" attr:class="menu-item">
                    "Terminales"
                </A>
                <A href="/agents-files" attr:class="menu-item">
                    "AGENTS.md"
                </A>
                <A href="/profiles" attr:class="menu-item">
                    "Perfiles"
                </A>
            </DropdownMenu>
        </div>
        <div class="min-h-0 flex-1 overflow-y-auto">
            <AppSidebar />
        </div>
        <div class="stack shrink-0 gap-1.5 border-t border-border p-3">
            <ThemeToggle />
            <AgentSettingsBar />
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

#[component]
pub fn App() -> impl IntoView {
    provide_meta_context();
    provide_theme_mode();

    view! {
        <Stylesheet id="leptos" href="/pkg/iworkflow.css" />
        <Title text="iworkflow" />
        <Router>
            <TaskStoreProvider>
            <AppShell brand="iworkflow" sidebar=app_sidebar>
                <Routes fallback=|| "Página no encontrada.".into_view()>
                    <Route path=StaticSegment("") view=BoardPage />
                    <Route path=StaticSegment("workspaces") view=WorkspacesPage />
                    <Route
                        path=(StaticSegment("workspaces"), ParamSegment("id"), StaticSegment("terminal"))
                        view=NewTerminalPage
                    />
                    <Route path=StaticSegment("terminals") view=TerminalsPage />
                    <Route
                        path=(StaticSegment("terminals"), ParamSegment("session_id"))
                        view=TerminalSessionPage
                    />
                    <Route
                        path=(StaticSegment("workspaces"), ParamSegment("id"), StaticSegment("edit"))
                        view=WorkspaceEditPage
                    />
                    <Route
                        path=(StaticSegment("workspaces"), ParamSegment("id"))
                        view=ProjectTreePage
                    />
                    <Route path=StaticSegment("agents-files") view=AgentsFilesPage />
                    <Route path=StaticSegment("profiles") view=ProfilesPage />
                    <Route path=(StaticSegment("tasks"), ParamSegment("id")) view=TaskDetailPage />
                    <Route
                        path=(StaticSegment("tasks"), ParamSegment("id"), StaticSegment("edit"))
                        view=TaskEditPage
                    />
                </Routes>
            </AppShell>
            </TaskStoreProvider>
        </Router>
    }
}
