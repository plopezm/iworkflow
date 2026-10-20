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
    use iworkflow::core::listen::{
        ListenInfo, SITE_ADDR_VAR, local_network_ip, resolve_listen_addr,
    };
    use iworkflow::core::restart::{RESTART_EXIT_CODE, RestartHandle, is_supervised, respawn_self};
    use iworkflow::modules::preferences::domain::models::access_token::AccessToken;
    use iworkflow::modules::preferences::infrastructure::adapters::inbound::access_guard::{
        AccessGuard, access_router, require_access,
    };
    use iworkflow::modules::preferences::infrastructure::adapters::inbound::language_cookie::sync_language_cookie;
    use iworkflow::modules::preferences::infrastructure::adapters::inbound::same_origin::reject_cross_site_writes;
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
    // The bind address is read once at startup: the saved network-access choice only applies
    // after a restart, and an explicit `LEPTOS_SITE_ADDR` always takes precedence.
    let network_access: bool = match state.preferences.get_network_access.execute().await {
        Ok(enabled) => enabled,
        Err(error) => {
            tracing::warn!(%error, "no se pudo leer el acceso desde la red; solo escucha en local");
            false
        }
    };
    let saved_port: Option<u16> = match state.preferences.get_listen_port.execute().await {
        Ok(port) => port.map(|port| port.get()),
        Err(error) => {
            tracing::warn!(%error, "no se pudo leer el puerto guardado; se usa el de por defecto");
            None
        }
    };
    let locked_by_env: bool = std::env::var_os(SITE_ADDR_VAR).is_some();
    let addr: std::net::SocketAddr = resolve_listen_addr(
        leptos_options.site_addr,
        locked_by_env,
        network_access,
        saved_port,
    );
    // The token is demanded whenever the server is reachable from outside this computer, unless
    // the user switched it off. A failed read keeps it required (fail closed).
    let require_token: bool = match state.preferences.get_require_access_token.execute().await {
        Ok(required) => required,
        Err(error) => {
            tracing::warn!(%error, "no se pudo leer si se exige el token; se exigirá");
            true
        }
    };
    let token_required: bool = !addr.ip().is_loopback() && require_token;
    let listen_info: ListenInfo = ListenInfo {
        addr,
        locked_by_env,
        token_required,
    };
    let access_token: AccessToken = state
        .preferences
        .get_access_token
        .execute()
        .await
        .expect("token de acceso");
    let access_guard: AccessGuard = AccessGuard::new(access_token.clone(), token_required);
    if token_required {
        let host: String = local_network_ip()
            .map_or_else(|| "<IP>".to_string(), |ip: std::net::IpAddr| ip.to_string());
        tracing::info!(
            "acceso desde la red protegido con token; enlace: http://{host}:{}/?token={}",
            addr.port(),
            access_token.as_str()
        );
    } else if !addr.ip().is_loopback() {
        tracing::warn!("el servidor es accesible desde la red SIN token de acceso");
    }
    let routes: Vec<AxumRouteListing> = generate_route_list(App);
    let restart: RestartHandle = RestartHandle::default();

    let context_state: AppState = state.clone();
    // Las server functions cuelgan del router de Leptos: solo ahí se eleva el límite de
    // cuerpo (2 MiB por defecto) para admitir mensajes con imágenes en base64.
    let leptos_router: Router<LeptosOptions> = with_server_fn_body_limit(
        Router::new()
            .leptos_routes_with_context(
                &leptos_options,
                routes,
                {
                    let guard: AccessGuard = access_guard.clone();
                    let restart: RestartHandle = restart.clone();
                    move || {
                        context_state.provide();
                        provide_context(listen_info);
                        provide_context(restart.clone());
                        provide_context(guard.clone());
                    }
                },
                {
                    let options: LeptosOptions = leptos_options.clone();
                    move || shell(options.clone())
                },
            )
            .fallback(leptos_axum::file_and_error_handler(shell)),
    );
    let app: Router = Router::new()
        .merge(task_events_router(state.tasks.clone()))
        .merge(task_attachments_router(state.tasks.clone()))
        .merge(terminal_ws_router(state.terminal.clone()))
        .merge(leptos_router)
        .layer(axum::middleware::from_fn_with_state(
            state.preferences.clone(),
            sync_language_cookie,
        ))
        .with_state(leptos_options)
        .merge(access_router(access_guard.clone()))
        // Outermost layer: nothing (pages, server functions, SSE, WebSocket) is served before it.
        .layer(axum::middleware::from_fn_with_state(
            access_guard,
            require_access,
        ))
        // Browsers cannot be used by another site to change settings or run actions here.
        .layer(axum::middleware::from_fn(reject_cross_site_writes));

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
    let restart_wait: RestartHandle = restart.clone();
    let server = axum::serve(listener, app.into_make_service())
        .with_graceful_shutdown(async move {
            tokio::select! {
                () = shutdown_signal() => {},
                () = restart_wait.requested() => tracing::info!("reinicio solicitado desde la interfaz"),
            }
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

    // Everything is flushed and the port is free: hand over to the supervisor or start again.
    if restart.is_requested() {
        if is_supervised() {
            std::process::exit(RESTART_EXIT_CODE);
        }
        let args: Vec<String> = std::env::args().skip(1).collect();
        if let Err(error) = respawn_self(&args) {
            tracing::error!(%error, "no se pudo reiniciar el servidor; inícialo manualmente");
            std::process::exit(1);
        }
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
