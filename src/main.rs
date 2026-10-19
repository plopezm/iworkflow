#![recursion_limit = "256"]

#[cfg(feature = "ssr")]
#[tokio::main]
async fn main() {
    use axum::Router;
    use iworkflow::app::{App, shell};
    use iworkflow::app_state::AppState;
    use iworkflow::core::config::AppConfig;
    use iworkflow::core::db::pool::{DbPool, build_pool, run_migrations};
    use iworkflow::core::http::body_limit::with_server_fn_body_limit;
    use iworkflow::core::launch::{LaunchCommand, LaunchOptions, USAGE, browser_url, open_browser};
    use iworkflow::modules::tasks::infrastructure::adapters::inbound::attachment_http::task_attachments_router;
    use iworkflow::modules::tasks::infrastructure::adapters::inbound::sse::task_events_router;
    use iworkflow::modules::tasks::infrastructure::background::graph_loop::run_graph_loop;
    use iworkflow::modules::tasks::infrastructure::background::resume_loop::run_resume_loop;
    use iworkflow::modules::terminal::infrastructure::adapters::inbound::ws::terminal_ws_router;
    use leptos::prelude::*;
    use leptos_axum::{AxumRouteListing, LeptosRoutes, generate_route_list};
    use tracing_subscriber::EnvFilter;

    let options: LaunchOptions =
        match LaunchOptions::parse(std::env::args().skip(1).collect::<Vec<String>>()) {
            Ok(LaunchCommand::Run(options)) => options,
            Ok(LaunchCommand::Help) => {
                println!("{USAGE}");
                return;
            }
            Err(error) => {
                eprintln!("{error}");
                std::process::exit(2);
            }
        };

    dotenvy::dotenv().ok();
    tracing_subscriber::fmt()
        .with_env_filter(
            EnvFilter::from_default_env()
                .add_directive("iworkflow=info".parse().expect("directiva válida")),
        )
        .init();

    // Los worktrees con rutas largas fallan en Windows sin `core.longpaths`; no es fatal.
    if let Err(error) = iworkflow::core::git::enable_long_paths().await {
        tracing::warn!(%error, "no se pudo habilitar core.longpaths en git");
    }

    let config: AppConfig = AppConfig::from_env().expect("configuración inválida");
    let pool: DbPool = build_pool(&config.database_url).expect("pool de Postgres");
    run_migrations(&pool).await.expect("migraciones");

    let state: AppState = AppState::new(pool, &config).expect("runtimes de agente");
    let interrupted: usize = state
        .tasks
        .reconcile_tasks
        .execute()
        .await
        .expect("reconciliación de tareas");
    if interrupted > 0 {
        tracing::info!("{interrupted} tareas marcadas como interrumpidas tras el reinicio");
    }

    let resume_loop: tokio::task::JoinHandle<()> = tokio::spawn(run_resume_loop(
        state.tasks.resume_rate_limited_tasks.clone(),
    ));

    let graph_loop: tokio::task::JoinHandle<()> =
        tokio::spawn(run_graph_loop(state.tasks.trigger_successors.clone()));

    let conf: ConfFile = get_configuration(None).expect("configuración de Leptos");
    let leptos_options: LeptosOptions =
        iworkflow::core::site::with_packaged_defaults(conf.leptos_options);
    let addr: std::net::SocketAddr = leptos_options.site_addr;
    let routes: Vec<AxumRouteListing> = generate_route_list(App);

    let context_state: AppState = state.clone();
    // Las server functions cuelgan del router de Leptos: solo ahí se eleva el límite de
    // cuerpo (2 MiB por defecto) para admitir mensajes con imágenes en base64.
    let leptos_router: Router<LeptosOptions> = with_server_fn_body_limit(
        Router::new()
            .leptos_routes_with_context(&leptos_options, routes, move || context_state.provide(), {
                let options: LeptosOptions = leptos_options.clone();
                move || shell(options.clone())
            })
            .fallback(leptos_axum::file_and_error_handler(shell)),
    );
    let app: Router = Router::new()
        .merge(task_events_router(state.tasks.clone()))
        .merge(task_attachments_router(state.tasks.clone()))
        .merge(terminal_ws_router(state.terminal.clone()))
        .merge(leptos_router)
        .with_state(leptos_options);

    tracing::info!("escuchando en http://{addr}");
    let listener: tokio::net::TcpListener =
        tokio::net::TcpListener::bind(&addr).await.expect("bind");
    if options.open_browser {
        let url: String = browser_url(addr);
        if let Err(error) = open_browser(&url) {
            tracing::warn!(%error, "abre {url} manualmente en el navegador");
        }
    }
    let signal_received: std::sync::Arc<tokio::sync::Notify> =
        std::sync::Arc::new(tokio::sync::Notify::new());
    let notifier: std::sync::Arc<tokio::sync::Notify> = signal_received.clone();
    let server = axum::serve(listener, app.into_make_service())
        .with_graceful_shutdown(async move {
            shutdown_signal().await;
            notifier.notify_one();
        })
        .into_future();

    // Los streams SSE no terminan solos: tras la señal el servidor tiene un plazo de
    // gracia y, si sigue con conexiones abiertas, se continúa con el apagado.
    tokio::select! {
        result = server => result.expect("servidor"),
        () = async {
            signal_received.notified().await;
            tokio::time::sleep(SHUTDOWN_GRACE).await;
        } => tracing::warn!(
            "conexiones abiertas tras {SHUTDOWN_GRACE:?}; se fuerza el apagado del servidor"
        ),
    }

    graph_loop.abort();
    if let Err(error) = graph_loop.await
        && !error.is_cancelled()
    {
        tracing::error!(%error, "el bucle del grafo de dependencias terminó con error");
    }

    resume_loop.abort();
    if let Err(error) = resume_loop.await
        && !error.is_cancelled()
    {
        tracing::error!(%error, "el bucle de reanudación terminó con error");
    }

    match tokio::time::timeout(SHUTDOWN_FLUSH_TIMEOUT, state.tasks.flush_events()).await {
        Ok(()) => tracing::info!("eventos de agente pendientes persistidos; apagando"),
        Err(_) => tracing::error!(
            "tiempo agotado ({SHUTDOWN_FLUSH_TIMEOUT:?}) vaciando eventos de agente: \
             los eventos pendientes se pierden"
        ),
    }
}

#[cfg(feature = "ssr")]
const SHUTDOWN_GRACE: std::time::Duration = std::time::Duration::from_secs(5);

#[cfg(feature = "ssr")]
const SHUTDOWN_FLUSH_TIMEOUT: std::time::Duration = std::time::Duration::from_secs(30);

/// Se resuelve al recibir SIGINT (Ctrl+C) o, en unix, SIGTERM.
#[cfg(feature = "ssr")]
async fn shutdown_signal() {
    let ctrl_c = async {
        if let Err(error) = tokio::signal::ctrl_c().await {
            tracing::error!(%error, "no se pudo escuchar SIGINT");
            std::future::pending::<()>().await;
        }
    };

    #[cfg(unix)]
    let terminate = async {
        match tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate()) {
            Ok(mut stream) => {
                stream.recv().await;
            }
            Err(error) => {
                tracing::error!(%error, "no se pudo escuchar SIGTERM");
                std::future::pending::<()>().await;
            }
        }
    };
    #[cfg(not(unix))]
    let terminate = std::future::pending::<()>();

    tokio::select! {
        () = ctrl_c => {},
        () = terminate => {},
    }
    tracing::info!("señal de apagado recibida");
}

#[cfg(not(feature = "ssr"))]
pub fn main() {}
