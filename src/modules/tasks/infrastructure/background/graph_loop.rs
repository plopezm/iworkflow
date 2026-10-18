use std::sync::Arc;
use std::time::Duration;

use crate::modules::tasks::domain::ports::inbound::successor_trigger::SuccessorTrigger;

/// Cada cuánto se reevalúan los disparadores del grafo (cubre merges hechos fuera de la UI y
/// arranques fallidos que vuelven a `Backlog`).
pub const GRAPH_TICK: Duration = Duration::from_secs(15);

/// Bucle de fondo: arranca las tareas cuyas predecesoras ya han cumplido su disparador. No
/// termina por sí solo; quien lo lanza conserva el `JoinHandle` y lo aborta al apagar.
pub async fn run_graph_loop(trigger: Arc<dyn SuccessorTrigger>) {
    let mut ticker: tokio::time::Interval = tokio::time::interval(GRAPH_TICK);
    ticker.set_missed_tick_behavior(tokio::time::MissedTickBehavior::Delay);
    loop {
        ticker.tick().await;
        match trigger.trigger().await {
            Ok(0) => {}
            Ok(started) => {
                tracing::info!("{started} tareas arrancadas por el grafo de dependencias")
            }
            Err(error) => tracing::warn!(%error, "fallo al evaluar el grafo de dependencias"),
        }
    }
}
