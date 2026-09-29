use leptos::prelude::*;
use leptos_meta::{MetaTags, Stylesheet, Title, provide_meta_context};
use leptos_router::components::{A, Route, Router, Routes};
use leptos_router::{ParamSegment, StaticSegment};

use crate::modules::tasks::presentation::pages::board_page::BoardPage;
use crate::modules::tasks::presentation::pages::task_detail_page::TaskDetailPage;
use crate::modules::workspaces::presentation::pages::workspaces_page::WorkspacesPage;

pub fn shell(options: LeptosOptions) -> impl IntoView {
    view! {
        <!DOCTYPE html>
        <html lang="es">
            <head>
                <meta charset="utf-8" />
                <meta name="viewport" content="width=device-width, initial-scale=1" />
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

#[component]
pub fn App() -> impl IntoView {
    provide_meta_context();

    view! {
        <Stylesheet id="leptos" href="/pkg/pi-workflow.css" />
        <Title text="pi-workflow" />
        <Router>
            <nav class="sticky top-0 z-20 border-b border-border/60 bg-surface/70 backdrop-blur-xl">
                <div class="mx-auto flex h-14 max-w-[1600px] items-center justify-between px-4 sm:px-6">
                    <div class="flex items-center gap-3">
                        <span class="bg-gradient-to-r from-accent to-purple bg-clip-text text-lg font-bold text-transparent">
                            "pi-workflow"
                        </span>
                    </div>
                    <div class="row gap-0.5 rounded-xl bg-surface-2/60 p-0.5 backdrop-blur-sm">
                        <A
                            href="/"
                            exact=true
                            attr:class="nav-pill"
                        >
                            "Tablero"
                        </A>
                        <A
                            href="/workspaces"
                            attr:class="nav-pill"
                        >
                            "Workspaces"
                        </A>
                    </div>
                </div>
            </nav>
            <main class="relative mx-auto max-w-[1600px] px-4 py-4 sm:px-6">
                <Routes fallback=|| "Página no encontrada.".into_view()>
                    <Route path=StaticSegment("") view=BoardPage />
                    <Route path=StaticSegment("workspaces") view=WorkspacesPage />
                    <Route path=(StaticSegment("tasks"), ParamSegment("id")) view=TaskDetailPage />
                </Routes>
            </main>
        </Router>
    }
}
