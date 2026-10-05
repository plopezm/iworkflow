use leptos::prelude::*;
use leptos_meta::{MetaTags, Stylesheet, Title, provide_meta_context};
use leptos_router::components::{A, Route, Router, Routes};
use leptos_router::{ParamSegment, StaticSegment};

use crate::core::components::shell::{AppShell, DropdownMenu};
use crate::core::components::theme_toggle::{THEME_INIT_SCRIPT, ThemeToggle, provide_theme_mode};
use crate::modules::agent_settings::presentation::components::agent_settings_bar::AgentSettingsBar;
use crate::modules::agents_files::presentation::pages::agents_files_page::AgentsFilesPage;
use crate::modules::tasks::presentation::components::app_sidebar::AppSidebar;
use crate::modules::tasks::presentation::pages::board_page::BoardPage;
use crate::modules::tasks::presentation::pages::task_detail_page::TaskDetailPage;
use crate::modules::tasks::presentation::pages::task_edit_page::TaskEditPage;
use crate::modules::terminal::presentation::pages::terminal_page::TerminalPage;
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
        <div class="flex items-center justify-between gap-2 border-b border-border px-3 py-2.5">
            <A href="/" attr:class="text-sm font-bold tracking-widest uppercase">
                "iworkflow"
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
                <A href="/agents-files" attr:class="menu-item">
                    "AGENTS.md"
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

#[component]
pub fn App() -> impl IntoView {
    provide_meta_context();
    provide_theme_mode();

    view! {
        <Stylesheet id="leptos" href="/pkg/iworkflow.css" />
        <Title text="iworkflow" />
        <Router>
            <AppShell brand="iworkflow" sidebar=app_sidebar>
                <Routes fallback=|| "Página no encontrada.".into_view()>
                    <Route path=StaticSegment("") view=BoardPage />
                    <Route path=StaticSegment("workspaces") view=WorkspacesPage />
                    <Route
                        path=(StaticSegment("workspaces"), ParamSegment("id"), StaticSegment("terminal"))
                        view=TerminalPage
                    />
                    <Route path=StaticSegment("agents-files") view=AgentsFilesPage />
                    <Route path=(StaticSegment("tasks"), ParamSegment("id")) view=TaskDetailPage />
                    <Route
                        path=(StaticSegment("tasks"), ParamSegment("id"), StaticSegment("edit"))
                        view=TaskEditPage
                    />
                </Routes>
            </AppShell>
        </Router>
    }
}
