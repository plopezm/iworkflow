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
            <nav class="sticky top-0 z-20 row h-14 justify-between gap-4 border-b border-border bg-surface px-4 sm:px-6">
                <span class="font-semibold text-fg">"pi-workflow"</span>
                <div class="row gap-1">
                    <A
                        href="/"
                        exact=true
                        attr:class="min-h-10 rounded-lg px-3 py-2 text-sm font-medium text-muted transition-colors hover:text-fg aria-[current=page]:bg-surface-2 aria-[current=page]:text-accent"
                    >
                        "Tablero"
                    </A>
                    <A
                        href="/workspaces"
                        attr:class="min-h-10 rounded-lg px-3 py-2 text-sm font-medium text-muted transition-colors hover:text-fg aria-[current=page]:bg-surface-2 aria-[current=page]:text-accent"
                    >
                        "Workspaces"
                    </A>
                </div>
            </nav>
            <main class="mx-auto max-w-[1600px] px-4 py-4 sm:px-6">
                <Routes fallback=|| "Página no encontrada.".into_view()>
                    <Route path=StaticSegment("") view=BoardPage />
                    <Route path=StaticSegment("workspaces") view=WorkspacesPage />
                    <Route path=(StaticSegment("tasks"), ParamSegment("id")) view=TaskDetailPage />
                </Routes>
            </main>
        </Router>
    }
}
